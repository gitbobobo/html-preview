package screenshot

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"html-preview/internal/storage"
)

const cftVersionsURL = "https://googlechromelabs.github.io/chrome-for-testing/last-known-good-versions-with-downloads.json"

type InstallStatus string

const (
	InstallIdle       InstallStatus = "idle"
	InstallPending    InstallStatus = "pending"
	InstallInstalling InstallStatus = "installing"
	InstallDone       InstallStatus = "done"
	InstallFailed     InstallStatus = "failed"
)

type InstallState struct {
	Status  InstallStatus `json:"status"`
	Message string        `json:"message"`
}

type installManager struct {
	mu    sync.Mutex
	state InstallState
}

func newInstallManager() *installManager {
	return &installManager{state: InstallState{Status: InstallIdle, Message: ""}}
}

func (m *installManager) snapshot() InstallState {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state
}

func (m *installManager) set(status InstallStatus, message string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state = InstallState{Status: status, Message: message}
}

func (m *installManager) tryStart() (InstallState, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state.Status == InstallPending || m.state.Status == InstallInstalling {
		return m.state, false
	}
	m.state = InstallState{Status: InstallPending, Message: "install queued"}
	return m.state, true
}

type cftVersionsResponse struct {
	Channels struct {
		Stable struct {
			Version   string `json:"version"`
			Downloads struct {
				Chrome []struct {
					Platform string `json:"platform"`
					URL      string `json:"url"`
				} `json:"chrome"`
			} `json:"downloads"`
		} `json:"Stable"`
	} `json:"channels"`
}

func cftPlatform() (string, error) {
	switch runtime.GOOS {
	case "linux":
		switch runtime.GOARCH {
		case "amd64":
			return "linux64", nil
		case "arm64":
			return "linux64", nil
		}
	case "darwin":
		switch runtime.GOARCH {
		case "arm64":
			return "mac-arm64", nil
		case "amd64":
			return "mac-x64", nil
		}
	case "windows":
		switch runtime.GOARCH {
		case "amd64":
			return "win64", nil
		case "386":
			return "win32", nil
		}
	}
	return "", fmt.Errorf("unsupported platform %s/%s", runtime.GOOS, runtime.GOARCH)
}

func downloadChromeForTesting(dataDir string) (string, error) {
	platform, err := cftPlatform()
	if err != nil {
		return "", err
	}

	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Get(cftVersionsURL)
	if err != nil {
		return "", fmt.Errorf("fetch versions: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch versions: HTTP %d", resp.StatusCode)
	}

	var versions cftVersionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&versions); err != nil {
		return "", fmt.Errorf("parse versions: %w", err)
	}

	var downloadURL string
	for _, item := range versions.Channels.Stable.Downloads.Chrome {
		if item.Platform == platform {
			downloadURL = item.URL
			break
		}
	}
	if downloadURL == "" {
		return "", fmt.Errorf("no chrome package for platform %s", platform)
	}

	tmpZip, err := os.CreateTemp(dataDir, "chrome-*.zip")
	if err != nil {
		return "", err
	}
	zipPath := tmpZip.Name()
	defer os.Remove(zipPath)

	dlResp, err := client.Get(downloadURL)
	if err != nil {
		tmpZip.Close()
		return "", fmt.Errorf("download chrome: %w", err)
	}
	defer dlResp.Body.Close()
	if dlResp.StatusCode != http.StatusOK {
		tmpZip.Close()
		return "", fmt.Errorf("download chrome: HTTP %d", dlResp.StatusCode)
	}
	if _, err := io.Copy(tmpZip, dlResp.Body); err != nil {
		tmpZip.Close()
		return "", fmt.Errorf("save chrome zip: %w", err)
	}
	if err := tmpZip.Close(); err != nil {
		return "", err
	}

	chromeRoot := ChromeDir(dataDir)
	if err := os.RemoveAll(chromeRoot); err != nil {
		return "", err
	}
	if err := os.MkdirAll(chromeRoot, 0o755); err != nil {
		return "", err
	}
	opts := storage.ZipExtractOpts{RequireIndexHTML: false}
	if _, err := storage.UnzipArchive(zipPath, chromeRoot, opts); err != nil {
		return "", err
	}

	exe, err := findChromeExecutable(chromeRoot)
	if err != nil {
		return "", err
	}
	return exe, nil
}

func findChromeExecutable(root string) (string, error) {
	var found string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		name := info.Name()
		if runtime.GOOS == "windows" {
			if strings.EqualFold(name, "chrome.exe") {
				found = path
				return filepath.SkipAll
			}
			return nil
		}
		if name == "chrome" || name == "Google Chrome for Testing" {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil && err != filepath.SkipAll {
		return "", err
	}
	if found == "" {
		return "", fmt.Errorf("chrome executable not found in %s", root)
	}
	if err := os.Chmod(found, 0o755); err != nil {
		return "", err
	}
	return found, nil
}


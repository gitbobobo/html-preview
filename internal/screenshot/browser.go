package screenshot

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"html-preview/internal/localconfig"
)

var pathCandidates = []string{
	"chromium",
	"chromium-browser",
	"google-chrome",
	"google-chrome-stable",
	"chrome",
}

type BrowserInfo struct {
	Available bool
	Path      string
	Source    string
	Message   string
}

func Detect(envChromePath string, cfg *localconfig.Store) BrowserInfo {
	if envChromePath != "" {
		if isExecutable(envChromePath) {
			return BrowserInfo{
				Available: true,
				Path:      envChromePath,
				Source:    "env",
				Message:   "browser from CHROME_PATH",
			}
		}
		return BrowserInfo{
			Available: false,
			Path:      envChromePath,
			Source:    "env",
			Message:   "CHROME_PATH is set but not executable",
		}
	}

	if cfg != nil {
		file, err := cfg.Load()
		if err == nil && file.ChromePath != "" {
			if isExecutable(file.ChromePath) {
				source := file.ChromeSource
				if source == "" {
					source = "config"
				}
				return BrowserInfo{
					Available: true,
					Path:      file.ChromePath,
					Source:    source,
					Message:   "browser from config.json",
				}
			}
		}
	}

	for _, name := range pathCandidates {
		if path, err := exec.LookPath(name); err == nil && isExecutable(path) {
			return BrowserInfo{
				Available: true,
				Path:      path,
				Source:    "path",
				Message:   "browser found on PATH",
			}
		}
	}

	return BrowserInfo{
		Available: false,
		Source:    "",
		Message:   "no browser found; set CHROME_PATH or install Chrome for Testing",
	}
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	if info.IsDir() {
		return false
	}
	if runtime.GOOS == "windows" {
		return true
	}
	return info.Mode()&0o111 != 0
}

func ChromeDir(dataDir string) string {
	return filepath.Join(dataDir, "chrome")
}

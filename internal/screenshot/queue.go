package screenshot

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"html-preview/internal/item"
	"html-preview/internal/localconfig"
	"html-preview/internal/storage"
)

const (
	queueCapacity          = 64
	compensateInitialDelay = 2 * time.Second
	compensateInterval     = 30 * time.Second
)

type Service struct {
	Items         *item.Service
	DataDir       string
	EnvChromePath string

	cfg     *localconfig.Store
	install *installManager
	queue   chan string

	mu     sync.Mutex
	active map[string]struct{}
}

func New(items *item.Service, envChromePath string) *Service {
	return &Service{
		Items:         items,
		DataDir:       items.DataDir,
		EnvChromePath: envChromePath,
		cfg:           localconfig.New(items.DataDir),
		install:       newInstallManager(),
		queue:         make(chan string, queueCapacity),
		active:        make(map[string]struct{}),
	}
}

func (s *Service) Start(ctx context.Context) {
	go s.worker(ctx)
	go s.compensate(ctx)
}

type BrowserStatusResponse struct {
	Available bool          `json:"available"`
	Path      string        `json:"path"`
	Source    string        `json:"source"`
	Message   string        `json:"message"`
	Install   *InstallState `json:"install,omitempty"`
}

func (s *Service) BrowserStatus() BrowserStatusResponse {
	info := Detect(s.EnvChromePath, s.cfg)
	install := s.install.snapshot()

	message := info.Message
	switch install.Status {
	case InstallInstalling, InstallPending, InstallFailed:
		message = install.Message
	case InstallDone:
		if info.Available {
			message = "Chrome for Testing installed"
		}
	}

	resp := BrowserStatusResponse{
		Available: info.Available,
		Path:      info.Path,
		Source:    info.Source,
		Message:   message,
	}
	if install.Status != InstallIdle {
		st := install
		resp.Install = &st
	}
	return resp
}

func (s *Service) StartInstall() InstallState {
	state, ok := s.install.tryStart()
	if !ok {
		return state
	}
	go s.runInstall()
	return state
}

func (s *Service) runInstall() {
	s.install.set(InstallInstalling, "downloading Chrome for Testing")
	exe, err := downloadChromeForTesting(s.DataDir)
	if err != nil {
		s.install.set(InstallFailed, err.Error())
		return
	}
	if err := s.cfg.SaveChrome(exe, "chrome-for-testing"); err != nil {
		s.install.set(InstallFailed, err.Error())
		return
	}
	s.install.set(InstallDone, "installation complete")
	s.requeueNoBrowser()
}

func (s *Service) HandleNewItem(itemID string) {
	info := Detect(s.EnvChromePath, s.cfg)
	if !info.Available {
		_ = s.Items.SetScreenshotStatus(itemID, "no_browser", "")
		return
	}
	go s.Enqueue(context.Background(), itemID)
}

func (s *Service) HandleReplacedItem(itemID string) {
	s.HandleNewItem(itemID)
}

func (s *Service) Enqueue(ctx context.Context, itemID string) {
	if !s.markActive(itemID) {
		return
	}

	select {
	case s.queue <- itemID:
	case <-ctx.Done():
		s.releaseItem(itemID)
	}
}

func (s *Service) markActive(itemID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.active[itemID]; ok {
		return false
	}
	s.active[itemID] = struct{}{}
	return true
}

func (s *Service) releaseItem(itemID string) {
	s.mu.Lock()
	delete(s.active, itemID)
	s.mu.Unlock()
}

func (s *Service) compensate(ctx context.Context) {
	select {
	case <-ctx.Done():
		return
	case <-time.After(compensateInitialDelay):
	}

	s.compensateOnce(ctx)

	ticker := time.NewTicker(compensateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.compensateOnce(ctx)
		}
	}
}

func (s *Service) compensateOnce(ctx context.Context) {
	info := Detect(s.EnvChromePath, s.cfg)
	if !info.Available {
		return
	}

	ids, err := s.Items.ListScreenshotRetryIDs()
	if err != nil {
		log.Printf("screenshot compensate query: %v", err)
		return
	}
	for _, id := range ids {
		s.Enqueue(ctx, id)
	}
}

func (s *Service) requeueNoBrowser() {
	ids, err := s.Items.ResetNoBrowserToPending()
	if err != nil {
		log.Printf("requeue no_browser: %v", err)
		return
	}
	for _, id := range ids {
		s.Enqueue(context.Background(), id)
	}
}

func (s *Service) worker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case itemID := <-s.queue:
			s.processItem(ctx, itemID)
			s.releaseItem(itemID)
		}
	}
}

func (s *Service) processItem(ctx context.Context, itemID string) {
	info := Detect(s.EnvChromePath, s.cfg)
	if !info.Available {
		_ = s.Items.SetScreenshotStatus(itemID, "no_browser", "")
		return
	}

	jobCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	if err := s.captureItem(jobCtx, itemID, info.Path); err != nil {
		_ = s.Items.SetScreenshotStatus(itemID, "failed", err.Error())
		log.Printf("screenshot %s failed: %v", itemID, err)
		return
	}

	_ = s.Items.SetScreenshotStatus(itemID, "ready", "")
}

func (s *Service) captureItem(ctx context.Context, itemID, chromePath string) error {
	htmlPath := filepath.Join(storage.ItemDir(s.DataDir, itemID), "index.html")
	if _, err := os.Stat(htmlPath); err != nil {
		return fmt.Errorf("index.html: %w", err)
	}
	pageURL, err := fileURL(htmlPath)
	if err != nil {
		return fmt.Errorf("page url: %w", err)
	}

	profileDir, err := userDataDir(s.DataDir)
	if err != nil {
		return err
	}

	desktopPNG, err := capturePage(ctx, chromePath, pageURL, profileDir, desktopViewport)
	if err != nil {
		return fmt.Errorf("desktop capture: %w", err)
	}
	desktopWebP, err := resizeToWebP(desktopPNG)
	if err != nil {
		return fmt.Errorf("desktop resize: %w", err)
	}

	mobilePNG, err := capturePage(ctx, chromePath, pageURL, profileDir, mobileViewport)
	if err != nil {
		return fmt.Errorf("mobile capture: %w", err)
	}
	mobileWebP, err := resizeToWebP(mobilePNG)
	if err != nil {
		return fmt.Errorf("mobile resize: %w", err)
	}

	return writeThumbsAtomically(s.DataDir, itemID, desktopWebP, mobileWebP)
}

func writeThumbsAtomically(dataDir, itemID string, desktopWebP, mobileWebP []byte) error {
	itemDir := storage.ItemDir(dataDir, itemID)
	if err := os.MkdirAll(itemDir, 0o755); err != nil {
		return err
	}

	desktopPath := storage.DesktopThumbPath(dataDir, itemID)
	mobilePath := storage.MobileThumbPath(dataDir, itemID)
	desktopTmp := desktopPath + ".tmp"
	mobileTmp := mobilePath + ".tmp"

	cleanup := func() {
		_ = os.Remove(desktopTmp)
		_ = os.Remove(mobileTmp)
	}

	if err := os.WriteFile(desktopTmp, desktopWebP, 0o644); err != nil {
		cleanup()
		return err
	}
	if err := os.WriteFile(mobileTmp, mobileWebP, 0o644); err != nil {
		cleanup()
		return err
	}

	if err := os.Rename(desktopTmp, desktopPath); err != nil {
		cleanup()
		return err
	}
	if err := os.Rename(mobileTmp, mobilePath); err != nil {
		_ = os.Remove(desktopPath)
		cleanup()
		return err
	}

	return nil
}

func (s *Service) DataDirPath() string {
	return filepath.Clean(s.DataDir)
}

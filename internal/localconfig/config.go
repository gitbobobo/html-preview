package localconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type File struct {
	ChromePath   string `json:"chrome_path,omitempty"`
	ChromeSource string `json:"chrome_source,omitempty"`
}

type Store struct {
	path string
	mu   sync.RWMutex
}

func New(dataDir string) *Store {
	return &Store{path: filepath.Join(dataDir, "config.json")}
}

func (s *Store) Path() string {
	return s.path
}

func (s *Store) Load() (File, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return File{}, nil
		}
		return File{}, err
	}

	var cfg File
	if err := json.Unmarshal(data, &cfg); err != nil {
		return File{}, err
	}
	return cfg, nil
}

func (s *Store) SaveChrome(path, source string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cfg := File{}
	if data, err := os.ReadFile(s.path); err == nil {
		_ = json.Unmarshal(data, &cfg)
	}
	cfg.ChromePath = path
	cfg.ChromeSource = source

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o644)
}

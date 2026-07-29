package storage

import (
	"io"
	"os"
	"path/filepath"
)

const MaxHTMLBytes = 2 << 20 // 2MB

func SaveHTML(dataDir, id string, r io.Reader, size int64) (int64, error) {
	return SaveHTMLToDir(ItemDir(dataDir, id), r, size)
}

func SaveHTMLToDir(dir string, r io.Reader, size int64) (int64, error) {
	if size > MaxHTMLBytes {
		return 0, ErrHTMLTooLarge
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return 0, err
	}
	dest := filepath.Join(dir, "index.html")
	f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	limited := io.LimitReader(r, MaxHTMLBytes+1)
	n, err := io.Copy(f, limited)
	if err != nil {
		os.Remove(dest)
		os.RemoveAll(dir)
		return 0, err
	}
	if n > MaxHTMLBytes {
		os.Remove(dest)
		os.RemoveAll(dir)
		return 0, ErrHTMLTooLarge
	}
	return n, nil
}

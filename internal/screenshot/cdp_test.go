package screenshot

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFileURL(t *testing.T) {
	dir := t.TempDir()
	htmlPath := filepath.Join(dir, "items", "abc", "index.html")
	if err := os.MkdirAll(filepath.Dir(htmlPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(htmlPath, []byte("<html></html>"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := fileURL(htmlPath)
	if err != nil {
		t.Fatalf("fileURL: %v", err)
	}
	if !strings.HasPrefix(got, "file://") {
		t.Fatalf("expected file:// prefix, got %q", got)
	}

	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if parsed.Scheme != "file" {
		t.Fatalf("scheme: %q", parsed.Scheme)
	}

	abs, err := filepath.Abs(htmlPath)
	if err != nil {
		t.Fatal(err)
	}
	wantPath := filepath.ToSlash(abs)
	if parsed.Path != wantPath {
		t.Fatalf("path: got %q want %q", parsed.Path, wantPath)
	}
}

func TestCapturePageFileURL(t *testing.T) {
	chromePath := os.Getenv("CHROME_PATH")
	if chromePath == "" {
		chromePath = "/usr/bin/google-chrome"
	}
	if st, err := os.Stat(chromePath); err != nil || st.IsDir() {
		t.Skip("chrome not available")
	}

	dir := t.TempDir()
	htmlPath := filepath.Join(dir, "index.html")
	html := `<!DOCTYPE html><html><head><style>body{background:rgb(255,0,0);margin:0}h1{font-size:48px}</style></head><body><h1>file capture</h1></body></html>`
	if err := os.WriteFile(htmlPath, []byte(html), 0o644); err != nil {
		t.Fatal(err)
	}
	pageURL, err := fileURL(htmlPath)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	profileDir := filepath.Join(dir, "chrome-profile")
	png, err := capturePage(ctx, chromePath, pageURL, profileDir, desktopViewport)
	if err != nil {
		t.Fatalf("capturePage: %v", err)
	}
	if len(png) < 100 {
		t.Fatalf("png too small: %d bytes", len(png))
	}
}

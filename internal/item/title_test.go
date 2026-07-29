package item

import (
	"bytes"
	"path/filepath"
	"testing"

	"html-preview/internal/db"
)

const titledHTML = `<!DOCTYPE html><html><head><title>Parsed Page Title</title></head><body></body></html>`
const untitledHTML = `<!DOCTYPE html><html><head></head><body><h1>no title</h1></body></html>`

func newTitleSvc(t *testing.T) (*Service, func()) {
	t.Helper()
	dir := t.TempDir()
	conn, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	return &Service{DB: conn, DataDir: dir}, func() { _ = conn.Close() }
}

func TestCreateFromUpload_ParsesHTMLTitle(t *testing.T) {
	svc, cleanup := newTitleSvc(t)
	defer cleanup()

	it, err := svc.CreateFromUpload("", "", "never", "", "page.html", bytes.NewReader([]byte(titledHTML)), int64(len(titledHTML)))
	if err != nil {
		t.Fatalf("CreateFromUpload: %v", err)
	}
	if it.Title != "Parsed Page Title" {
		t.Fatalf("expected parsed title, got %q", it.Title)
	}
}

func TestCreateFromUpload_FallsBackToFilename(t *testing.T) {
	svc, cleanup := newTitleSvc(t)
	defer cleanup()

	// No parseable title -> filename default, extension stripped.
	it, err := svc.CreateFromUpload("", "", "never", "", "my page.html", bytes.NewReader([]byte(untitledHTML)), int64(len(untitledHTML)))
	if err != nil {
		t.Fatalf("CreateFromUpload: %v", err)
	}
	if it.Title != "my page" {
		t.Fatalf("expected filename fallback %q, got %q", "my page", it.Title)
	}
}

func TestCreateFromUpload_ClientTitleWins(t *testing.T) {
	svc, cleanup := newTitleSvc(t)
	defer cleanup()

	it, err := svc.CreateFromUpload("Custom", "never", "", "", "page.html", bytes.NewReader([]byte(titledHTML)), int64(len(titledHTML)))
	if err != nil {
		t.Fatalf("CreateFromUpload: %v", err)
	}
	if it.Title != "Custom" {
		t.Fatalf("expected client title to win, got %q", it.Title)
	}
}

func TestReplaceContent_RefreshesDefaultTitle(t *testing.T) {
	svc, cleanup := newTitleSvc(t)
	defer cleanup()

	// Create with untitled HTML -> title becomes filename default "doc".
	it, err := svc.CreateFromUpload("", "", "never", "", "doc.html", bytes.NewReader([]byte(untitledHTML)), int64(len(untitledHTML)))
	if err != nil {
		t.Fatalf("CreateFromUpload: %v", err)
	}
	if it.Title != "doc" {
		t.Fatalf("setup title got %q", it.Title)
	}

	// Replace with HTML that now has a <title>; default title should refresh.
	it2, err := svc.ReplaceContent(it.ID, "doc.html", bytes.NewReader([]byte(titledHTML)), int64(len(titledHTML)))
	if err != nil {
		t.Fatalf("ReplaceContent: %v", err)
	}
	if it2.Title != "Parsed Page Title" {
		t.Fatalf("expected refreshed title, got %q", it2.Title)
	}
}

func TestReplaceContent_PreservesCustomTitle(t *testing.T) {
	svc, cleanup := newTitleSvc(t)
	defer cleanup()

	it, err := svc.CreateFromUpload("My Custom Name", "never", "", "", "doc.html", bytes.NewReader([]byte(titledHTML)), int64(len(titledHTML)))
	if err != nil {
		t.Fatalf("CreateFromUpload: %v", err)
	}

	it2, err := svc.ReplaceContent(it.ID, "other.html", bytes.NewReader([]byte(titledHTML)), int64(len(titledHTML)))
	if err != nil {
		t.Fatalf("ReplaceContent: %v", err)
	}
	if it2.Title != "My Custom Name" {
		t.Fatalf("expected custom title preserved, got %q", it2.Title)
	}
}

func TestTitleFromFilename_StripsExtension(t *testing.T) {
	cases := map[string]string{
		"page.html":      "page",
		"archive.tar.gz": "archive.tar",
		"a/b/c/d.htm":    "d",
		`win\path.html`:  "path",
		"noext":          "noext",
		"":               "",
	}
	for in, want := range cases {
		if got := TitleFromFilename(in); got != want {
			t.Errorf("TitleFromFilename(%q) = %q, want %q", in, got, want)
		}
	}
}

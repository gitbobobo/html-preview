package item

import (
	"path/filepath"
	"testing"
	"time"

	"html-preview/internal/db"
)

func TestSetScreenshotStatus(t *testing.T) {
	dir := t.TempDir()
	conn, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = conn.Exec(`
		INSERT INTO items (
			id, title, notes, status, source_kind, original_filename, size_bytes,
			screenshot_status, created_at, updated_at
		) VALUES ('item1', 't', '', 'active', 'html', 'a.html', 10, 'pending', ?, ?)
	`, now, now)
	if err != nil {
		t.Fatalf("insert item: %v", err)
	}

	svc := &Service{DB: conn, DataDir: dir}
	if err := svc.SetScreenshotStatus("item1", "failed", "boom"); err != nil {
		t.Fatalf("SetScreenshotStatus: %v", err)
	}

	var status string
	var screenshotError *string
	if err := conn.QueryRow(`SELECT screenshot_status, screenshot_error FROM items WHERE id = 'item1'`).
		Scan(&status, &screenshotError); err != nil {
		t.Fatal(err)
	}
	if status != "failed" || screenshotError == nil || *screenshotError != "boom" {
		t.Fatalf("unexpected status=%q err=%v", status, screenshotError)
	}

	if err := svc.SetScreenshotStatus("missing", "ready", ""); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestListScreenshotRetryIDs(t *testing.T) {
	dir := t.TempDir()
	conn, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	now := time.Now().UTC().Format(time.RFC3339)
	for _, row := range []struct {
		id, status string
	}{
		{"pending1", "pending"},
		{"failed1", "failed"},
		{"ready1", "ready"},
	} {
		_, err = conn.Exec(`
			INSERT INTO items (
				id, title, notes, status, source_kind, original_filename, size_bytes,
				screenshot_status, created_at, updated_at
			) VALUES (?, 't', '', 'active', 'html', 'a.html', 10, ?, ?, ?)
		`, row.id, row.status, now, now)
		if err != nil {
			t.Fatalf("insert %s: %v", row.id, err)
		}
	}

	svc := &Service{DB: conn, DataDir: dir}
	ids, err := svc.ListScreenshotRetryIDs()
	if err != nil {
		t.Fatalf("ListScreenshotRetryIDs: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("expected 2 ids, got %v", ids)
	}
}

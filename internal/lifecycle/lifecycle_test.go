package lifecycle_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"html-preview/internal/db"
	"html-preview/internal/lifecycle"
	"html-preview/internal/storage"
)

func setupTest(t *testing.T) (*sql.DB, string) {
	t.Helper()
	dir := t.TempDir()
	conn, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn, dir
}

func insertItem(t *testing.T, conn *sql.DB, id, status string, expiresAt, trashedAt *string) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := conn.Exec(`
		INSERT INTO items (
			id, title, notes, status, source_kind, original_filename, size_bytes,
			expires_at, trashed_at, screenshot_status, created_at, updated_at
		) VALUES (?, 't', '', ?, 'html', 'a.html', 10, ?, ?, 'pending', ?, ?)
	`, id, status, expiresAt, trashedAt, now, now)
	if err != nil {
		t.Fatalf("insert item: %v", err)
	}
}

func TestExpireActive(t *testing.T) {
	conn, dataDir := setupTest(t)

	past := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
	future := time.Now().UTC().Add(time.Hour).Format(time.RFC3339)

	insertItem(t, conn, "expired1", "active", &past, nil)
	insertItem(t, conn, "active1", "active", &future, nil)
	insertItem(t, conn, "permanent", "active", nil, nil)

	if err := os.MkdirAll(storage.ItemDir(dataDir, "expired1"), 0o755); err != nil {
		t.Fatal(err)
	}

	svc := lifecycle.New(conn, dataDir)
	n, err := svc.ExpireActive(context.Background())
	if err != nil {
		t.Fatalf("ExpireActive: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 expired, got %d", n)
	}

	var status string
	var trashedAt string
	if err := conn.QueryRow(`SELECT status, trashed_at FROM items WHERE id = 'expired1'`).Scan(&status, &trashedAt); err != nil {
		t.Fatal(err)
	}
	if status != "trash" || trashedAt == "" {
		t.Fatalf("expired1: status=%q trashed_at=%q", status, trashedAt)
	}

	if err := conn.QueryRow(`SELECT status FROM items WHERE id = 'active1'`).Scan(&status); err != nil || status != "active" {
		t.Fatalf("active1 should stay active, got %q err=%v", status, err)
	}
	if err := conn.QueryRow(`SELECT status FROM items WHERE id = 'permanent'`).Scan(&status); err != nil || status != "active" {
		t.Fatalf("permanent should stay active, got %q err=%v", status, err)
	}
}

func TestPurgeTrash(t *testing.T) {
	conn, dataDir := setupTest(t)

	old := time.Now().UTC().Add(-31 * 24 * time.Hour).Format(time.RFC3339)
	recent := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)

	insertItem(t, conn, "old-trash", "trash", nil, &old)
	insertItem(t, conn, "new-trash", "trash", nil, &recent)

	for _, id := range []string{"old-trash", "new-trash"} {
		if err := os.MkdirAll(storage.ItemDir(dataDir, id), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(storage.ItemDir(dataDir, id), "index.html"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	svc := lifecycle.New(conn, dataDir)
	n, err := svc.PurgeTrash(context.Background())
	if err != nil {
		t.Fatalf("PurgeTrash: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 purged, got %d", n)
	}

	var count int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM items WHERE id = 'old-trash'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("old-trash should be deleted from db")
	}
	if _, err := os.Stat(storage.ItemDir(dataDir, "old-trash")); !os.IsNotExist(err) {
		t.Fatalf("old-trash dir should be removed")
	}
	if err := conn.QueryRow(`SELECT COUNT(*) FROM items WHERE id = 'new-trash'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("new-trash should remain in db")
	}
	if _, err := os.Stat(storage.ItemDir(dataDir, "new-trash")); err != nil {
		t.Fatalf("new-trash dir should remain: %v", err)
	}
}

func TestParseInterval(t *testing.T) {
	if lifecycle.ParseInterval("") != time.Minute {
		t.Fatal("empty should default to 1m")
	}
	if lifecycle.ParseInterval("2s") != 2*time.Second {
		t.Fatal("2s expected")
	}
	if lifecycle.ParseInterval("invalid") != time.Minute {
		t.Fatal("invalid should default to 1m")
	}
}

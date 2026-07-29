package screenshot

import (
	"os"
	"testing"

	"html-preview/internal/storage"
)

func TestWriteThumbsAtomically(t *testing.T) {
	dir := t.TempDir()
	id := "test-item"
	desktop := []byte("desktop-webp")
	mobile := []byte("mobile-webp")

	if err := writeThumbsAtomically(dir, id, desktop, mobile); err != nil {
		t.Fatalf("writeThumbsAtomically: %v", err)
	}

	desktopPath := storage.DesktopThumbPath(dir, id)
	mobilePath := storage.MobileThumbPath(dir, id)

	gotDesktop, err := os.ReadFile(desktopPath)
	if err != nil {
		t.Fatalf("read desktop: %v", err)
	}
	gotMobile, err := os.ReadFile(mobilePath)
	if err != nil {
		t.Fatalf("read mobile: %v", err)
	}
	if string(gotDesktop) != string(desktop) || string(gotMobile) != string(mobile) {
		t.Fatalf("unexpected thumb contents: desktop=%q mobile=%q", gotDesktop, gotMobile)
	}

	for _, path := range []string{desktopPath + ".tmp", mobilePath + ".tmp"} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("temp file should not remain: %s", path)
		}
	}
}

func TestWriteThumbsAtomicallyCleansUpOnFailure(t *testing.T) {
	dir := t.TempDir()
	id := "fail-item"
	itemDir := storage.ItemDir(dir, id)
	if err := os.MkdirAll(itemDir, 0o755); err != nil {
		t.Fatal(err)
	}

	mobilePath := storage.MobileThumbPath(dir, id)
	if err := os.Mkdir(mobilePath, 0o755); err != nil {
		t.Fatal(err)
	}

	err := writeThumbsAtomically(dir, id, []byte("new-desktop"), []byte("new-mobile"))
	if err == nil {
		t.Fatal("expected rename failure")
	}

	if _, err := os.Stat(storage.DesktopThumbPath(dir, id)); !os.IsNotExist(err) {
		t.Fatal("desktop final should not remain after failed rename")
	}
	if _, err := os.Stat(storage.DesktopThumbPath(dir, id) + ".tmp"); !os.IsNotExist(err) {
		t.Fatal("desktop tmp should be cleaned up")
	}
	if _, err := os.Stat(mobilePath + ".tmp"); !os.IsNotExist(err) {
		t.Fatal("mobile tmp should be cleaned up")
	}
}

package item

import (
	"bytes"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"html-preview/internal/db"
	"html-preview/internal/model"
)

// pinnedOld is a fixed past timestamp used to detect unwanted rewrites.
const pinnedOld = "2020-01-01T00:00:00Z"

func newFavoriteSvc(t *testing.T) (*Service, *sql.DB, func()) {
	t.Helper()
	dir := t.TempDir()
	conn, err := db.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	return &Service{DB: conn, DataDir: dir}, conn, func() { _ = conn.Close() }
}

func createFavItem(t *testing.T, svc *Service, title string) *model.Item {
	t.Helper()
	html := "<!DOCTYPE html><html><head><title>" + title + "</title></head><body></body></html>"
	it, err := svc.CreateFromUpload("", "", "never", "", title+".html", bytes.NewReader([]byte(html)), int64(len(html)))
	if err != nil {
		t.Fatalf("CreateFromUpload(%s): %v", title, err)
	}
	return it
}

func pinColumn(t *testing.T, conn *sql.DB, id, column, value string) {
	t.Helper()
	if _, err := conn.Exec("UPDATE items SET "+column+" = ? WHERE id = ?", value, id); err != nil {
		t.Fatalf("pin %s: %v", column, err)
	}
}

func TestSetFavorite_ToggleAndTimestamps(t *testing.T) {
	svc, _, cleanup := newFavoriteSvc(t)
	defer cleanup()

	it := createFavItem(t, svc, "Page")
	if it.Favorite || it.FavoritedAt != nil {
		t.Fatalf("new item should not be favorite, got favorite=%v favorited_at=%v", it.Favorite, it.FavoritedAt)
	}

	faved, err := svc.SetFavorite(it.ID, true)
	if err != nil {
		t.Fatalf("SetFavorite(true): %v", err)
	}
	if !faved.Favorite || faved.FavoritedAt == nil {
		t.Fatalf("expected favorite with timestamp, got favorite=%v favorited_at=%v", faved.Favorite, faved.FavoritedAt)
	}
	if _, err := time.Parse(time.RFC3339, *faved.FavoritedAt); err != nil {
		t.Fatalf("favorited_at not RFC3339: %v", err)
	}
	if faved.PublicPath == "" {
		t.Fatal("expected enriched public_path on returned item")
	}

	unfaved, err := svc.SetFavorite(it.ID, false)
	if err != nil {
		t.Fatalf("SetFavorite(false): %v", err)
	}
	if unfaved.Favorite || unfaved.FavoritedAt != nil {
		t.Fatalf("expected favorite cleared, got favorite=%v favorited_at=%v", unfaved.Favorite, unfaved.FavoritedAt)
	}
}

func TestSetFavorite_IdempotentFavoriteKeepsTimestamp(t *testing.T) {
	svc, conn, cleanup := newFavoriteSvc(t)
	defer cleanup()

	it := createFavItem(t, svc, "Page")
	if _, err := svc.SetFavorite(it.ID, true); err != nil {
		t.Fatalf("SetFavorite(true): %v", err)
	}
	pinColumn(t, conn, it.ID, "favorited_at", pinnedOld)

	again, err := svc.SetFavorite(it.ID, true)
	if err != nil {
		t.Fatalf("idempotent SetFavorite(true): %v", err)
	}
	if !again.Favorite || again.FavoritedAt == nil || *again.FavoritedAt != pinnedOld {
		t.Fatalf("expected favorite with pinned timestamp, got favorite=%v favorited_at=%v", again.Favorite, again.FavoritedAt)
	}

	var favorite bool
	var favoritedAt sql.NullString
	if err := conn.QueryRow(`SELECT favorite, favorited_at FROM items WHERE id = ?`, it.ID).
		Scan(&favorite, &favoritedAt); err != nil {
		t.Fatal(err)
	}
	if !favorite || !favoritedAt.Valid || favoritedAt.String != pinnedOld {
		t.Fatalf("db row changed on idempotent call: favorite=%v favorited_at=%v", favorite, favoritedAt)
	}
}

func TestSetFavorite_IdempotentUnfavorite(t *testing.T) {
	svc, _, cleanup := newFavoriteSvc(t)
	defer cleanup()

	it := createFavItem(t, svc, "Page")

	again, err := svc.SetFavorite(it.ID, false)
	if err != nil {
		t.Fatalf("idempotent SetFavorite(false): %v", err)
	}
	if again.Favorite || again.FavoritedAt != nil {
		t.Fatalf("expected favorite cleared, got favorite=%v favorited_at=%v", again.Favorite, again.FavoritedAt)
	}
}

func TestSetFavorite_RefavoriteWritesNewTimestamp(t *testing.T) {
	svc, conn, cleanup := newFavoriteSvc(t)
	defer cleanup()

	it := createFavItem(t, svc, "Page")
	if _, err := svc.SetFavorite(it.ID, true); err != nil {
		t.Fatalf("SetFavorite(true): %v", err)
	}
	pinColumn(t, conn, it.ID, "favorited_at", pinnedOld)

	if _, err := svc.SetFavorite(it.ID, false); err != nil {
		t.Fatalf("SetFavorite(false): %v", err)
	}

	again, err := svc.SetFavorite(it.ID, true)
	if err != nil {
		t.Fatalf("re-favorite: %v", err)
	}
	if !again.Favorite || again.FavoritedAt == nil || *again.FavoritedAt == pinnedOld {
		t.Fatalf("expected fresh timestamp on re-favorite, got favorite=%v favorited_at=%v", again.Favorite, again.FavoritedAt)
	}
}

func TestSetFavorite_DoesNotTouchUpdatedAt(t *testing.T) {
	svc, conn, cleanup := newFavoriteSvc(t)
	defer cleanup()

	it := createFavItem(t, svc, "Page")
	pinColumn(t, conn, it.ID, "updated_at", pinnedOld)

	faved, err := svc.SetFavorite(it.ID, true)
	if err != nil {
		t.Fatalf("SetFavorite(true): %v", err)
	}
	if faved.UpdatedAt != pinnedOld {
		t.Fatalf("favorite changed updated_at: %q", faved.UpdatedAt)
	}

	unfaved, err := svc.SetFavorite(it.ID, false)
	if err != nil {
		t.Fatalf("SetFavorite(false): %v", err)
	}
	if unfaved.UpdatedAt != pinnedOld {
		t.Fatalf("unfavorite changed updated_at: %q", unfaved.UpdatedAt)
	}

	var updatedAt string
	if err := conn.QueryRow(`SELECT updated_at FROM items WHERE id = ?`, it.ID).Scan(&updatedAt); err != nil {
		t.Fatal(err)
	}
	if updatedAt != pinnedOld {
		t.Fatalf("db updated_at changed: %q", updatedAt)
	}
}

func TestSetFavorite_TrashedConflict(t *testing.T) {
	svc, conn, cleanup := newFavoriteSvc(t)
	defer cleanup()

	it := createFavItem(t, svc, "Page")
	if _, err := svc.Trash(it.ID); err != nil {
		t.Fatalf("Trash: %v", err)
	}

	if _, err := svc.SetFavorite(it.ID, true); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict on favorite, got %v", err)
	}
	if _, err := svc.SetFavorite(it.ID, false); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict on unfavorite, got %v", err)
	}

	var favorite bool
	if err := conn.QueryRow(`SELECT favorite FROM items WHERE id = ?`, it.ID).Scan(&favorite); err != nil {
		t.Fatal(err)
	}
	if favorite {
		t.Fatal("trashed item must not become favorite")
	}
}

func TestSetFavorite_NotFound(t *testing.T) {
	svc, _, cleanup := newFavoriteSvc(t)
	defer cleanup()

	if _, err := svc.SetFavorite("missing", true); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestSetFavorite_PreservedAcrossTrashAndRestore(t *testing.T) {
	svc, conn, cleanup := newFavoriteSvc(t)
	defer cleanup()

	it := createFavItem(t, svc, "Page")
	if _, err := svc.SetFavorite(it.ID, true); err != nil {
		t.Fatalf("SetFavorite(true): %v", err)
	}
	pinColumn(t, conn, it.ID, "favorited_at", pinnedOld)

	trashed, err := svc.Trash(it.ID)
	if err != nil {
		t.Fatalf("Trash: %v", err)
	}
	if !trashed.Favorite || trashed.FavoritedAt == nil || *trashed.FavoritedAt != pinnedOld {
		t.Fatalf("trash lost favorite state: favorite=%v favorited_at=%v", trashed.Favorite, trashed.FavoritedAt)
	}

	restored, err := svc.Restore(it.ID)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if !restored.Favorite || restored.FavoritedAt == nil || *restored.FavoritedAt != pinnedOld {
		t.Fatalf("restore lost favorite state: favorite=%v favorited_at=%v", restored.Favorite, restored.FavoritedAt)
	}
}

func TestList_FavoriteFilter(t *testing.T) {
	svc, _, cleanup := newFavoriteSvc(t)
	defer cleanup()

	alpha := createFavItem(t, svc, "Alpha Report")
	beta := createFavItem(t, svc, "Beta Report")
	gamma := createFavItem(t, svc, "Gamma Notes")
	for _, id := range []string{alpha.ID, gamma.ID} {
		if _, err := svc.SetFavorite(id, true); err != nil {
			t.Fatalf("SetFavorite(%s): %v", id, err)
		}
	}

	result, err := svc.List("", "", true, 1, 24)
	if err != nil {
		t.Fatalf("List favorites: %v", err)
	}
	if result.Total != 2 || len(result.Items) != 2 {
		t.Fatalf("expected 2 favorites, got total=%d len=%d", result.Total, len(result.Items))
	}
	got := map[string]bool{}
	for _, it := range result.Items {
		if !it.Favorite {
			t.Fatalf("non-favorite %q in filtered list", it.ID)
		}
		got[it.ID] = true
	}
	if !got[alpha.ID] || !got[gamma.ID] || got[beta.ID] {
		t.Fatalf("unexpected favorite set: %v", got)
	}

	// q combines with the favorite filter.
	result, err = svc.List("report", "", true, 1, 24)
	if err != nil {
		t.Fatalf("List favorites with q: %v", err)
	}
	if result.Total != 1 || len(result.Items) != 1 || result.Items[0].ID != alpha.ID {
		t.Fatalf("expected only %q, got total=%d items=%v", alpha.ID, result.Total, result.Items)
	}

	// Without the filter everything active is listed.
	result, err = svc.List("", "", false, 1, 24)
	if err != nil {
		t.Fatalf("List without filter: %v", err)
	}
	if result.Total != 3 {
		t.Fatalf("expected all 3 active items, got %d", result.Total)
	}

	// Trashing a favorite drops it from the favorite listing.
	if _, err := svc.Trash(gamma.ID); err != nil {
		t.Fatalf("Trash: %v", err)
	}
	result, err = svc.List("", "", true, 1, 24)
	if err != nil {
		t.Fatalf("List favorites after trash: %v", err)
	}
	if result.Total != 1 || len(result.Items) != 1 || result.Items[0].ID != alpha.ID {
		t.Fatalf("expected only %q after trash, got total=%d items=%v", alpha.ID, result.Total, result.Items)
	}
}

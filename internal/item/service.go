package item

import (
	"bytes"
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"html-preview/internal/htmltitle"
	"html-preview/internal/model"
	"html-preview/internal/storage"
)

type Service struct {
	DB      *sql.DB
	DataDir string
}

type ListResult struct {
	Items    []*model.Item
	Page     int
	PageSize int
	Total    int
}

func (s *Service) CreateFromUpload(
	title, notes, expiresIn, expiresAt string,
	filename string,
	content io.Reader,
	size int64,
) (*model.Item, error) {
	filename = strings.TrimSpace(filename)
	if filename == "" {
		return nil, ErrFilenameRequired
	}

	sourceKind, err := sourceKindFromFilename(filename)
	if err != nil {
		return nil, err
	}

	expires, err := ParseExpires(expiresIn, expiresAt)
	if err != nil {
		return nil, err
	}

	id, err := GenerateID()
	if err != nil {
		return nil, err
	}

	storedSize, err := s.saveContent(id, sourceKind, filename, content, size)
	if err != nil {
		return nil, err
	}

	// Client title wins; otherwise parse <title>, then fall back to the filename.
	if strings.TrimSpace(title) == "" {
		title = s.parsedTitle(id)
	}
	if strings.TrimSpace(title) == "" {
		title = TitleFromFilename(filename)
	}

	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339)
	expiresStr := formatExpires(expires)

	_, err = s.DB.Exec(`
		INSERT INTO items (
			id, title, notes, status, source_kind, original_filename, size_bytes,
			expires_at, screenshot_status, created_at, updated_at
		) VALUES (?, ?, ?, 'active', ?, ?, ?, ?, 'pending', ?, ?)
	`, id, title, notes, sourceKind, filename, storedSize, expiresStr, nowStr, nowStr)
	if err != nil {
		os.RemoveAll(storage.ItemDir(s.DataDir, id))
		return nil, err
	}

	return s.GetByID(id)
}

func (s *Service) List(q, status string, page, pageSize int) (*ListResult, error) {
	if status == "" {
		status = "active"
	}
	if status != "active" && status != "trash" {
		return nil, ErrBadStatus
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 24
	}
	if pageSize > 100 {
		pageSize = 100
	}

	q = strings.TrimSpace(q)

	where := "status = ?"
	args := []any{status}
	if q != "" {
		where += " AND (title LIKE ? ESCAPE '\\' OR notes LIKE ? ESCAPE '\\')"
		pattern := "%" + escapeLike(q) + "%"
		args = append(args, pattern, pattern)
	}

	var total int
	if err := s.DB.QueryRow("SELECT COUNT(*) FROM items WHERE "+where, args...).Scan(&total); err != nil {
		return nil, err
	}

	offset := (page - 1) * pageSize
	query := `
		SELECT id, title, notes, status, source_kind, original_filename, size_bytes,
			expires_at, trashed_at, screenshot_status, screenshot_error, created_at, updated_at
		FROM items WHERE ` + where + ` ORDER BY updated_at DESC LIMIT ? OFFSET ?`
	listArgs := append(append([]any{}, args...), pageSize, offset)

	rows, err := s.DB.Query(query, listArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]*model.Item, 0)
	for rows.Next() {
		it, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		s.enrichItem(it)
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &ListResult{
		Items:    items,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}, nil
}

func (s *Service) Patch(id string, title, notes *string, expiresIn, expiresAt *string, updateExpires bool) (*model.Item, error) {
	it, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}
	if err := requireActive(it); err != nil {
		return nil, err
	}

	newTitle := it.Title
	if title != nil {
		newTitle = strings.TrimSpace(*title)
	}
	newNotes := it.Notes
	if notes != nil {
		newNotes = *notes
	}

	now := time.Now().UTC().Format(time.RFC3339)

	if updateExpires {
		var in, at string
		if expiresIn != nil {
			in = *expiresIn
		}
		if expiresAt != nil {
			at = *expiresAt
		}
		expires, err := ParseExpires(in, at)
		if err != nil {
			return nil, err
		}
		expiresStr := formatExpires(expires)
		_, err = s.DB.Exec(`
			UPDATE items SET title = ?, notes = ?, expires_at = ?, updated_at = ? WHERE id = ?
		`, newTitle, newNotes, expiresStr, now, id)
		if err != nil {
			return nil, err
		}
	} else {
		_, err = s.DB.Exec(`
			UPDATE items SET title = ?, notes = ?, updated_at = ? WHERE id = ?
		`, newTitle, newNotes, now, id)
		if err != nil {
			return nil, err
		}
	}

	return s.GetByID(id)
}

func (s *Service) ReplaceContent(id string, filename string, content io.Reader, size int64) (*model.Item, error) {
	filename = strings.TrimSpace(filename)
	if filename == "" {
		return nil, ErrFilenameRequired
	}

	it, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}
	if err := requireActive(it); err != nil {
		return nil, err
	}

	sourceKind, err := sourceKindFromFilename(filename)
	if err != nil {
		return nil, err
	}

	storedSize, err := storage.ReplaceItemContent(s.DataDir, id, func(stagingDir string) (int64, error) {
		return s.saveContentToDir(stagingDir, sourceKind, filename, content, size)
	})
	if err != nil {
		return nil, err
	}

	// Refresh the title from the new HTML only if it's empty or still the
	// filename default; never clobber a custom title.
	newTitle := it.Title
	if parsed := s.parsedTitle(id); parsed != "" {
		if it.Title == "" || it.Title == TitleFromFilename(it.OriginalFilename) {
			newTitle = parsed
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = s.DB.Exec(`
		UPDATE items SET
			source_kind = ?, original_filename = ?, size_bytes = ?,
			title = ?, screenshot_status = 'pending', screenshot_error = NULL, updated_at = ?
		WHERE id = ?
	`, sourceKind, filename, storedSize, newTitle, now, id)
	if err != nil {
		_ = storage.RollbackItemContent(s.DataDir, id)
		return nil, err
	}
	storage.CleanupItemContentBackup(s.DataDir, id)

	return s.GetByID(id)
}

func (s *Service) Trash(id string) (*model.Item, error) {
	it, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}
	if it.Status == "trash" {
		return nil, ErrConflict
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = s.DB.Exec(`
		UPDATE items SET status = 'trash', trashed_at = ?, updated_at = ? WHERE id = ?
	`, now, now, id)
	if err != nil {
		return nil, err
	}

	return s.GetByID(id)
}

func (s *Service) Restore(id string) (*model.Item, error) {
	it, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}
	if it.Status != "trash" {
		return nil, ErrConflict
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = s.DB.Exec(`
		UPDATE items SET status = 'active', trashed_at = NULL, updated_at = ? WHERE id = ?
	`, now, id)
	if err != nil {
		return nil, err
	}

	return s.GetByID(id)
}

func (s *Service) PermanentDelete(id string) error {
	if _, err := s.GetByID(id); err != nil {
		return err
	}

	if err := storage.RemoveItemDir(s.DataDir, id); err != nil {
		return err
	}

	_, err := s.DB.Exec(`DELETE FROM items WHERE id = ?`, id)
	return err
}

func (s *Service) GetByID(id string) (*model.Item, error) {
	row := s.DB.QueryRow(`
		SELECT id, title, notes, status, source_kind, original_filename, size_bytes,
			expires_at, trashed_at, screenshot_status, screenshot_error, created_at, updated_at
		FROM items WHERE id = ?
	`, id)

	it, err := scanItem(row)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	s.enrichItem(it)
	return it, nil
}

func (s *Service) IsPubliclyAvailable(it *model.Item) bool {
	if it == nil || it.Status != "active" {
		return false
	}
	if it.ExpiresAt == nil {
		return true
	}
	t, err := time.Parse(time.RFC3339, *it.ExpiresAt)
	if err != nil {
		return false
	}
	return t.After(time.Now().UTC())
}

func (s *Service) ItemContentDir(id string) string {
	return storage.ItemDir(s.DataDir, id)
}

func (s *Service) SetScreenshotStatus(id, status, errMsg string) error {
	if _, err := s.GetByID(id); err != nil {
		return err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	var screenshotError any
	if errMsg == "" {
		screenshotError = nil
	} else {
		screenshotError = errMsg
	}

	_, err := s.DB.Exec(`
		UPDATE items SET screenshot_status = ?, screenshot_error = ?, updated_at = ? WHERE id = ?
	`, status, screenshotError, now, id)
	return err
}

func (s *Service) ListScreenshotRetryIDs() ([]string, error) {
	rows, err := s.DB.Query(`
		SELECT id FROM items WHERE screenshot_status IN ('pending', 'failed')
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *Service) ResetNoBrowserToPending() ([]string, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := s.DB.Exec(`
		UPDATE items SET screenshot_status = 'pending', screenshot_error = NULL, updated_at = ?
		WHERE screenshot_status = 'no_browser'
	`, now); err != nil {
		return nil, err
	}

	rows, err := s.DB.Query(`SELECT id FROM items WHERE screenshot_status = 'pending'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *Service) ThumbPath(id, variant string) (string, error) {
	switch variant {
	case "desktop":
		return storage.DesktopThumbPath(s.DataDir, id), nil
	case "mobile":
		return storage.MobileThumbPath(s.DataDir, id), nil
	default:
		return "", ErrInvalidThumbVariant
	}
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanItem(row rowScanner) (*model.Item, error) {
	var it model.Item
	var expiresAt, trashedAt, screenshotError sql.NullString
	err := row.Scan(
		&it.ID, &it.Title, &it.Notes, &it.Status, &it.SourceKind, &it.OriginalFilename,
		&it.SizeBytes, &expiresAt, &trashedAt, &it.ScreenshotStatus, &screenshotError,
		&it.CreatedAt, &it.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if expiresAt.Valid {
		v := expiresAt.String
		it.ExpiresAt = &v
	}
	if trashedAt.Valid {
		v := trashedAt.String
		it.TrashedAt = &v
	}
	if screenshotError.Valid {
		v := screenshotError.String
		it.ScreenshotError = &v
	}
	return &it, nil
}

func (s *Service) enrichItem(it *model.Item) {
	it.PublicPath = PublicPath(it.ID)
	it.Thumbs = s.thumbsFor(it.ID, it.ScreenshotStatus)
}

func (s *Service) thumbsFor(id, screenshotStatus string) model.Thumbs {
	if screenshotStatus != "ready" {
		return model.Thumbs{}
	}
	desktopURL := "/api/items/" + id + "/thumb/desktop"
	mobileURL := "/api/items/" + id + "/thumb/mobile"
	return model.Thumbs{
		Desktop: &desktopURL,
		Mobile:  &mobileURL,
	}
}

func requireActive(it *model.Item) error {
	if it.Status == "trash" {
		return ErrConflict
	}
	return nil
}

func (s *Service) saveContent(id, sourceKind, filename string, content io.Reader, size int64) (int64, error) {
	return s.saveContentToDir(storage.ItemDir(s.DataDir, id), sourceKind, filename, content, size)
}

// parsedTitle extracts the title from the item's stored index.html; any open or
// parse error yields "".
func (s *Service) parsedTitle(id string) string {
	f, err := os.Open(storage.IndexHTMLPath(s.DataDir, id))
	if err != nil {
		return ""
	}
	defer f.Close()
	return htmltitle.ExtractHTMLTitle(f)
}

func (s *Service) saveContentToDir(dir, sourceKind, filename string, content io.Reader, size int64) (int64, error) {
	switch sourceKind {
	case "html":
		return storage.SaveHTMLToDir(dir, content, size)
	case "zip":
		data, readErr := io.ReadAll(io.LimitReader(content, storage.MaxZipBytes+1))
		if readErr != nil {
			return 0, readErr
		}
		if int64(len(data)) > storage.MaxZipBytes {
			return 0, storage.ErrZipTooLarge
		}
		reader := bytes.NewReader(data)
		return storage.ExtractZipToDir(dir, reader, int64(len(data)), storage.UploadZipOpts())
	default:
		return 0, ErrUnsupportedFileType
	}
}

func sourceKindFromFilename(filename string) (string, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".html", ".htm":
		return "html", nil
	case ".zip":
		return "zip", nil
	default:
		return "", ErrUnsupportedFileType
	}
}

func formatExpires(expires *time.Time) *string {
	if expires == nil {
		return nil
	}
	v := expires.Format(time.RFC3339)
	return &v
}

func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

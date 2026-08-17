package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"

	"html-preview/internal/item"
	"html-preview/internal/storage"
)

func (s *Server) ListItems(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	status := r.URL.Query().Get("status")

	favorite := r.URL.Query().Get("favorite")
	if favorite != "" && favorite != "true" {
		s.writeErr(w, 40001, "invalid favorite")
		return
	}

	page := 1
	if v := r.URL.Query().Get("page"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			s.writeErr(w, 40001, "invalid page")
			return
		}
		page = n
	}

	pageSize := 24
	if v := r.URL.Query().Get("page_size"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			s.writeErr(w, 40001, "invalid page_size")
			return
		}
		pageSize = n
	}

	result, err := s.Items.List(q, status, favorite == "true", page, pageSize)
	if err != nil {
		if errors.Is(err, item.ErrBadStatus) {
			s.writeErr(w, 40001, "invalid status")
			return
		}
		s.writeErr(w, 50000, "internal error")
		return
	}

	writeOK(w, map[string]any{
		"items":     result.Items,
		"page":      result.Page,
		"page_size": result.PageSize,
		"total":     result.Total,
	})
}

func (s *Server) CreateItem(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(22 << 20); err != nil {
		s.writeErr(w, 40001, "invalid multipart form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		s.writeErr(w, 40001, "file is required")
		return
	}
	defer file.Close()

	title := strings.TrimSpace(r.FormValue("title"))
	notes := r.FormValue("notes")
	expiresIn := r.FormValue("expires_in")
	expiresAt := r.FormValue("expires_at")

	size := header.Size
	if size < 0 {
		size = 0
	}

	it, err := s.Items.CreateFromUpload(title, notes, expiresIn, expiresAt, header.Filename, file, size)
	if err != nil {
		code, msg := mapItemError(err)
		s.writeErr(w, code, msg)
		return
	}

	if s.Screenshot != nil {
		s.Screenshot.HandleNewItem(it.ID)
		if refreshed, err := s.Items.GetByID(it.ID); err == nil {
			it = refreshed
		}
	}

	writeOK(w, it)
}

func (s *Server) GetItem(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		s.writeErr(w, 40001, "id is required")
		return
	}

	it, err := s.Items.GetByID(id)
	if err != nil {
		code, msg := mapItemError(err)
		s.writeErr(w, code, msg)
		return
	}

	writeOK(w, it)
}

type patchItemRequest struct {
	Title     *string `json:"title"`
	Notes     *string `json:"notes"`
	ExpiresIn *string `json:"expires_in"`
	ExpiresAt *string `json:"expires_at"`
}

func (s *Server) PatchItem(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		s.writeErr(w, 40001, "id is required")
		return
	}

	var raw map[string]json.RawMessage
	if err := decodeJSON(r, &raw); err != nil {
		s.writeErr(w, 40001, "invalid request body")
		return
	}
	if len(raw) == 0 {
		s.writeErr(w, 40001, "empty request body")
		return
	}

	var req patchItemRequest
	for key, val := range raw {
		switch key {
		case "title":
			if err := json.Unmarshal(val, &req.Title); err != nil {
				s.writeErr(w, 40001, "invalid request body")
				return
			}
		case "notes":
			if err := json.Unmarshal(val, &req.Notes); err != nil {
				s.writeErr(w, 40001, "invalid request body")
				return
			}
		case "expires_in":
			if err := json.Unmarshal(val, &req.ExpiresIn); err != nil {
				s.writeErr(w, 40001, "invalid request body")
				return
			}
		case "expires_at":
			if err := json.Unmarshal(val, &req.ExpiresAt); err != nil {
				s.writeErr(w, 40001, "invalid request body")
				return
			}
		}
	}

	updateExpires := raw["expires_in"] != nil || raw["expires_at"] != nil

	it, err := s.Items.Patch(id, req.Title, req.Notes, req.ExpiresIn, req.ExpiresAt, updateExpires)
	if err != nil {
		code, msg := mapItemError(err)
		s.writeErr(w, code, msg)
		return
	}

	writeOK(w, it)
}

func (s *Server) ReplaceItemContent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		s.writeErr(w, 40001, "id is required")
		return
	}

	if err := r.ParseMultipartForm(22 << 20); err != nil {
		s.writeErr(w, 40001, "invalid multipart form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		s.writeErr(w, 40001, "file is required")
		return
	}
	defer file.Close()

	size := header.Size
	if size < 0 {
		size = 0
	}

	it, err := s.Items.ReplaceContent(id, header.Filename, file, size)
	if err != nil {
		code, msg := mapItemError(err)
		s.writeErr(w, code, msg)
		return
	}

	if s.Screenshot != nil {
		s.Screenshot.HandleReplacedItem(it.ID)
		if refreshed, err := s.Items.GetByID(it.ID); err == nil {
			it = refreshed
		}
	}

	writeOK(w, it)
}

func (s *Server) TrashItem(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		s.writeErr(w, 40001, "id is required")
		return
	}

	it, err := s.Items.Trash(id)
	if err != nil {
		code, msg := mapItemError(err)
		s.writeErr(w, code, msg)
		return
	}

	writeOK(w, it)
}

func (s *Server) RestoreItem(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		s.writeErr(w, 40001, "id is required")
		return
	}

	it, err := s.Items.Restore(id)
	if err != nil {
		code, msg := mapItemError(err)
		s.writeErr(w, code, msg)
		return
	}

	writeOK(w, it)
}

func (s *Server) FavoriteItem(w http.ResponseWriter, r *http.Request) {
	s.setItemFavorite(w, r, true)
}

func (s *Server) UnfavoriteItem(w http.ResponseWriter, r *http.Request) {
	s.setItemFavorite(w, r, false)
}

func (s *Server) setItemFavorite(w http.ResponseWriter, r *http.Request, favorite bool) {
	id := r.PathValue("id")
	if id == "" {
		s.writeErr(w, 40001, "id is required")
		return
	}

	it, err := s.Items.SetFavorite(id, favorite)
	if err != nil {
		code, msg := mapItemError(err)
		s.writeErr(w, code, msg)
		return
	}

	writeOK(w, it)
}

func (s *Server) PermanentDeleteItem(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		s.writeErr(w, 40001, "id is required")
		return
	}

	if err := s.Items.PermanentDelete(id); err != nil {
		code, msg := mapItemError(err)
		s.writeErr(w, code, msg)
		return
	}

	writeOK(w, map[string]bool{"ok": true})
}

func (s *Server) ServeItemThumb(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	variant := r.PathValue("variant")
	if id == "" || variant == "" {
		s.writeErr(w, 40001, "id is required")
		return
	}

	it, err := s.Items.GetByID(id)
	if err != nil {
		code, msg := mapItemError(err)
		s.writeErr(w, code, msg)
		return
	}
	_ = it

	path, err := s.Items.ThumbPath(id, variant)
	if err != nil {
		s.writeErr(w, 40001, "invalid thumb variant")
		return
	}

	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			s.writeErr(w, 40400, "thumb not found")
			return
		}
		s.writeErr(w, 50000, "internal error")
		return
	}

	w.Header().Set("Content-Type", "image/webp")
	http.ServeFile(w, r, path)
}

func mapItemError(err error) (int, string) {
	switch {
	case errors.Is(err, item.ErrNotFound):
		return 40400, "item not found"
	case errors.Is(err, item.ErrConflict):
		return 40900, "item state conflict"
	case errors.Is(err, item.ErrBadStatus):
		return 40001, "invalid status"
	case errors.Is(err, item.ErrFilenameRequired),
		errors.Is(err, item.ErrUnsupportedFileType),
		errors.Is(err, item.ErrInvalidExpiresAt),
		errors.Is(err, item.ErrInvalidExpiresIn),
		errors.Is(err, item.ErrInvalidThumbVariant),
		errors.Is(err, storage.ErrZipInvalid),
		errors.Is(err, storage.ErrZipTooManyFiles),
		errors.Is(err, storage.ErrZipPathTraversal),
		errors.Is(err, storage.ErrZipAbsolutePath),
		errors.Is(err, storage.ErrZipSymlink),
		errors.Is(err, storage.ErrZipNested),
		errors.Is(err, storage.ErrZipMissingIndex),
		errors.Is(err, storage.ErrZipEmptyPath),
		errors.Is(err, storage.ErrZipBackslashPath):
		return 40001, err.Error()
	case errors.Is(err, storage.ErrHTMLTooLarge),
		errors.Is(err, storage.ErrZipTooLarge),
		errors.Is(err, storage.ErrZipDecompressedTooLarge):
		return 41300, err.Error()
	default:
		return 50000, "internal error"
	}
}

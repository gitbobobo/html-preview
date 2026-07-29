package api

import (
	"errors"
	"net/http"

	"html-preview/internal/auth"
)

func (s *Server) ListKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := auth.ListAPIKeys(s.DB)
	if err != nil {
		s.writeErr(w, 50000, "internal error")
		return
	}
	if keys == nil {
		keys = []auth.APIKey{}
	}
	writeOK(w, keys)
}

type createKeyRequest struct {
	Name string `json:"name"`
}

func (s *Server) CreateKey(w http.ResponseWriter, r *http.Request) {
	var req createKeyRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeErr(w, 40001, "invalid request body")
		return
	}

	created, err := auth.CreateAPIKey(s.DB, req.Name)
	if err != nil {
		if errors.Is(err, auth.ErrNameRequired) {
			s.writeErr(w, 40001, err.Error())
			return
		}
		s.writeErr(w, 50000, "internal error")
		return
	}
	writeOK(w, created)
}

func (s *Server) RevokeKey(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		s.writeErr(w, 40001, "id is required")
		return
	}

	ok, err := auth.RevokeAPIKey(s.DB, id)
	if err != nil {
		s.writeErr(w, 50000, "internal error")
		return
	}
	if !ok {
		s.writeErr(w, 40400, "api key not found")
		return
	}
	writeOK(w, map[string]bool{"ok": true})
}

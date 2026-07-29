package api

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"

	"html-preview/internal/auth"
	"html-preview/internal/config"
	"html-preview/internal/item"
	"html-preview/internal/screenshot"
)

type Server struct {
	DB         *sql.DB
	DataDir    string
	Config     config.Config
	Screenshot *screenshot.Service
	Items      *item.Service
}

func (s *Server) Status(w http.ResponseWriter, r *http.Request) {
	initialized, err := auth.IsInitialized(s.DB)
	if err != nil {
		s.writeErr(w, 50000, "internal error")
		return
	}

	authenticated := false
	if _, ok := auth.AuthenticateSession(s.DB, r); ok {
		authenticated = true
	}

	writeOK(w, map[string]bool{
		"initialized":   initialized,
		"authenticated": authenticated,
	})
}

type setupRequest struct {
	Password string `json:"password"`
}

func (s *Server) Setup(w http.ResponseWriter, r *http.Request) {
	initialized, err := auth.IsInitialized(s.DB)
	if err != nil {
		s.writeErr(w, 50000, "internal error")
		return
	}
	if initialized {
		s.writeErr(w, 40900, "already initialized")
		return
	}

	var req setupRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeErr(w, 40001, "invalid request body")
		return
	}
	if err := auth.ValidatePassword(req.Password); err != nil {
		s.writeErr(w, 40001, err.Error())
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		s.writeErr(w, 50000, "internal error")
		return
	}
	if err := auth.SetPasswordHash(s.DB, hash); err != nil {
		s.writeErr(w, 50000, "internal error")
		return
	}

	token, err := auth.CreateSession(s.DB)
	if err != nil {
		s.writeErr(w, 50000, "internal error")
		return
	}
	auth.SetSessionCookie(w, r, token)
	writeOK(w, map[string]bool{"ok": true})
}

type loginRequest struct {
	Password string `json:"password"`
}

func (s *Server) Login(w http.ResponseWriter, r *http.Request) {
	initialized, err := auth.IsInitialized(s.DB)
	if err != nil {
		s.writeErr(w, 50000, "internal error")
		return
	}
	if !initialized {
		s.writeErr(w, 40300, "not initialized")
		return
	}

	var req loginRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeErr(w, 40001, "invalid request body")
		return
	}

	hash, ok, err := auth.GetPasswordHash(s.DB)
	if err != nil {
		s.writeErr(w, 50000, "internal error")
		return
	}
	if !ok || !auth.VerifyPassword(hash, req.Password) {
		s.writeErr(w, 40100, "invalid password")
		return
	}

	token, err := auth.CreateSession(s.DB)
	if err != nil {
		s.writeErr(w, 50000, "internal error")
		return
	}
	auth.SetSessionCookie(w, r, token)
	writeOK(w, map[string]bool{"ok": true})
}

func (s *Server) Logout(w http.ResponseWriter, r *http.Request) {
	info, ok := auth.InfoFromContext(r.Context())
	if !ok || info.Method != auth.MethodSession {
		s.writeErr(w, 40100, "not authenticated")
		return
	}
	_ = auth.DeleteSession(s.DB, info.SessionID)
	auth.ClearSessionCookie(w, r)
	writeOK(w, map[string]bool{"ok": true})
}

type changePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

func (s *Server) ChangePassword(w http.ResponseWriter, r *http.Request) {
	var req changePasswordRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeErr(w, 40001, "invalid request body")
		return
	}
	if err := auth.ValidatePassword(req.NewPassword); err != nil {
		s.writeErr(w, 40001, err.Error())
		return
	}

	hash, ok, err := auth.GetPasswordHash(s.DB)
	if err != nil {
		s.writeErr(w, 50000, "internal error")
		return
	}
	if !ok || !auth.VerifyPassword(hash, req.OldPassword) {
		s.writeErr(w, 40100, "invalid password")
		return
	}

	newHash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		s.writeErr(w, 50000, "internal error")
		return
	}
	if err := auth.SetPasswordHash(s.DB, newHash); err != nil {
		s.writeErr(w, 50000, "internal error")
		return
	}
	writeOK(w, map[string]bool{"ok": true})
}

func decodeJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return err
	}
	if len(body) == 0 {
		return io.EOF
	}
	return json.Unmarshal(body, v)
}

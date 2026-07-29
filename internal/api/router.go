package api

import (
	"database/sql"
	"io/fs"
	"net/http"
	"strings"

	"html-preview/internal/auth"
	"html-preview/internal/config"
	"html-preview/internal/item"
	"html-preview/internal/screenshot"
)

func NewRouter(db *sql.DB, cfg config.Config, web fs.FS, ss *screenshot.Service) http.Handler {
	mux := http.NewServeMux()
	srv := &Server{
		DB:         db,
		DataDir:    cfg.DataDir,
		Config:     cfg,
		Screenshot: ss,
		Items:      &item.Service{DB: db, DataDir: cfg.DataDir},
	}

	mux.HandleFunc("GET /api/auth/status", srv.Status)
	mux.HandleFunc("POST /api/auth/setup", srv.Setup)
	mux.HandleFunc("POST /api/auth/login", srv.Login)
	mux.Handle("POST /api/auth/logout", auth.RequireSession(db, srv.writeErr, http.HandlerFunc(srv.Logout)))
	mux.Handle("POST /api/auth/password", auth.RequireSession(db, srv.writeErr, http.HandlerFunc(srv.ChangePassword)))

	mux.Handle("GET /api/keys", auth.RequireSession(db, srv.writeErr, http.HandlerFunc(srv.ListKeys)))
	mux.Handle("POST /api/keys", auth.RequireSession(db, srv.writeErr, http.HandlerFunc(srv.CreateKey)))
	mux.Handle("DELETE /api/keys/{id}", auth.RequireSession(db, srv.writeErr, http.HandlerFunc(srv.RevokeKey)))

	mux.Handle("GET /api/items", auth.RequireAuth(db, srv.writeErr, http.HandlerFunc(srv.ListItems)))
	mux.Handle("POST /api/items", auth.RequireAuth(db, srv.writeErr, http.HandlerFunc(srv.CreateItem)))
	mux.Handle("GET /api/items/{id}/thumb/{variant}", auth.RequireAuth(db, srv.writeErr, http.HandlerFunc(srv.ServeItemThumb)))
	mux.Handle("PUT /api/items/{id}/content", auth.RequireAuth(db, srv.writeErr, http.HandlerFunc(srv.ReplaceItemContent)))
	mux.Handle("POST /api/items/{id}/restore", auth.RequireAuth(db, srv.writeErr, http.HandlerFunc(srv.RestoreItem)))
	mux.Handle("DELETE /api/items/{id}/permanent", auth.RequireAuth(db, srv.writeErr, http.HandlerFunc(srv.PermanentDeleteItem)))
	mux.Handle("GET /api/items/{id}", auth.RequireAuth(db, srv.writeErr, http.HandlerFunc(srv.GetItem)))
	mux.Handle("PATCH /api/items/{id}", auth.RequireAuth(db, srv.writeErr, http.HandlerFunc(srv.PatchItem)))
	mux.Handle("DELETE /api/items/{id}", auth.RequireAuth(db, srv.writeErr, http.HandlerFunc(srv.TrashItem)))

	mux.Handle("GET /api/settings/info", auth.RequireSession(db, srv.writeErr, http.HandlerFunc(srv.SettingsInfo)))
	mux.Handle("GET /api/settings/browser", auth.RequireSession(db, srv.writeErr, http.HandlerFunc(srv.SettingsBrowser)))
	mux.Handle("POST /api/settings/browser/install", auth.RequireSession(db, srv.writeErr, http.HandlerFunc(srv.SettingsBrowserInstall)))

	mux.HandleFunc("GET /c/{id}", srv.ServePublicRedirect)
	mux.HandleFunc("GET /c/{id}/{path...}", srv.ServePublic)

	fileServer := http.FileServer(http.FS(web))
	mux.Handle("GET /{$}", spaHandler(web, fileServer))
	mux.Handle("GET /trash", spaHandler(web, fileServer))
	mux.Handle("GET /settings", spaHandler(web, fileServer))
	mux.Handle("GET /setup", spaHandler(web, fileServer))
	mux.Handle("GET /login", spaHandler(web, fileServer))
	mux.Handle("GET /css/{path...}", fileServer)
	mux.Handle("GET /js/{path...}", fileServer)
	mux.Handle("GET /assets/{path...}", fileServer)

	return mux
}

// spaHandler serves static embed files when present, otherwise index.html for SPA routes.
func spaHandler(web fs.FS, fileServer http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/c/") {
			http.NotFound(w, r)
			return
		}
		clean := strings.TrimPrefix(r.URL.Path, "/")
		if clean != "" {
			if info, err := fs.Stat(web, clean); err == nil && !info.IsDir() {
				fileServer.ServeHTTP(w, r)
				return
			}
		}
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/"
		fileServer.ServeHTTP(w, r2)
	})
}

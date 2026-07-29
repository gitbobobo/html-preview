package api

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

)

func (s *Server) ServePublicRedirect(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	http.Redirect(w, r, "/c/"+id+"/", http.StatusFound)
}

func (s *Server) ServePublic(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		s.serveNotFound(w)
		return
	}

	svc := s.Items
	it, err := svc.GetByID(id)
	if err != nil {
		s.serveNotFound(w)
		return
	}
	if !svc.IsPubliclyAvailable(it) {
		s.serveNotFound(w)
		return
	}

	subPath := r.PathValue("path")
	if subPath == "" {
		subPath = "index.html"
	}
	subPath = path.Clean(subPath)
	if subPath == "." || strings.HasPrefix(subPath, "../") || subPath == ".." {
		s.serveNotFound(w)
		return
	}

	root := svc.ItemContentDir(id)
	full := filepath.Join(root, filepath.FromSlash(subPath))
	rel, err := filepath.Rel(root, full)
	if err != nil || strings.HasPrefix(rel, "..") {
		s.serveNotFound(w)
		return
	}

	info, err := os.Stat(full)
	if err != nil || info.IsDir() {
		if subPath == "index.html" || strings.HasSuffix(r.URL.Path, "/") {
			index := filepath.Join(root, "index.html")
			if _, err := os.Stat(index); err == nil {
				full = index
			} else {
				s.serveNotFound(w)
				return
			}
		} else {
			s.serveNotFound(w)
			return
		}
	}

	setPublicHeaders(w)
	http.ServeFile(w, r, full)
}

func setPublicHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Security-Policy", "sandbox allow-scripts allow-same-origin; frame-ancestors 'none'")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
}

func (s *Server) serveNotFound(w http.ResponseWriter) {
	setPublicHeaders(w)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte("Not found"))
}

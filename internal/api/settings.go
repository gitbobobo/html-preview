package api

import (
	"net/http"
	"strconv"
)

func (s *Server) SettingsInfo(w http.ResponseWriter, r *http.Request) {
	writeOK(w, map[string]any{
		"data_dir":  s.Screenshot.DataDirPath(),
		"host":      s.Config.Host,
		"port":      s.Config.Port,
		"local_url": "http://127.0.0.1:" + strconv.Itoa(s.Config.Port) + "/",
		"lan_urls":  s.Config.LANURLs(),
	})
}

func (s *Server) SettingsBrowser(w http.ResponseWriter, r *http.Request) {
	writeOK(w, s.Screenshot.BrowserStatus())
}

func (s *Server) SettingsBrowserInstall(w http.ResponseWriter, r *http.Request) {
	state := s.Screenshot.StartInstall()
	writeOK(w, map[string]any{
		"status":  state.Status,
		"message": state.Message,
	})
}

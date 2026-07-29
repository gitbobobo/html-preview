package api

import (
	"encoding/json"
	"net/http"
)

type envelope struct {
	Code    int    `json:"code"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeOK(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusOK, envelope{Code: 0, Data: data})
}

func writeAPIError(w http.ResponseWriter, code int, message string) {
	status := httpStatusForCode(code)
	writeJSON(w, status, envelope{Code: code, Message: message})
}

func httpStatusForCode(code int) int {
	switch code {
	case 40001:
		return http.StatusBadRequest
	case 40100:
		return http.StatusUnauthorized
	case 40300:
		return http.StatusForbidden
	case 40400:
		return http.StatusNotFound
	case 40900:
		return http.StatusConflict
	case 41300:
		return http.StatusRequestEntityTooLarge
	default:
		return http.StatusInternalServerError
	}
}

func (s *Server) writeErr(w http.ResponseWriter, code int, message string) {
	writeAPIError(w, code, message)
}

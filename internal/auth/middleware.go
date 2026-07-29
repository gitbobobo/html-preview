package auth

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
)

type Method string

const (
	MethodSession Method = "session"
	MethodAPIKey  Method = "apikey"
)

type Info struct {
	Method    Method
	SessionID string
	APIKeyID  string
}

type contextKey int

const authInfoKey contextKey = 1

func InfoFromContext(ctx context.Context) (Info, bool) {
	v, ok := ctx.Value(authInfoKey).(Info)
	return v, ok
}

func HasBearer(r *http.Request) bool {
	return strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ")
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return ""
	}
	return strings.TrimSpace(h[len("Bearer "):])
}

func AuthenticateRequest(db *sql.DB, r *http.Request) (Info, bool) {
	token := SessionTokenFromRequest(r)
	if token != "" && ValidateSession(db, token) {
		return Info{Method: MethodSession, SessionID: token}, true
	}
	key := bearerToken(r)
	if key != "" {
		id, err := AuthenticateAPIKey(db, key)
		if err == nil {
			return Info{Method: MethodAPIKey, APIKeyID: id}, true
		}
	}
	return Info{}, false
}

func AuthenticateSession(db *sql.DB, r *http.Request) (Info, bool) {
	token := SessionTokenFromRequest(r)
	if token == "" {
		return Info{}, false
	}
	if !ValidateSession(db, token) {
		return Info{}, false
	}
	return Info{Method: MethodSession, SessionID: token}, true
}

type ErrorWriter func(w http.ResponseWriter, code int, message string)

func RequireSession(db *sql.DB, writeErr ErrorWriter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if HasBearer(r) {
			writeErr(w, 40300, "session required")
			return
		}
		info, ok := AuthenticateSession(db, r)
		if !ok {
			writeErr(w, 40100, "not authenticated")
			return
		}
		ctx := context.WithValue(r.Context(), authInfoKey, info)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func RequireAuth(db *sql.DB, writeErr ErrorWriter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info, ok := AuthenticateRequest(db, r)
		if !ok {
			writeErr(w, 40100, "not authenticated")
			return
		}
		ctx := context.WithValue(r.Context(), authInfoKey, info)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

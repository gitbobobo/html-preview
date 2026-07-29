package auth

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
	"net/http"
	"time"
)

const (
	CookieName      = "hp_session"
	SessionDuration = 7 * 24 * time.Hour
)

func generateToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func CreateSession(db *sql.DB) (string, error) {
	token, err := generateToken(32)
	if err != nil {
		return "", err
	}
	now := time.Now().UTC()
	expires := now.Add(SessionDuration)
	_, err = db.Exec(
		`INSERT INTO sessions (id, created_at, expires_at) VALUES (?, ?, ?)`,
		token, now.Format(time.RFC3339), expires.Format(time.RFC3339),
	)
	if err != nil {
		return "", err
	}
	return token, nil
}

func ValidateSession(db *sql.DB, token string) bool {
	if token == "" {
		return false
	}
	var expiresAt string
	err := db.QueryRow(
		`SELECT expires_at FROM sessions WHERE id = ?`, token,
	).Scan(&expiresAt)
	if err != nil {
		return false
	}
	expires, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil || time.Now().UTC().After(expires) {
		_, _ = db.Exec(`DELETE FROM sessions WHERE id = ?`, token)
		return false
	}
	return true
}

func DeleteSession(db *sql.DB, token string) error {
	_, err := db.Exec(`DELETE FROM sessions WHERE id = ?`, token)
	return err
}

func SetSessionCookie(w http.ResponseWriter, r *http.Request, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   isSecureRequest(r),
		MaxAge:   int(SessionDuration.Seconds()),
	})
}

func ClearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   isSecureRequest(r),
		MaxAge:   -1,
	})
}

func SessionTokenFromRequest(r *http.Request) string {
	c, err := r.Cookie(CookieName)
	if err != nil {
		return ""
	}
	return c.Value
}

func isSecureRequest(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return r.Header.Get("X-Forwarded-Proto") == "https"
}

func RenewSession(db *sql.DB, token string) error {
	expires := time.Now().UTC().Add(SessionDuration)
	res, err := db.Exec(
		`UPDATE sessions SET expires_at = ? WHERE id = ?`,
		expires.Format(time.RFC3339), token,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("session not found")
	}
	return nil
}

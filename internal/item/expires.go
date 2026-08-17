package item

import (
	"crypto/rand"
	"encoding/base64"
	"path/filepath"
	"strings"
	"time"
)

func ParseExpires(expiresIn, expiresAt string) (*time.Time, error) {
	expiresAt = strings.TrimSpace(expiresAt)
	expiresIn = strings.TrimSpace(expiresIn)

	if expiresAt != "" {
		t, err := time.Parse(time.RFC3339, expiresAt)
		if err != nil {
			return nil, ErrInvalidExpiresAt
		}
		utc := t.UTC()
		return &utc, nil
	}

	if expiresIn == "never" {
		return nil, nil
	}
	if expiresIn == "" {
		expiresIn = "30d"
	}

	now := time.Now().UTC()
	switch expiresIn {
	case "1d":
		t := now.Add(24 * time.Hour)
		return &t, nil
	case "7d":
		t := now.Add(7 * 24 * time.Hour)
		return &t, nil
	case "30d":
		t := now.Add(30 * 24 * time.Hour)
		return &t, nil
	case "90d":
		t := now.Add(90 * 24 * time.Hour)
		return &t, nil
	default:
		return nil, ErrInvalidExpiresIn
	}
}

func GenerateID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func PublicPath(id string) string {
	return "/c/" + id + "/"
}

// TitleFromFilename derives a default item title from an upload filename: it
// strips any path separators and removes one trailing extension (e.g.
// "page.html" -> "page", "archive.tar.gz" -> "archive.tar").
func TitleFromFilename(filename string) string {
	base := strings.TrimSpace(filename)
	if base == "" {
		return ""
	}
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[i+1:]
	}
	if i := strings.LastIndex(base, "\\"); i >= 0 {
		base = base[i+1:]
	}
	if ext := filepath.Ext(base); ext != "" {
		base = strings.TrimSuffix(base, ext)
	}
	return base
}

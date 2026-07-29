package item

import (
	"crypto/rand"
	"encoding/base64"
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

	if expiresIn == "" || expiresIn == "never" {
		return nil, nil
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

func TitleFromFilename(filename string) string {
	filename = strings.TrimSpace(filename)
	if filename == "" {
		return ""
	}
	if i := strings.LastIndex(filename, "/"); i >= 0 {
		filename = filename[i+1:]
	}
	if i := strings.LastIndex(filename, "\\"); i >= 0 {
		filename = filename[i+1:]
	}
	return filename
}

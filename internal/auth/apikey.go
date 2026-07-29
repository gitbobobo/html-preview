package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

const (
	apiKeyPrefix    = "hp_"
	apiKeyEntropy   = 32
	displayPrefixLen = 12
)

type APIKey struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	KeyPrefix  string  `json:"key_prefix"`
	CreatedAt  string  `json:"created_at"`
	LastUsedAt *string `json:"last_used_at"`
}

type APIKeyCreated struct {
	APIKey
	Key string `json:"key"`
}

func HashAPIKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

func generateAPIKey() (plain, prefix, hash string, err error) {
	b := make([]byte, apiKeyEntropy)
	if _, err = rand.Read(b); err != nil {
		return "", "", "", err
	}
	plain = apiKeyPrefix + base64.RawURLEncoding.EncodeToString(b)
	if len(plain) < displayPrefixLen {
		prefix = plain
	} else {
		prefix = plain[:displayPrefixLen]
	}
	hash = HashAPIKey(plain)
	return plain, prefix, hash, nil
}

func generateID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func CreateAPIKey(db *sql.DB, name string) (*APIKeyCreated, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrNameRequired
	}

	id, err := generateID()
	if err != nil {
		return nil, err
	}
	plain, prefix, hash, err := generateAPIKey()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = db.Exec(
		`INSERT INTO api_keys (id, name, key_prefix, key_hash, created_at) VALUES (?, ?, ?, ?, ?)`,
		id, name, prefix, hash, now,
	)
	if err != nil {
		return nil, err
	}
	return &APIKeyCreated{
		APIKey: APIKey{
			ID:        id,
			Name:      name,
			KeyPrefix: prefix,
			CreatedAt: now,
		},
		Key: plain,
	}, nil
}

func ListAPIKeys(db *sql.DB) ([]APIKey, error) {
	rows, err := db.Query(`
		SELECT id, name, key_prefix, created_at, last_used_at
		FROM api_keys
		WHERE revoked_at IS NULL
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []APIKey
	for rows.Next() {
		var k APIKey
		var lastUsed sql.NullString
		if err := rows.Scan(&k.ID, &k.Name, &k.KeyPrefix, &k.CreatedAt, &lastUsed); err != nil {
			return nil, err
		}
		if lastUsed.Valid {
			v := lastUsed.String
			k.LastUsedAt = &v
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

func RevokeAPIKey(db *sql.DB, id string) (bool, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := db.Exec(
		`UPDATE api_keys SET revoked_at = ? WHERE id = ? AND revoked_at IS NULL`,
		now, id,
	)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func AuthenticateAPIKey(db *sql.DB, key string) (string, error) {
	if !strings.HasPrefix(key, apiKeyPrefix) {
		return "", fmt.Errorf("invalid api key")
	}
	hash := HashAPIKey(key)
	var id string
	var revoked sql.NullString
	err := db.QueryRow(
		`SELECT id, revoked_at FROM api_keys WHERE key_hash = ?`, hash,
	).Scan(&id, &revoked)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("invalid api key")
	}
	if err != nil {
		return "", err
	}
	if revoked.Valid {
		return "", fmt.Errorf("api key revoked")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, _ = db.Exec(`UPDATE api_keys SET last_used_at = ? WHERE id = ?`, now, id)
	return id, nil
}

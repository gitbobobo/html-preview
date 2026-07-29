package auth

import (
	"database/sql"
	"time"
)

func GetPasswordHash(db *sql.DB) (string, bool, error) {
	var hash sql.NullString
	err := db.QueryRow(`SELECT password_hash FROM settings WHERE id = 1`).Scan(&hash)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if !hash.Valid || hash.String == "" {
		return "", false, nil
	}
	return hash.String, true, nil
}

func SetPasswordHash(db *sql.DB, passwordHash string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := db.Exec(
		`UPDATE settings SET password_hash = ?, updated_at = ? WHERE id = 1`,
		passwordHash, now,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	_, err = db.Exec(
		`INSERT INTO settings (id, password_hash, created_at, updated_at) VALUES (1, ?, ?, ?)`,
		passwordHash, now, now,
	)
	return err
}

func IsInitialized(db *sql.DB) (bool, error) {
	_, ok, err := GetPasswordHash(db)
	return ok, err
}

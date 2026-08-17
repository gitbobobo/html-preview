package db

import (
	"database/sql"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"html-preview/migrations"
)

func Open(path string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)", path)
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, err
	}
	if err := migrate(conn); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

// migration is a single embedded versioned SQL file.
type migration struct {
	version int
	sql     string
}

// migrate applies every pending migration in ascending version order. Each
// migration runs in its own transaction together with the bookkeeping insert,
// so a mid-way failure leaves that migration unapplied and Open returns an error.
func migrate(conn *sql.DB) error {
	// The bookkeeping table is created outside the numbered files: fresh and
	// legacy databases alike get it before any version is recorded.
	if _, err := conn.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY,
    applied_at TEXT NOT NULL
)`); err != nil {
		return fmt.Errorf("migrate: create schema_migrations: %w", err)
	}

	applied, err := appliedVersions(conn)
	if err != nil {
		return err
	}
	pending, err := embeddedMigrations(applied)
	if err != nil {
		return err
	}

	for _, m := range pending {
		tx, err := conn.Begin()
		if err != nil {
			return fmt.Errorf("migrate %03d: begin: %w", m.version, err)
		}
		// modernc.org/sqlite executes multi-statement SQL in a single Exec,
		// so a whole migration file runs as one batch.
		if _, err := tx.Exec(m.sql); err != nil {
			tx.Rollback()
			return fmt.Errorf("migrate %03d: %w", m.version, err)
		}
		if _, err := tx.Exec(
			"INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)",
			m.version, time.Now().UTC().Format(time.RFC3339),
		); err != nil {
			tx.Rollback()
			return fmt.Errorf("migrate %03d: record version: %w", m.version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("migrate %03d: commit: %w", m.version, err)
		}
	}
	return nil
}

func appliedVersions(conn *sql.DB) (map[int]bool, error) {
	rows, err := conn.Query("SELECT version FROM schema_migrations")
	if err != nil {
		return nil, fmt.Errorf("migrate: read applied versions: %w", err)
	}
	defer rows.Close()

	applied := map[int]bool{}
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("migrate: scan version: %w", err)
		}
		applied[version] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("migrate: read applied versions: %w", err)
	}
	return applied, nil
}

// embeddedMigrations returns the migrations not present in applied, in
// ascending version order. The version is the numeric prefix of the filename
// (001_init.sql -> 1).
func embeddedMigrations(applied map[int]bool) ([]migration, error) {
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		return nil, fmt.Errorf("migrate: read embedded migrations: %w", err)
	}

	var pending []migration
	for _, entry := range entries {
		name := entry.Name()
		numPart, _, _ := strings.Cut(strings.TrimSuffix(name, ".sql"), "_")
		version, err := strconv.Atoi(numPart)
		if err != nil {
			return nil, fmt.Errorf("migrate: parse version from %s: %w", name, err)
		}
		if applied[version] {
			continue
		}
		sqlText, err := fs.ReadFile(migrations.FS, name)
		if err != nil {
			return nil, fmt.Errorf("migrate: read %s: %w", name, err)
		}
		pending = append(pending, migration{version: version, sql: string(sqlText)})
	}

	sort.Slice(pending, func(i, j int) bool { return pending[i].version < pending[j].version })
	for i := 1; i < len(pending); i++ {
		if pending[i].version == pending[i-1].version {
			return nil, fmt.Errorf("migrate: duplicate version %d", pending[i].version)
		}
	}
	return pending, nil
}

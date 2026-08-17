package db

import (
	"database/sql"
	"fmt"
	"io/fs"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"html-preview/migrations"
)

type migrationRow struct {
	Version   int
	AppliedAt string
}

func appliedRows(t *testing.T, conn *sql.DB) []migrationRow {
	t.Helper()
	rows, err := conn.Query("SELECT version, applied_at FROM schema_migrations ORDER BY version")
	if err != nil {
		t.Fatalf("select schema_migrations: %v", err)
	}
	defer rows.Close()

	var got []migrationRow
	for rows.Next() {
		var r migrationRow
		if err := rows.Scan(&r.Version, &r.AppliedAt); err != nil {
			t.Fatalf("scan schema_migrations: %v", err)
		}
		if _, err := time.Parse(time.RFC3339, r.AppliedAt); err != nil {
			t.Errorf("version %d applied_at %q is not RFC3339: %v", r.Version, r.AppliedAt, err)
		}
		got = append(got, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate schema_migrations: %v", err)
	}
	return got
}

type columnInfo struct {
	Name    string
	Type    string
	NotNull bool
	Default sql.NullString
}

func itemColumns(t *testing.T, conn *sql.DB) map[string]columnInfo {
	t.Helper()
	rows, err := conn.Query("PRAGMA table_info(items)")
	if err != nil {
		t.Fatalf("pragma table_info(items): %v", err)
	}
	defer rows.Close()

	cols := map[string]columnInfo{}
	for rows.Next() {
		var cid, notNull, pk int
		var c columnInfo
		if err := rows.Scan(&cid, &c.Name, &c.Type, &notNull, &c.Default, &pk); err != nil {
			t.Fatalf("scan table_info: %v", err)
		}
		c.NotNull = notNull == 1
		cols[c.Name] = c
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate table_info: %v", err)
	}
	return cols
}

// dumpSchema renders every user object so two databases can be compared.
func dumpSchema(t *testing.T, conn *sql.DB) []string {
	t.Helper()
	rows, err := conn.Query("SELECT type, name, sql FROM sqlite_master WHERE name NOT LIKE 'sqlite_%' ORDER BY type, name")
	if err != nil {
		t.Fatalf("select sqlite_master: %v", err)
	}
	defer rows.Close()

	var dump []string
	for rows.Next() {
		var typ, name string
		var ddl sql.NullString
		if err := rows.Scan(&typ, &name, &ddl); err != nil {
			t.Fatalf("scan sqlite_master: %v", err)
		}
		dump = append(dump, fmt.Sprintf("%s|%s|%s", typ, name, ddl.String))
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate sqlite_master: %v", err)
	}
	return dump
}

// createLegacyDB builds a database in the pre-migration deployment state:
// the 001 schema exists, but schema_migrations and the favorite columns do not.
func createLegacyDB(t *testing.T, path string) {
	t.Helper()
	initSQL, err := fs.ReadFile(migrations.FS, "001_init.sql")
	if err != nil {
		t.Fatalf("read 001_init.sql: %v", err)
	}
	conn, err := sql.Open("sqlite", fmt.Sprintf("file:%s?_pragma=foreign_keys(1)", path))
	if err != nil {
		t.Fatalf("open legacy db: %v", err)
	}
	defer conn.Close()
	if _, err := conn.Exec(string(initSQL)); err != nil {
		t.Fatalf("create legacy schema: %v", err)
	}

	var n int
	if err := conn.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'schema_migrations'").Scan(&n); err != nil {
		t.Fatalf("check legacy state: %v", err)
	}
	if n != 0 {
		t.Fatal("legacy db unexpectedly has schema_migrations")
	}
	if _, ok := itemColumns(t, conn)["favorite"]; ok {
		t.Fatal("legacy db unexpectedly has favorite column")
	}
}

func assertFavoriteColumns(t *testing.T, conn *sql.DB) {
	t.Helper()
	cols := itemColumns(t, conn)

	fav, ok := cols["favorite"]
	if !ok {
		t.Fatalf("items is missing favorite column, got %v", cols)
	}
	if fav.Type != "INTEGER" || !fav.NotNull || fav.Default.String != "0" {
		t.Errorf("favorite column = %+v, want INTEGER NOT NULL DEFAULT 0", fav)
	}
	favAt, ok := cols["favorited_at"]
	if !ok {
		t.Fatalf("items is missing favorited_at column, got %v", cols)
	}
	if favAt.Type != "TEXT" || favAt.NotNull || favAt.Default.Valid {
		t.Errorf("favorited_at column = %+v, want nullable TEXT with no default", favAt)
	}
	if len(cols) != 15 {
		t.Errorf("items has %d columns, want 15", len(cols))
	}
}

func TestOpen_FreshDBAppliesAllMigrations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	conn, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer conn.Close()

	rows := appliedRows(t, conn)
	if len(rows) != 2 {
		t.Fatalf("schema_migrations has %d rows, want 2", len(rows))
	}
	for i, want := range []int{1, 2} {
		if rows[i].Version != want {
			t.Errorf("row %d version = %d, want %d", i, rows[i].Version, want)
		}
	}
	assertFavoriteColumns(t, conn)
}

func TestOpen_SecondOpenIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	conn, err := Open(path)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	firstCols := itemColumns(t, conn)
	firstDump := dumpSchema(t, conn)
	if err := conn.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	conn, err = Open(path)
	if err != nil {
		t.Fatalf("second open: %v", err)
	}
	defer conn.Close()

	if rows := appliedRows(t, conn); len(rows) != 2 {
		t.Fatalf("after second open schema_migrations has %d rows, want 2", len(rows))
	}
	if got := itemColumns(t, conn); !reflect.DeepEqual(got, firstCols) {
		t.Errorf("columns changed after second open: %+v", got)
	}
	if got := dumpSchema(t, conn); !reflect.DeepEqual(got, firstDump) {
		t.Errorf("schema changed after second open: %v", got)
	}
}

func TestOpen_FailingMigrationAborts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "drift.db")
	createLegacyDB(t, path)

	// Simulate schema drift: the favorite column already exists, so 002 fails.
	drift, err := sql.Open("sqlite", fmt.Sprintf("file:%s?_pragma=foreign_keys(1)", path))
	if err != nil {
		t.Fatalf("open drift db: %v", err)
	}
	if _, err := drift.Exec("ALTER TABLE items ADD COLUMN favorite INTEGER NOT NULL DEFAULT 0"); err != nil {
		t.Fatalf("seed drift: %v", err)
	}
	if err := drift.Close(); err != nil {
		t.Fatalf("close drift db: %v", err)
	}

	conn, err := Open(path)
	if err == nil {
		conn.Close()
		t.Fatal("open unexpectedly succeeded with a failing migration")
	}
	if !strings.Contains(err.Error(), "migrate 002") {
		t.Errorf("error %q does not mention the failing migration", err)
	}

	// The failed migration must leave no version row behind.
	conn, err = sql.Open("sqlite", fmt.Sprintf("file:%s?_pragma=foreign_keys(1)", path))
	if err != nil {
		t.Fatalf("reopen drift db: %v", err)
	}
	defer conn.Close()
	if rows := appliedRows(t, conn); len(rows) != 1 || rows[0].Version != 1 {
		t.Errorf("after failed migration got versions %+v, want only [1]", rows)
	}
}

func TestOpen_UpgradesLegacyDB(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	createLegacyDB(t, path)

	conn, err := Open(path)
	if err != nil {
		t.Fatalf("open legacy db: %v", err)
	}
	defer conn.Close()

	rows := appliedRows(t, conn)
	if len(rows) != 2 {
		t.Fatalf("schema_migrations has %d rows, want 2 (001 must be recorded as a no-op)", len(rows))
	}
	assertFavoriteColumns(t, conn)

	// The upgraded legacy database must match one built fresh from 001+002.
	fresh, err := Open(filepath.Join(t.TempDir(), "fresh.db"))
	if err != nil {
		t.Fatalf("open fresh db: %v", err)
	}
	defer fresh.Close()
	if got, want := dumpSchema(t, conn), dumpSchema(t, fresh); !reflect.DeepEqual(got, want) {
		t.Errorf("upgraded schema differs from fresh:\nupgraded:\n%v\nfresh:\n%v", got, want)
	}
}

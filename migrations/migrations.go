// Package migrations embeds the versioned database migration files.
// Files are named NNN_description.sql; the numeric prefix is the version.
package migrations

import "embed"

// FS holds every migration SQL file in this directory.
//
//go:embed *.sql
var FS embed.FS

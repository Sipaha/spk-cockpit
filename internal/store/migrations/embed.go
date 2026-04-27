// Package migrations holds embedded SQLite migration files.
package migrations

import "embed"

// FS exposes the embedded migration .sql files (named NNNN_description.sql).
//
//go:embed *.sql
var FS embed.FS

// Package store provides a SQLite-backed Store and repository implementations
// for spk-cockpit's domain repos defined in internal/todo.
package store

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite" // pure-Go SQLite driver
)

// Store is a thin wrapper over a configured *sql.DB.
type Store struct {
	DB *sql.DB
}

// Open opens the SQLite database at dsn (e.g. "file:/path/to.db") and applies
// the standard pragmas (foreign_keys ON, WAL, NORMAL sync, busy_timeout 5s).
func Open(dsn string) (*Store, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("sql open: %w", err)
	}
	db.SetMaxOpenConns(1) // serialize writes via single conn for simplicity in v1
	pragmas := []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA journal_mode = WAL",
		"PRAGMA synchronous = NORMAL",
		"PRAGMA busy_timeout = 5000",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("pragma %q: %w", p, err)
		}
	}
	return &Store{DB: db}, nil
}

// Close closes the underlying database.
func (s *Store) Close() error {
	return s.DB.Close()
}

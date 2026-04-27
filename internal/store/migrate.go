package store

import (
	"database/sql"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spk/spk-cockpit/internal/store/migrations"
)

// Migrate applies all embedded migrations not yet recorded in schema_migrations.
// Each migration runs inside its own transaction; failure rolls back.
func Migrate(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at INTEGER NOT NULL
	)`); err != nil {
		return fmt.Errorf("ensure migrations table: %w", err)
	}

	entries, err := migrations.FS.ReadDir(".")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}

	type m struct {
		version int
		name    string
	}
	var ms []m
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		parts := strings.SplitN(e.Name(), "_", 2)
		if len(parts) != 2 {
			return fmt.Errorf("bad migration filename %q", e.Name())
		}
		v, err := strconv.Atoi(parts[0])
		if err != nil {
			return fmt.Errorf("parse migration version %q: %w", parts[0], err)
		}
		ms = append(ms, m{version: v, name: e.Name()})
	}
	sort.Slice(ms, func(i, j int) bool { return ms[i].version < ms[j].version })

	for _, mig := range ms {
		var exists int
		if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, mig.version).Scan(&exists); err != nil {
			return fmt.Errorf("check migration %d: %w", mig.version, err)
		}
		if exists > 0 {
			continue
		}
		body, err := migrations.FS.ReadFile(mig.name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", mig.name, err)
		}
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("begin migration tx: %w", err)
		}
		if _, err := tx.Exec(string(body)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("exec migration %s: %w", mig.name, err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations(version, applied_at) VALUES (?, ?)`, mig.version, time.Now().Unix()); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %s: %w", mig.name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", mig.name, err)
		}
	}
	return nil
}

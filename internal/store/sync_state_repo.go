package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/spk/spk-cockpit/internal/api"
)

// SyncStateRepo is the SQLite-backed implementation of meeting.SyncStateRepo. //nolint:revive // domain naming intentional
type SyncStateRepo struct {
	db *sql.DB
}

// NewSyncStateRepo constructs a SyncStateRepo over db.
func NewSyncStateRepo(db *sql.DB) *SyncStateRepo { return &SyncStateRepo{db: db} }

// Get returns the sync state for source. Absent → empty entry, no error.
func (r *SyncStateRepo) Get(ctx context.Context, source string) (api.SyncStateEntry, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT source, cursor, last_ok_at, last_err FROM sync_state WHERE source = ?`, source)
	var entry api.SyncStateEntry
	var lastOk sql.NullInt64
	if err := row.Scan(&entry.Source, &entry.Cursor, &lastOk, &entry.LastErr); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return api.SyncStateEntry{Source: source}, nil
		}
		return api.SyncStateEntry{}, fmt.Errorf("get sync_state: %w", err)
	}
	if lastOk.Valid {
		v := lastOk.Int64
		entry.LastOkAt = &v
	}
	return entry, nil
}

// Save upserts a sync state entry.
func (r *SyncStateRepo) Save(ctx context.Context, e api.SyncStateEntry) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO sync_state(source, cursor, last_ok_at, last_err) VALUES (?, ?, ?, ?)
		ON CONFLICT(source) DO UPDATE SET cursor=excluded.cursor, last_ok_at=excluded.last_ok_at, last_err=excluded.last_err
	`, e.Source, e.Cursor, e.LastOkAt, e.LastErr)
	if err != nil {
		return fmt.Errorf("save sync_state: %w", err)
	}
	return nil
}

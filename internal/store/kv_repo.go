package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// KvRepo is the SQLite-backed implementation of todo.KvRepo.
//
//nolint:revive // KvRepo intentionally includes package qualifier for cross-package readability
type KvRepo struct {
	db *sql.DB
}

// NewKvRepo constructs a KvRepo over db.
func NewKvRepo(db *sql.DB) *KvRepo { return &KvRepo{db: db} }

// Get returns (value, found, err). When the key is absent: ("", false, nil).
func (r *KvRepo) Get(ctx context.Context, key string) (string, bool, error) {
	var v string
	err := r.db.QueryRowContext(ctx, `SELECT v FROM kv WHERE k = ?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("kv get: %w", err)
	}
	return v, true, nil
}

// Set upserts the key.
func (r *KvRepo) Set(ctx context.Context, key, value string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO kv(k, v) VALUES (?, ?)
		ON CONFLICT(k) DO UPDATE SET v=excluded.v
	`, key, value)
	if err != nil {
		return fmt.Errorf("kv set: %w", err)
	}
	return nil
}

// Delete removes a key.
func (r *KvRepo) Delete(ctx context.Context, key string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM kv WHERE k = ?`, key)
	if err != nil {
		return fmt.Errorf("kv delete: %w", err)
	}
	return nil
}

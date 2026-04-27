package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/spk/spk-cockpit/internal/secret"
)

// SecretRepo is the SQLite-backed implementation of secret.SecretRepo. //nolint:revive // domain naming intentional
type SecretRepo struct {
	db *sql.DB
}

// NewSecretRepo constructs a SecretRepo over db.
func NewSecretRepo(db *sql.DB) *SecretRepo { return &SecretRepo{db: db} }

// Get returns the encrypted secret by name.
func (r *SecretRepo) Get(ctx context.Context, name string) (secret.EncryptedSecret, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT name, ciphertext, nonce, updated_at FROM secrets WHERE name = ?`, name)
	var s secret.EncryptedSecret
	if err := row.Scan(&s.Name, &s.Ciphertext, &s.Nonce, &s.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return secret.EncryptedSecret{}, secret.ErrNotFound
		}
		return secret.EncryptedSecret{}, fmt.Errorf("get secret: %w", err)
	}
	return s, nil
}

// Set inserts or updates a secret.
func (r *SecretRepo) Set(ctx context.Context, s secret.EncryptedSecret) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO secrets(name, ciphertext, nonce, updated_at) VALUES (?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET ciphertext=excluded.ciphertext, nonce=excluded.nonce, updated_at=excluded.updated_at
	`, s.Name, s.Ciphertext, s.Nonce, s.UpdatedAt)
	if err != nil {
		return fmt.Errorf("set secret: %w", err)
	}
	return nil
}

// Delete removes a secret.
func (r *SecretRepo) Delete(ctx context.Context, name string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM secrets WHERE name = ?`, name)
	if err != nil {
		return fmt.Errorf("delete secret: %w", err)
	}
	return nil
}

// ListNames returns all known secret names (no values).
func (r *SecretRepo) ListNames(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT name FROM secrets ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list secrets: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

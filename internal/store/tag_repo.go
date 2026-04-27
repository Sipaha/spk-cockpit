package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/spk/spk-cockpit/internal/api"
)

// TagRepo is the SQLite-backed implementation of todo.TagRepo.
//
//nolint:revive // TagRepo intentionally includes package qualifier for cross-package readability
type TagRepo struct {
	db *sql.DB
}

// NewTagRepo constructs a TagRepo over db.
func NewTagRepo(db *sql.DB) *TagRepo { return &TagRepo{db: db} }

// Upsert inserts or updates a tag (color is updated; created_at preserved on conflict).
func (r *TagRepo) Upsert(ctx context.Context, t api.Tag) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO tags(name, color, created_at) VALUES (?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET color=excluded.color
	`, t.Name, t.Color, t.CreatedAt)
	if err != nil {
		return fmt.Errorf("upsert tag: %w", err)
	}
	return nil
}

// List returns all tags ordered by name.
func (r *TagRepo) List(ctx context.Context) ([]api.Tag, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT name, color, created_at FROM tags ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []api.Tag
	for rows.Next() {
		var t api.Tag
		if err := rows.Scan(&t.Name, &t.Color, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// Delete removes a tag and (via FK cascade) all its links.
func (r *TagRepo) Delete(ctx context.Context, name string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM tags WHERE name = ?`, name)
	if err != nil {
		return fmt.Errorf("delete tag: %w", err)
	}
	return nil
}

// SetTodoTags replaces the full set of tags on a todo, auto-creating any missing tag rows.
func (r *TagRepo) SetTodoTags(ctx context.Context, todoID string, tags []string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM todo_tags WHERE todo_id = ?`, todoID); err != nil {
		return fmt.Errorf("clear todo_tags: %w", err)
	}
	for _, name := range tags {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO tags(name, color, created_at) VALUES (?, '', strftime('%s','now'))
			 ON CONFLICT(name) DO NOTHING`, name); err != nil {
			return fmt.Errorf("ensure tag %s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO todo_tags(todo_id, tag) VALUES (?, ?)`, todoID, name); err != nil {
			return fmt.Errorf("link todo_tag: %w", err)
		}
	}
	return tx.Commit()
}

// GetTodoTags returns the tag names attached to a todo, ordered alphabetically.
func (r *TagRepo) GetTodoTags(ctx context.Context, todoID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT tag FROM todo_tags WHERE todo_id = ? ORDER BY tag`, todoID)
	if err != nil {
		return nil, fmt.Errorf("get todo_tags: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

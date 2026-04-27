package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/spk/spk-cockpit/internal/api"
)

// EventRepo is the SQLite-backed implementation of todo.EventRepo.
//
//nolint:revive // EventRepo intentionally includes package qualifier for cross-package readability
type EventRepo struct {
	db *sql.DB
}

// NewEventRepo constructs an EventRepo over db.
func NewEventRepo(db *sql.DB) *EventRepo { return &EventRepo{db: db} }

// Append inserts an audit event row; the ID field on input is ignored (autoincrement).
func (r *EventRepo) Append(ctx context.Context, e api.TodoEvent) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO todo_events(todo_id, kind, from_value, to_value, payload, at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, e.TodoID, e.Kind, nullStr(e.FromValue), nullStr(e.ToValue), nullStr(e.Payload), e.At)
	if err != nil {
		return fmt.Errorf("insert todo_event: %w", err)
	}
	return nil
}

// ListByTodo returns events for a todo, newest first; limit<=0 = no limit.
func (r *EventRepo) ListByTodo(ctx context.Context, todoID string, limit int) ([]api.TodoEvent, error) {
	q := `SELECT id, todo_id, kind, COALESCE(from_value,''), COALESCE(to_value,''), COALESCE(payload,''), at
		FROM todo_events WHERE todo_id = ? ORDER BY at DESC, id DESC`
	if limit > 0 {
		//nolint:gosec // limit is a safe integer, not user input
		q += fmt.Sprintf(" LIMIT %d", limit)
	}
	rows, err := r.db.QueryContext(ctx, q, todoID)
	return scanEvents(rows, err)
}

// ListAll returns all events with at >= sinceUnix, newest first; limit<=0 = no limit.
func (r *EventRepo) ListAll(ctx context.Context, sinceUnix int64, limit int) ([]api.TodoEvent, error) {
	q := `SELECT id, todo_id, kind, COALESCE(from_value,''), COALESCE(to_value,''), COALESCE(payload,''), at
		FROM todo_events WHERE at >= ? ORDER BY at DESC, id DESC`
	if limit > 0 {
		//nolint:gosec // limit is a safe integer, not user input
		q += fmt.Sprintf(" LIMIT %d", limit)
	}
	rows, err := r.db.QueryContext(ctx, q, sinceUnix)
	return scanEvents(rows, err)
}

func scanEvents(rows *sql.Rows, err error) ([]api.TodoEvent, error) {
	if err != nil {
		return nil, fmt.Errorf("query events: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []api.TodoEvent
	for rows.Next() {
		var e api.TodoEvent
		if err := rows.Scan(&e.ID, &e.TodoID, &e.Kind, &e.FromValue, &e.ToValue, &e.Payload, &e.At); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

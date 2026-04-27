package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/timer"
)

// TimerRepo is the SQLite-backed implementation of timer.TimerRepo. //nolint:revive // domain naming intentional
type TimerRepo struct {
	db *sql.DB
}

// NewTimerRepo constructs a TimerRepo over db.
func NewTimerRepo(db *sql.DB) *TimerRepo { return &TimerRepo{db: db} }

// Start inserts an active session.
func (r *TimerRepo) Start(ctx context.Context, todoID string, startedAt int64, source string) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO timer_sessions(todo_id, started_at, source) VALUES (?, ?, ?)`,
		todoID, startedAt, source,
	)
	if err != nil {
		// Detect partial unique index violation: UNIQUE constraint on (todo_id) WHERE ended_at IS NULL.
		// The error message varies but typically contains "UNIQUE" for constraint violations.
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return 0, timer.ErrAlreadyActiveOnTodo
		}
		return 0, fmt.Errorf("insert timer_session: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("last insert id: %w", err)
	}
	return id, nil
}

// Stop sets ended_at on the single active row for todoID.
func (r *TimerRepo) Stop(ctx context.Context, todoID string, endedAt int64) (api.TimerSession, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return api.TimerSession{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	row := tx.QueryRowContext(ctx,
		`SELECT id, todo_id, started_at, ended_at, source
		 FROM timer_sessions WHERE todo_id = ? AND ended_at IS NULL`, todoID)
	s, err := scanSession(row)
	if errors.Is(err, sql.ErrNoRows) {
		return api.TimerSession{}, timer.ErrNoActiveSession
	}
	if err != nil {
		return api.TimerSession{}, err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE timer_sessions SET ended_at = ? WHERE id = ?`, endedAt, s.ID); err != nil {
		return api.TimerSession{}, fmt.Errorf("update timer_session: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return api.TimerSession{}, fmt.Errorf("commit tx: %w", err)
	}
	s.EndedAt = &endedAt
	return s, nil
}

// Active returns the current active session, or (nil, nil) if none.
func (r *TimerRepo) Active(ctx context.Context) (*api.TimerSession, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, todo_id, started_at, ended_at, source
		 FROM timer_sessions WHERE ended_at IS NULL LIMIT 1`)
	s, err := scanSession(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// ListByTodo returns sessions newest first.
func (r *TimerRepo) ListByTodo(ctx context.Context, todoID string, limit int) ([]api.TimerSession, error) {
	q := `SELECT id, todo_id, started_at, ended_at, source
		FROM timer_sessions WHERE todo_id = ? ORDER BY started_at DESC, id DESC`
	if limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", limit) //nolint:gosec // limit is a controlled int
	}
	rows, err := r.db.QueryContext(ctx, q, todoID)
	if err != nil {
		return nil, fmt.Errorf("query timer_sessions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []api.TimerSession
	for rows.Next() {
		s, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// TotalForTodo aggregates completed sessions only (active session excluded).
func (r *TimerRepo) TotalForTodo(ctx context.Context, todoID string, sinceUnix int64) (int64, int, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(ended_at - started_at), 0), COUNT(*)
		 FROM timer_sessions
		 WHERE todo_id = ? AND ended_at IS NOT NULL AND started_at >= ?`,
		todoID, sinceUnix,
	)
	var total int64
	var count int
	if err := row.Scan(&total, &count); err != nil {
		return 0, 0, fmt.Errorf("aggregate sessions: %w", err)
	}
	return total, count, nil
}

type sessionScanner interface {
	Scan(...any) error
}

func scanSession(s sessionScanner) (api.TimerSession, error) {
	var x api.TimerSession
	var ended sql.NullInt64
	if err := s.Scan(&x.ID, &x.TodoID, &x.StartedAt, &ended, &x.Source); err != nil {
		return api.TimerSession{}, err
	}
	if ended.Valid {
		v := ended.Int64
		x.EndedAt = &v
	}
	return x, nil
}

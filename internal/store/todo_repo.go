package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/todo"
)

// TodoRepo is the SQLite-backed implementation of todo.TodoRepo.
//
//nolint:revive // TodoRepo intentionally includes package qualifier for cross-package readability
type TodoRepo struct {
	db *sql.DB
}

// NewTodoRepo constructs a TodoRepo over db.
func NewTodoRepo(db *sql.DB) *TodoRepo { return &TodoRepo{db: db} }

// Create inserts a todo. Caller is responsible for ID and timestamps.
func (r *TodoRepo) Create(ctx context.Context, t api.Todo) error {
	if t.SortOrder == 0 {
		// Default: created_at as sort key so new todos land at the top of
		// their column without the caller having to think about ordering.
		t.SortOrder = float64(t.CreatedAt)
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO todos(id, title, notes, priority, status, due_at, created_at, updated_at, done_at, sort_order, dismissed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, t.ID, t.Title, t.Notes, int(t.Priority), string(t.Status), t.DueAt, t.CreatedAt, t.UpdatedAt, t.DoneAt, t.SortOrder, t.DismissedAt)
	if err != nil {
		return fmt.Errorf("insert todo: %w", err)
	}
	return nil
}

// Get returns a non-deleted todo by id, or todo.ErrNotFound.
func (r *TodoRepo) Get(ctx context.Context, id string) (api.Todo, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, title, notes, priority, status, due_at, created_at, updated_at, done_at, sort_order, dismissed_at
		FROM todos WHERE id = ? AND deleted_at IS NULL
	`, id)
	t, err := scanTodo(row)
	if errors.Is(err, sql.ErrNoRows) {
		return api.Todo{}, todo.ErrNotFound
	}
	return t, err
}

// Update loads, mutates and saves a todo atomically.
func (r *TodoRepo) Update(ctx context.Context, id string, mutate func(*api.Todo) error) (api.Todo, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return api.Todo{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	row := tx.QueryRowContext(ctx, `
		SELECT id, title, notes, priority, status, due_at, created_at, updated_at, done_at, sort_order, dismissed_at
		FROM todos WHERE id = ? AND deleted_at IS NULL
	`, id)
	t, err := scanTodo(row)
	if errors.Is(err, sql.ErrNoRows) {
		return api.Todo{}, todo.ErrNotFound
	}
	if err != nil {
		return api.Todo{}, err
	}

	if err := mutate(&t); err != nil {
		return api.Todo{}, err
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE todos SET title=?, notes=?, priority=?, status=?, due_at=?, updated_at=?, done_at=?, sort_order=?, dismissed_at=?
		WHERE id=?
	`, t.Title, t.Notes, int(t.Priority), string(t.Status), t.DueAt, t.UpdatedAt, t.DoneAt, t.SortOrder, t.DismissedAt, t.ID)
	if err != nil {
		return api.Todo{}, fmt.Errorf("update todo: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return api.Todo{}, fmt.Errorf("commit tx: %w", err)
	}
	return t, nil
}

// Delete soft-deletes a todo by setting deleted_at.
func (r *TodoRepo) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `UPDATE todos SET deleted_at=strftime('%s','now') WHERE id=? AND deleted_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("delete todo: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return todo.ErrNotFound
	}
	return nil
}

// MoveAndReorder updates the primary todo via mutate (same contract as
// Update) and rewrites sort_order on the supplied siblings, all in a single
// transaction. Returns the post-update primary plus a slice of the siblings
// that were actually rewritten, read fresh inside the same tx — callers
// should publish events from this slice rather than re-fetching, so a
// concurrent soft-delete can't make a sibling vanish between commit and read.
func (r *TodoRepo) MoveAndReorder(ctx context.Context, primaryID string, mutate func(*api.Todo) error, siblings []todo.SortOrderUpdate) (api.Todo, []api.Todo, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return api.Todo{}, nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	row := tx.QueryRowContext(ctx, `
		SELECT id, title, notes, priority, status, due_at, created_at, updated_at, done_at, sort_order, dismissed_at
		FROM todos WHERE id = ? AND deleted_at IS NULL
	`, primaryID)
	t, err := scanTodo(row)
	if errors.Is(err, sql.ErrNoRows) {
		return api.Todo{}, nil, todo.ErrNotFound
	}
	if err != nil {
		return api.Todo{}, nil, err
	}
	if err := mutate(&t); err != nil {
		return api.Todo{}, nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE todos SET title=?, notes=?, priority=?, status=?, due_at=?, updated_at=?, done_at=?, sort_order=?, dismissed_at=?
		WHERE id=?
	`, t.Title, t.Notes, int(t.Priority), string(t.Status), t.DueAt, t.UpdatedAt, t.DoneAt, t.SortOrder, t.DismissedAt, t.ID); err != nil {
		return api.Todo{}, nil, fmt.Errorf("update primary: %w", err)
	}
	updatedSiblings := make([]api.Todo, 0, len(siblings))
	for _, s := range siblings {
		if s.ID == primaryID {
			continue
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE todos SET sort_order=?, updated_at=? WHERE id=? AND deleted_at IS NULL`,
			s.SortOrder, t.UpdatedAt, s.ID); err != nil {
			return api.Todo{}, nil, fmt.Errorf("update sibling %s: %w", s.ID, err)
		}
		// Read the sibling back fresh — concurrent updates from another tx
		// can't slip between this read and the commit because we hold the row
		// lock from the UPDATE above.
		row := tx.QueryRowContext(ctx, `
			SELECT id, title, notes, priority, status, due_at, created_at, updated_at, done_at, sort_order, dismissed_at
			FROM todos WHERE id = ? AND deleted_at IS NULL
		`, s.ID)
		sib, err := scanTodo(row)
		if errors.Is(err, sql.ErrNoRows) {
			// Sibling was soft-deleted concurrently — drop from the published set.
			continue
		}
		if err != nil {
			return api.Todo{}, nil, fmt.Errorf("re-read sibling %s: %w", s.ID, err)
		}
		updatedSiblings = append(updatedSiblings, sib)
	}
	if err := tx.Commit(); err != nil {
		return api.Todo{}, nil, fmt.Errorf("commit tx: %w", err)
	}
	return t, updatedSiblings, nil
}

// Restore clears deleted_at on a previously soft-deleted todo, returning the
// freshly-undeleted row. Used by the Revert toast and the Trash page.
func (r *TodoRepo) Restore(ctx context.Context, id string) (api.Todo, error) {
	res, err := r.db.ExecContext(ctx, `UPDATE todos SET deleted_at=NULL WHERE id=? AND deleted_at IS NOT NULL`, id)
	if err != nil {
		return api.Todo{}, fmt.Errorf("restore todo: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return api.Todo{}, todo.ErrNotFound
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT id, title, notes, priority, status, due_at, created_at, updated_at, done_at, sort_order, dismissed_at
		FROM todos WHERE id = ?
	`, id)
	return scanTodo(row)
}

// ListDeleted returns soft-deleted todos newest-deleted first, capped to limit
// (defaults to 100 when limit <= 0). Tags aren't joined here — the trash UI
// only renders title + when-deleted, so we save the per-row query.
func (r *TodoRepo) ListDeleted(ctx context.Context, limit int) ([]api.Todo, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT id, title, notes, priority, status, due_at, created_at, updated_at, done_at, sort_order, dismissed_at
		FROM todos WHERE deleted_at IS NOT NULL
		ORDER BY deleted_at DESC LIMIT %d
	`, limit))
	if err != nil {
		return nil, fmt.Errorf("list deleted: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []api.Todo
	for rows.Next() {
		t, err := scanTodo(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// List returns todos matching the filter, sorted by status (open first), then priority desc, then due_at asc, then created_at desc.
func (r *TodoRepo) List(ctx context.Context, f todo.TodoFilter) ([]api.Todo, error) {
	var (
		conds []string
		args  []any
	)
	conds = append(conds, "deleted_at IS NULL")
	if !f.IncludeDone {
		conds = append(conds, "status NOT IN ('done', 'cancelled')")
	}
	if len(f.Statuses) > 0 {
		ph := strings.Repeat("?,", len(f.Statuses))
		ph = ph[:len(ph)-1]
		conds = append(conds, "status IN ("+ph+")")
		for _, s := range f.Statuses {
			args = append(args, string(s))
		}
	}
	if len(f.Priorities) > 0 {
		ph := strings.Repeat("?,", len(f.Priorities))
		ph = ph[:len(ph)-1]
		conds = append(conds, "priority IN ("+ph+")")
		for _, p := range f.Priorities {
			args = append(args, int(p))
		}
	}
	if f.Search != "" {
		conds = append(conds, "(title LIKE ? OR notes LIKE ?)")
		s := "%" + f.Search + "%"
		args = append(args, s, s)
	}
	if len(f.Tags) > 0 {
		ph := strings.Repeat("?,", len(f.Tags))
		ph = ph[:len(ph)-1]
		conds = append(conds, "id IN (SELECT todo_id FROM todo_tags WHERE tag IN ("+ph+"))")
		for _, t := range f.Tags {
			args = append(args, t)
		}
	}
	//nolint:gosec // all conds are built from safe sources, not user input
	q := `SELECT id, title, notes, priority, status, due_at, created_at, updated_at, done_at, sort_order, dismissed_at
		FROM todos WHERE ` + strings.Join(conds, " AND ") +
		` ORDER BY status='done' ASC, sort_order DESC, created_at DESC`
	if f.Limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", f.Limit)
	}
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list todos: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []api.Todo
	for rows.Next() {
		t, err := scanTodo(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

type scanner interface {
	Scan(...any) error
}

func scanTodo(s scanner) (api.Todo, error) {
	var t api.Todo
	var prio int
	var status string
	var dueAt, doneAt, dismissedAt sql.NullInt64
	if err := s.Scan(&t.ID, &t.Title, &t.Notes, &prio, &status, &dueAt, &t.CreatedAt, &t.UpdatedAt, &doneAt, &t.SortOrder, &dismissedAt); err != nil {
		return api.Todo{}, err
	}
	t.Priority = api.Priority(prio)
	t.Status = api.TodoStatus(status)
	if dueAt.Valid {
		v := dueAt.Int64
		t.DueAt = &v
	}
	if doneAt.Valid {
		v := doneAt.Int64
		t.DoneAt = &v
	}
	if dismissedAt.Valid {
		v := dismissedAt.Int64
		t.DismissedAt = &v
	}
	return t, nil
}

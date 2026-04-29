package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/note"
)

// NoteRepo is the SQLite-backed implementation of note.NoteRepo. //nolint:revive // domain naming intentional
type NoteRepo struct {
	db *sql.DB
}

// NewNoteRepo constructs a NoteRepo over db.
func NewNoteRepo(db *sql.DB) *NoteRepo { return &NoteRepo{db: db} }

// Upsert inserts or replaces a note by id.
func (r *NoteRepo) Upsert(ctx context.Context, n api.Note) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO notes(id, meeting_id, todo_id, body, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
		  meeting_id = excluded.meeting_id,
		  todo_id = excluded.todo_id,
		  body = excluded.body,
		  updated_at = excluded.updated_at
	`, n.ID, nullStr(n.MeetingID), nullStr(n.TodoID), n.Body, n.CreatedAt, n.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upsert note: %w", err)
	}
	return nil
}

// Get returns a non-deleted note by id.
func (r *NoteRepo) Get(ctx context.Context, id string) (api.Note, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, COALESCE(meeting_id,''), COALESCE(todo_id,''), body, created_at, updated_at
		FROM notes WHERE id = ? AND deleted_at IS NULL`, id)
	n, err := scanNote(row)
	if errors.Is(err, sql.ErrNoRows) {
		return api.Note{}, note.ErrNotFound
	}
	return n, err
}

// Delete soft-deletes the note.
func (r *NoteRepo) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `UPDATE notes SET deleted_at=strftime('%s','now') WHERE id=? AND deleted_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("delete note: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return note.ErrNotFound
	}
	return nil
}

// FindByAttachment returns the (single) note attached to the given meeting OR todo.
func (r *NoteRepo) FindByAttachment(ctx context.Context, meetingID, todoID string) (api.Note, error) {
	if meetingID == "" && todoID == "" {
		return api.Note{}, errors.New("meetingID or todoID required")
	}
	var row *sql.Row
	if meetingID != "" {
		row = r.db.QueryRowContext(ctx, `
			SELECT id, COALESCE(meeting_id,''), COALESCE(todo_id,''), body, created_at, updated_at
			FROM notes WHERE meeting_id = ? AND deleted_at IS NULL ORDER BY updated_at DESC LIMIT 1`,
			meetingID)
	} else {
		row = r.db.QueryRowContext(ctx, `
			SELECT id, COALESCE(meeting_id,''), COALESCE(todo_id,''), body, created_at, updated_at
			FROM notes WHERE todo_id = ? AND deleted_at IS NULL ORDER BY updated_at DESC LIMIT 1`,
			todoID)
	}
	n, err := scanNote(row)
	if errors.Is(err, sql.ErrNoRows) {
		return api.Note{}, note.ErrNotFound
	}
	return n, err
}

func scanNote(s sessionScanner) (api.Note, error) {
	var n api.Note
	if err := s.Scan(&n.ID, &n.MeetingID, &n.TodoID, &n.Body, &n.CreatedAt, &n.UpdatedAt); err != nil {
		return api.Note{}, err
	}
	return n, nil
}

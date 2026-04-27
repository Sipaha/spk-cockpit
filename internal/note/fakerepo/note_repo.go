// Package fakerepo provides an in-memory note.NoteRepo for tests.
package fakerepo

import (
	"context"
	"sort"
	"sync"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/note"
)

// Note is an in-memory note.NoteRepo.
type Note struct {
	mu      sync.Mutex
	byID    map[string]api.Note
	deleted map[string]bool
}

// NewNote constructs an empty in-memory note repo.
func NewNote() *Note { return &Note{byID: map[string]api.Note{}, deleted: map[string]bool{}} }

// Upsert inserts or replaces by id.
func (r *Note) Upsert(_ context.Context, n api.Note) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byID[n.ID] = n
	delete(r.deleted, n.ID)
	return nil
}

// Get returns a non-deleted note.
func (r *Note) Get(_ context.Context, id string) (api.Note, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.deleted[id] {
		return api.Note{}, note.ErrNotFound
	}
	n, ok := r.byID[id]
	if !ok {
		return api.Note{}, note.ErrNotFound
	}
	return n, nil
}

// Delete soft-deletes.
func (r *Note) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byID[id]; !ok {
		return note.ErrNotFound
	}
	if r.deleted[id] {
		return note.ErrNotFound
	}
	r.deleted[id] = true
	return nil
}

// List filters in memory.
func (r *Note) List(_ context.Context, f note.NoteFilter) ([]api.Note, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []api.Note
	for id, n := range r.byID {
		if r.deleted[id] {
			continue
		}
		if f.MeetingID != "" && n.MeetingID != f.MeetingID {
			continue
		}
		if f.TodoID != "" && n.TodoID != f.TodoID {
			continue
		}
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt > out[j].UpdatedAt })
	if f.Limit > 0 && len(out) > f.Limit {
		out = out[:f.Limit]
	}
	return out, nil
}

// FindByAttachment returns the latest note attached to (meetingID, todoID).
func (r *Note) FindByAttachment(_ context.Context, meetingID, todoID string) (api.Note, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var best *api.Note
	for id, n := range r.byID {
		if r.deleted[id] {
			continue
		}
		match := false
		if meetingID != "" && n.MeetingID == meetingID {
			match = true
		}
		if todoID != "" && n.TodoID == todoID {
			match = true
		}
		if !match {
			continue
		}
		if best == nil || n.UpdatedAt > best.UpdatedAt {
			n := n
			best = &n
		}
	}
	if best == nil {
		return api.Note{}, note.ErrNotFound
	}
	return *best, nil
}

package fakerepo

import (
	"context"
	"sort"
	"sync"

	"github.com/spk/spk-cockpit/internal/api"
)

// Event is an in-memory todo.EventRepo.
type Event struct {
	mu     sync.Mutex
	nextID int64
	rows   []api.TodoEvent
}

// NewEvent creates an empty in-memory event repo.
func NewEvent() *Event { return &Event{} }

// Append assigns an autoincrementing ID and stores the event.
func (r *Event) Append(_ context.Context, e api.TodoEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextID++
	e.ID = r.nextID
	r.rows = append(r.rows, e)
	return nil
}

// ListByTodo returns events for a todo, newest first.
func (r *Event) ListByTodo(_ context.Context, todoID string, limit int) ([]api.TodoEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []api.TodoEvent
	for _, e := range r.rows {
		if e.TodoID == todoID {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].At != out[j].At {
			return out[i].At > out[j].At
		}
		return out[i].ID > out[j].ID
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// ListAll returns events with at >= sinceUnix, newest first.
func (r *Event) ListAll(_ context.Context, sinceUnix int64, limit int) ([]api.TodoEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []api.TodoEvent
	for _, e := range r.rows {
		if e.At >= sinceUnix {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].At > out[j].At })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

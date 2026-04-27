// Package fakerepo provides an in-memory timer.TimerRepo for unit tests.
package fakerepo

import (
	"context"
	"sort"
	"sync"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/timer"
)

// Timer is an in-memory timer.TimerRepo.
type Timer struct {
	mu     sync.Mutex
	nextID int64
	rows   map[int64]api.TimerSession
}

// NewTimer creates an empty in-memory timer repo.
func NewTimer() *Timer {
	return &Timer{rows: map[int64]api.TimerSession{}}
}

// Start inserts a session.
func (r *Timer) Start(_ context.Context, todoID string, startedAt int64, source string) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, s := range r.rows {
		if s.TodoID == todoID && s.EndedAt == nil {
			return 0, timer.ErrAlreadyActiveOnTodo
		}
	}
	r.nextID++
	r.rows[r.nextID] = api.TimerSession{
		ID:        r.nextID,
		TodoID:    todoID,
		StartedAt: startedAt,
		Source:    source,
	}
	return r.nextID, nil
}

// Stop closes the active session for todoID.
func (r *Timer) Stop(_ context.Context, todoID string, endedAt int64) (api.TimerSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, s := range r.rows {
		if s.TodoID == todoID && s.EndedAt == nil {
			s.EndedAt = &endedAt
			r.rows[id] = s
			return s, nil
		}
	}
	return api.TimerSession{}, timer.ErrNoActiveSession
}

// Active returns the single active session (one expected by domain invariant).
func (r *Timer) Active(_ context.Context) (*api.TimerSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, s := range r.rows {
		if s.EndedAt == nil {
			s := s
			return &s, nil
		}
	}
	return nil, nil
}

// ListByTodo returns sessions newest first.
func (r *Timer) ListByTodo(_ context.Context, todoID string, limit int) ([]api.TimerSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []api.TimerSession
	for _, s := range r.rows {
		if s.TodoID == todoID {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].StartedAt != out[j].StartedAt {
			return out[i].StartedAt > out[j].StartedAt
		}
		return out[i].ID > out[j].ID
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// TotalForTodo aggregates completed sessions.
func (r *Timer) TotalForTodo(_ context.Context, todoID string, sinceUnix int64) (int64, int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var total int64
	var count int
	for _, s := range r.rows {
		if s.TodoID != todoID || s.EndedAt == nil || s.StartedAt < sinceUnix {
			continue
		}
		total += *s.EndedAt - s.StartedAt
		count++
	}
	return total, count, nil
}

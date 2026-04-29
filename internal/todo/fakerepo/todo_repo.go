// Package fakerepo provides in-memory implementations of todo repository
// interfaces, suitable for unit-testing the Todo domain without a database.
package fakerepo

import (
	"context"
	"sort"
	"strings"
	"sync"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/todo"
)

// Todo is an in-memory todo.TodoRepo.
type Todo struct {
	mu    sync.Mutex
	byID  map[string]api.Todo
	delAt map[string]int64
}

// NewTodo creates an empty in-memory todo repo.
func NewTodo() *Todo {
	return &Todo{byID: map[string]api.Todo{}, delAt: map[string]int64{}}
}

// Create inserts a todo.
func (r *Todo) Create(_ context.Context, t api.Todo) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byID[t.ID] = t
	return nil
}

// Get returns a non-deleted todo or todo.ErrNotFound.
func (r *Todo) Get(_ context.Context, id string) (api.Todo, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, deleted := r.delAt[id]; deleted {
		return api.Todo{}, todo.ErrNotFound
	}
	t, ok := r.byID[id]
	if !ok {
		return api.Todo{}, todo.ErrNotFound
	}
	return t, nil
}

// Update applies mutate to the existing todo and saves it.
func (r *Todo) Update(_ context.Context, id string, mutate func(*api.Todo) error) (api.Todo, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, deleted := r.delAt[id]; deleted {
		return api.Todo{}, todo.ErrNotFound
	}
	t, ok := r.byID[id]
	if !ok {
		return api.Todo{}, todo.ErrNotFound
	}
	if err := mutate(&t); err != nil {
		return api.Todo{}, err
	}
	r.byID[id] = t
	return t, nil
}

// Delete soft-deletes a todo.
func (r *Todo) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byID[id]; !ok {
		return todo.ErrNotFound
	}
	if _, already := r.delAt[id]; already {
		return todo.ErrNotFound
	}
	r.delAt[id] = 1
	return nil
}

// Restore clears the soft-delete marker for id.
func (r *Todo) Restore(_ context.Context, id string) (api.Todo, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.byID[id]
	if !ok {
		return api.Todo{}, todo.ErrNotFound
	}
	if _, deleted := r.delAt[id]; !deleted {
		return api.Todo{}, todo.ErrNotFound
	}
	delete(r.delAt, id)
	return t, nil
}

// MoveAndReorder applies mutate to the primary todo and rewrites sort_order
// on each listed sibling in a single mutex-protected pass.
func (r *Todo) MoveAndReorder(_ context.Context, primaryID string, mutate func(*api.Todo) error, siblings []todo.SortOrderUpdate) (api.Todo, []api.Todo, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.byID[primaryID]
	if !ok {
		return api.Todo{}, nil, todo.ErrNotFound
	}
	if _, deleted := r.delAt[primaryID]; deleted {
		return api.Todo{}, nil, todo.ErrNotFound
	}
	if err := mutate(&t); err != nil {
		return api.Todo{}, nil, err
	}
	r.byID[primaryID] = t
	updatedSiblings := make([]api.Todo, 0, len(siblings))
	for _, s := range siblings {
		if s.ID == primaryID {
			continue
		}
		sib, ok := r.byID[s.ID]
		if !ok {
			continue
		}
		if _, deleted := r.delAt[s.ID]; deleted {
			continue
		}
		sib.SortOrder = s.SortOrder
		r.byID[s.ID] = sib
		updatedSiblings = append(updatedSiblings, sib)
	}
	return t, updatedSiblings, nil
}

// ListDeleted returns the deleted todos newest-first (best effort, since we
// don't track deleted_at timestamps in this fake).
func (r *Todo) ListDeleted(_ context.Context, limit int) ([]api.Todo, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []api.Todo
	for id := range r.delAt {
		if t, ok := r.byID[id]; ok {
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt > out[j].UpdatedAt })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// List filters todos in memory the same way the SQLite repo does (status + priority + search; tags handled by domain).
func (r *Todo) List(_ context.Context, f todo.TodoFilter) ([]api.Todo, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []api.Todo
	for id, t := range r.byID {
		if _, deleted := r.delAt[id]; deleted {
			continue
		}
		if !f.IncludeDone && (t.Status == api.StatusDone || t.Status == api.StatusCancelled) {
			continue
		}
		if len(f.Statuses) > 0 && !contains(f.Statuses, t.Status) {
			continue
		}
		if len(f.Priorities) > 0 && !containsP(f.Priorities, t.Priority) {
			continue
		}
		if f.Search != "" {
			s := strings.ToLower(f.Search)
			if !strings.Contains(strings.ToLower(t.Title), s) && !strings.Contains(strings.ToLower(t.Notes), s) {
				continue
			}
		}
		out = append(out, t)
	}
	// Match SQLite's ORDER BY exactly: done last, then sort_order DESC,
	// then created_at DESC. Service.Move depends on this ordering for
	// neighbor math when computing rebalance positions.
	sort.SliceStable(out, func(i, j int) bool {
		di := out[i].Status == api.StatusDone
		dj := out[j].Status == api.StatusDone
		if di != dj {
			return !di // not-done comes first
		}
		if out[i].SortOrder != out[j].SortOrder {
			return out[i].SortOrder > out[j].SortOrder
		}
		return out[i].CreatedAt > out[j].CreatedAt
	})
	if f.Limit > 0 && len(out) > f.Limit {
		out = out[:f.Limit]
	}
	return out, nil
}

func contains(xs []api.TodoStatus, x api.TodoStatus) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func containsP(xs []api.Priority, x api.Priority) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

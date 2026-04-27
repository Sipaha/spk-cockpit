package fakerepo

import (
	"context"
	"sort"
	"sync"

	"github.com/spk/spk-cockpit/internal/api"
)

// Tag is an in-memory todo.TagRepo.
type Tag struct {
	mu       sync.Mutex
	tags     map[string]api.Tag
	todoTags map[string]map[string]struct{}
}

// NewTag creates an empty in-memory tag repo.
func NewTag() *Tag {
	return &Tag{tags: map[string]api.Tag{}, todoTags: map[string]map[string]struct{}{}}
}

// Upsert inserts or replaces a tag.
func (r *Tag) Upsert(_ context.Context, t api.Tag) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tags[t.Name] = t
	return nil
}

// List returns tags ordered by name.
func (r *Tag) List(_ context.Context) ([]api.Tag, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]api.Tag, 0, len(r.tags))
	for _, t := range r.tags {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Delete removes a tag and unlinks it from all todos.
func (r *Tag) Delete(_ context.Context, name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.tags, name)
	for _, m := range r.todoTags {
		delete(m, name)
	}
	return nil
}

// SetTodoTags replaces the full tag set on a todo (auto-creating tag rows).
func (r *Tag) SetTodoTags(_ context.Context, todoID string, tags []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	m := map[string]struct{}{}
	for _, name := range tags {
		if _, ok := r.tags[name]; !ok {
			r.tags[name] = api.Tag{Name: name, CreatedAt: 0}
		}
		m[name] = struct{}{}
	}
	r.todoTags[todoID] = m
	return nil
}

// GetTodoTags returns the tag names attached to a todo, alphabetically.
func (r *Tag) GetTodoTags(_ context.Context, todoID string) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m := r.todoTags[todoID]
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out, nil
}

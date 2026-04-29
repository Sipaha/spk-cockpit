// Package todo holds the Todo domain (service, repository contracts, errors).
package todo

import (
	"context"
	"errors"

	"github.com/spk/spk-cockpit/internal/api"
)

// Domain errors.
var (
	ErrNotFound = errors.New("todo: not found")
)

// TodoFilter controls TodoRepo.List.
//
//nolint:revive // TodoFilter intentionally includes package qualifier for cross-package readability
type TodoFilter struct {
	Statuses    []api.TodoStatus
	Priorities  []api.Priority
	Tags        []string
	Search      string
	IncludeDone bool
	Limit       int
}

// TodoRepo persists todos.
//
//nolint:revive // TodoRepo intentionally includes package qualifier for cross-package readability
type TodoRepo interface {
	Create(ctx context.Context, t api.Todo) error
	Get(ctx context.Context, id string) (api.Todo, error)
	Update(ctx context.Context, id string, mutate func(*api.Todo) error) (api.Todo, error)
	Delete(ctx context.Context, id string) error
	Restore(ctx context.Context, id string) (api.Todo, error)
	ListDeleted(ctx context.Context, limit int) ([]api.Todo, error)
	List(ctx context.Context, f TodoFilter) ([]api.Todo, error)
	// MoveAndReorder applies `mutate` to one primary todo (same shape as
	// Update) and bulk-rewrites sort_order on the listed siblings, all in
	// one transaction. Returns the primary plus the siblings actually
	// rewritten (read fresh inside the same tx) so callers can publish
	// events without a post-commit Get loop.
	MoveAndReorder(ctx context.Context, primaryID string, mutate func(*api.Todo) error, siblings []SortOrderUpdate) (api.Todo, []api.Todo, error)
}

// SortOrderUpdate is a small (id, sortOrder) tuple consumed by
// TodoRepo.MoveAndReorder.
type SortOrderUpdate struct {
	ID        string
	SortOrder float64
}

// TagRepo persists tags and links them to todos.
type TagRepo interface {
	Upsert(ctx context.Context, t api.Tag) error
	List(ctx context.Context) ([]api.Tag, error)
	Delete(ctx context.Context, name string) error
	Rename(ctx context.Context, oldName, newName string) error
	SetTodoTags(ctx context.Context, todoID string, tags []string) error
	GetTodoTags(ctx context.Context, todoID string) ([]string, error)
}

// EventRepo appends and queries todo audit events.
type EventRepo interface {
	Append(ctx context.Context, e api.TodoEvent) error
	ListByTodo(ctx context.Context, todoID string, limit int) ([]api.TodoEvent, error)
	ListAll(ctx context.Context, sinceUnix int64, limit int) ([]api.TodoEvent, error)
}

// KvRepo stores small string-keyed runtime settings.
type KvRepo interface {
	Get(ctx context.Context, key string) (string, bool, error)
	Set(ctx context.Context, key, value string) error
	Delete(ctx context.Context, key string) error
}

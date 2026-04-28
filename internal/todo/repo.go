// Package todo holds the Todo domain (service, repository contracts, errors).
package todo

import (
	"context"
	"errors"

	"github.com/spk/spk-cockpit/internal/api"
)

// Domain errors.
var (
	ErrNotFound      = errors.New("todo: not found")
	ErrTagNotFound   = errors.New("todo: tag not found")
	ErrInvalidStatus = errors.New("todo: invalid status transition")
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
}

// TagRepo persists tags and links them to todos.
type TagRepo interface {
	Upsert(ctx context.Context, t api.Tag) error
	List(ctx context.Context) ([]api.Tag, error)
	Delete(ctx context.Context, name string) error
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

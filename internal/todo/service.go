package todo

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/oklog/ulid/v2"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/clock"
)

// EventPublisher publishes domain events. The bus passed to Service may be nil — methods are nil-safe.
type EventPublisher interface {
	Publish(api.Event)
}

// Service is the Todo domain entry point. It owns mutations, audit log emission and event publishing.
type Service struct {
	todos  TodoRepo
	tags   TagRepo
	events EventRepo
	clock  clock.Clock
	bus    EventPublisher
}

// NewService wires the service. bus may be nil.
func NewService(t TodoRepo, g TagRepo, e EventRepo, c clock.Clock, bus EventPublisher) *Service {
	return &Service{todos: t, tags: g, events: e, clock: c, bus: bus}
}

func (s *Service) publish(t string, data any) {
	if s.bus == nil {
		return
	}
	s.bus.Publish(api.Event{Type: t, Data: data})
}

// Create builds a new todo, persists it, attaches tags, appends a "created" audit event, and publishes EventTodoCreated.
func (s *Service) Create(ctx context.Context, req api.CreateTodoRequest) (api.Todo, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return api.Todo{}, errors.New("title is required")
	}
	now := s.clock.Now().Unix()
	t := api.Todo{
		ID:        newID(),
		Title:     title,
		Notes:     req.Notes,
		Priority:  req.Priority,
		Status:    api.StatusOpen,
		DueAt:     req.DueAt,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.todos.Create(ctx, t); err != nil {
		return api.Todo{}, fmt.Errorf("create todo: %w", err)
	}
	if len(req.Tags) > 0 {
		if err := s.tags.SetTodoTags(ctx, t.ID, req.Tags); err != nil {
			return api.Todo{}, fmt.Errorf("set tags: %w", err)
		}
	}
	if err := s.events.Append(ctx, api.TodoEvent{TodoID: t.ID, Kind: "created", At: now}); err != nil {
		return api.Todo{}, fmt.Errorf("audit: %w", err)
	}
	t.Tags = req.Tags
	if t.Tags == nil {
		t.Tags = []string{}
	}
	s.publish(api.EventTodoCreated, api.TodoCreatedData{Todo: t})
	return t, nil
}

// Get returns a todo with its tags loaded.
func (s *Service) Get(ctx context.Context, id string) (api.Todo, error) {
	t, err := s.todos.Get(ctx, id)
	if err != nil {
		return api.Todo{}, err
	}
	tags, err := s.tags.GetTodoTags(ctx, id)
	if err != nil {
		return api.Todo{}, fmt.Errorf("get tags: %w", err)
	}
	t.Tags = tags
	return t, nil
}

// Update applies non-nil fields, updates timestamps, sets DoneAt for status=done,
// replaces tags if provided, and emits audit and bus events.
func (s *Service) Update(ctx context.Context, id string, req api.UpdateTodoRequest) (api.Todo, error) {
	now := s.clock.Now().Unix()
	var changed []string
	var (
		statusFrom, statusTo api.TodoStatus
		statusChanged        bool
		prioFrom, prioTo     api.Priority
		prioChanged          bool
	)

	updated, err := s.todos.Update(ctx, id, func(t *api.Todo) error {
		if req.Title != nil {
			if strings.TrimSpace(*req.Title) == "" {
				return errors.New("title is required")
			}
			if t.Title != *req.Title {
				t.Title = *req.Title
				changed = append(changed, "title")
			}
		}
		if req.Notes != nil && t.Notes != *req.Notes {
			t.Notes = *req.Notes
			changed = append(changed, "notes")
		}
		if req.Priority != nil && t.Priority != *req.Priority {
			prioFrom, prioTo = t.Priority, *req.Priority
			t.Priority = *req.Priority
			prioChanged = true
			changed = append(changed, "priority")
		}
		if req.Status != nil && t.Status != *req.Status {
			statusFrom, statusTo = t.Status, *req.Status
			t.Status = *req.Status
			statusChanged = true
			changed = append(changed, "status")
			if t.Status == api.StatusDone {
				doneAt := now
				t.DoneAt = &doneAt
			} else {
				t.DoneAt = nil
				// Leaving Done re-exposes the card on the board; the
				// dismiss flag was scoped to the previous Done visit.
				t.DismissedAt = nil
			}
		}
		if req.DueAt != nil {
			t.DueAt = req.DueAt
			changed = append(changed, "dueAt")
		}
		if req.SortOrder != nil && t.SortOrder != *req.SortOrder {
			t.SortOrder = *req.SortOrder
			changed = append(changed, "sortOrder")
		}
		t.UpdatedAt = now
		return nil
	})
	if err != nil {
		return api.Todo{}, err
	}

	if req.Tags != nil {
		if err := s.tags.SetTodoTags(ctx, id, *req.Tags); err != nil {
			return api.Todo{}, fmt.Errorf("set tags: %w", err)
		}
		changed = append(changed, "tags")
		if *req.Tags == nil {
			empty := []string{}
			req.Tags = &empty
		}
		updated.Tags = *req.Tags
	} else {
		tags, err := s.tags.GetTodoTags(ctx, id)
		if err != nil {
			return api.Todo{}, fmt.Errorf("get tags: %w", err)
		}
		updated.Tags = tags
	}

	if statusChanged {
		_ = s.events.Append(ctx, api.TodoEvent{
			TodoID:    id,
			Kind:      "status_changed",
			FromValue: string(statusFrom),
			ToValue:   string(statusTo),
			At:        now,
		})
		s.publish(api.EventTodoStatusChanged, api.TodoStatusChangedData{
			TodoID: id,
			From:   statusFrom,
			To:     statusTo,
		})
	}
	if prioChanged {
		_ = s.events.Append(ctx, api.TodoEvent{
			TodoID:    id,
			Kind:      "priority_changed",
			FromValue: priorityStr(prioFrom),
			ToValue:   priorityStr(prioTo),
			At:        now,
		})
	}
	if len(changed) > 0 {
		_ = s.events.Append(ctx, api.TodoEvent{
			TodoID:  id,
			Kind:    "edited",
			Payload: marshalChanged(changed),
			At:      now,
		})
		s.publish(api.EventTodoUpdated, api.TodoUpdatedData{Todo: updated, ChangedFields: changed})
	}
	return updated, nil
}

// DismissDone marks a Done todo as hidden from the kanban board without
// touching its lifecycle. Errors if the todo isn't in done status. Republishes
// EventTodoUpdated so subscribers can drop the card immediately.
func (s *Service) DismissDone(ctx context.Context, id string) (api.Todo, error) {
	now := s.clock.Now().Unix()
	updated, err := s.todos.Update(ctx, id, func(t *api.Todo) error {
		if t.Status != api.StatusDone {
			return fmt.Errorf("dismiss: todo %s is not in done status", id)
		}
		dismissed := now
		t.DismissedAt = &dismissed
		t.UpdatedAt = now
		return nil
	})
	if err != nil {
		return api.Todo{}, err
	}
	tags, err := s.tags.GetTodoTags(ctx, id)
	if err != nil {
		return api.Todo{}, fmt.Errorf("get tags: %w", err)
	}
	updated.Tags = tags
	_ = s.events.Append(ctx, api.TodoEvent{TodoID: id, Kind: "dismissed", At: now})
	s.publish(api.EventTodoUpdated, api.TodoUpdatedData{Todo: updated, ChangedFields: []string{"dismissedAt"}})
	return updated, nil
}

// Delete soft-deletes a todo and emits the deletion audit and bus events.
func (s *Service) Delete(ctx context.Context, id string) error {
	now := s.clock.Now().Unix()
	if err := s.todos.Delete(ctx, id); err != nil {
		return err
	}
	_ = s.events.Append(ctx, api.TodoEvent{TodoID: id, Kind: "deleted", At: now})
	s.publish(api.EventTodoDeleted, api.TodoDeletedData{TodoID: id})
	return nil
}

// Restore undoes a soft-delete: deleted_at is cleared and the freshly-restored
// Todo is published as TodoCreated so subscribers (the React store, the tray
// state subscriber) can re-add it without needing a separate event type.
func (s *Service) Restore(ctx context.Context, id string) (api.Todo, error) {
	t, err := s.todos.Restore(ctx, id)
	if err != nil {
		return api.Todo{}, err
	}
	tags, err := s.tags.GetTodoTags(ctx, id)
	if err != nil {
		return api.Todo{}, fmt.Errorf("get tags: %w", err)
	}
	t.Tags = tags
	now := s.clock.Now().Unix()
	_ = s.events.Append(ctx, api.TodoEvent{TodoID: id, Kind: "restored", At: now})
	s.publish(api.EventTodoCreated, api.TodoCreatedData{Todo: t})
	return t, nil
}

// ListDeleted returns soft-deleted todos with their tags, newest-deleted first.
func (s *Service) ListDeleted(ctx context.Context, limit int) ([]api.Todo, error) {
	list, err := s.todos.ListDeleted(ctx, limit)
	if err != nil {
		return nil, err
	}
	for i := range list {
		tags, err := s.tags.GetTodoTags(ctx, list[i].ID)
		if err != nil {
			return nil, fmt.Errorf("get tags for %s: %w", list[i].ID, err)
		}
		list[i].Tags = tags
	}
	return list, nil
}

// List returns todos with their tags loaded.
func (s *Service) List(ctx context.Context, f TodoFilter) ([]api.Todo, error) {
	list, err := s.todos.List(ctx, f)
	if err != nil {
		return nil, err
	}
	for i := range list {
		tags, err := s.tags.GetTodoTags(ctx, list[i].ID)
		if err != nil {
			return nil, fmt.Errorf("get tags for %s: %w", list[i].ID, err)
		}
		list[i].Tags = tags
	}
	return list, nil
}

// History returns audit events for a single todo (newest first).
func (s *Service) History(ctx context.Context, id string, limit int) ([]api.TodoEvent, error) {
	return s.events.ListByTodo(ctx, id, limit)
}

func newID() string {
	return ulid.MustNew(ulid.Now(), rand.Reader).String()
}

func priorityStr(p api.Priority) string {
	switch p {
	case api.PriorityLow:
		return "low"
	case api.PriorityHigh:
		return "high"
	case api.PriorityUrgent:
		return "urgent"
	default:
		return "normal"
	}
}

func marshalChanged(changed []string) string {
	b, _ := json.Marshal(map[string]any{"changedFields": changed})
	return string(b)
}

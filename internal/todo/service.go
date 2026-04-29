package todo

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/oklog/ulid/v2"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/clock"
)

// Service is the Todo domain entry point. It owns mutations, audit log emission and event publishing.
type Service struct {
	todos  TodoRepo
	tags   TagRepo
	events EventRepo
	clock  clock.Clock
	bus    api.EventPublisher
}

// NewService wires the service. bus may be nil.
func NewService(t TodoRepo, g TagRepo, e EventRepo, c clock.Clock, bus api.EventPublisher) *Service {
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
// replaces tags if provided, and emits audit and bus events. Returns the
// updated todo plus the pre-mutation status so handlers can wire transition-
// triggered side-effects (e.g. auto-timer) without a separate pre-flight Get
// (which would TOCTOU against a concurrent edit).
func (s *Service) Update(ctx context.Context, id string, req api.UpdateTodoRequest) (api.Todo, api.TodoStatus, error) {
	now := s.clock.Now().Unix()
	var changed []string
	var (
		oldStatus            api.TodoStatus
		statusFrom, statusTo api.TodoStatus
		statusChanged        bool
		prioFrom, prioTo     api.Priority
		prioChanged          bool
	)

	updated, err := s.todos.Update(ctx, id, func(t *api.Todo) error {
		oldStatus = t.Status
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
		return api.Todo{}, "", err
	}

	if req.Tags != nil {
		if err := s.tags.SetTodoTags(ctx, id, *req.Tags); err != nil {
			return api.Todo{}, "", fmt.Errorf("set tags: %w", err)
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
			return api.Todo{}, "", fmt.Errorf("get tags: %w", err)
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
	return updated, oldStatus, nil
}

// Move places `id` in (possibly new) target column, ordered between AfterID
// and BeforeID. Server picks a fresh sort_order; if neighbors leave no
// usable gap (equal values, or a sub-1.0 gap that would soon collide under
// successive averaging), the entire target column is rebalanced atomically
// alongside the move.
//
// Returns the post-mutation primary plus the pre-mutation status (for
// handlers' auto-timer side-effects). The pre-status is captured inside the
// repo's tx so concurrent edits between the handler's request and this call
// can't slip in undetected — closing the TOCTOU window the HTTP handler
// previously had via a separate pre-flight Get.
func (s *Service) Move(ctx context.Context, id string, req api.MoveTodoRequest) (api.Todo, api.TodoStatus, error) {
	now := s.clock.Now().Unix()

	moved, err := s.todos.Get(ctx, id)
	if err != nil {
		return api.Todo{}, "", err
	}

	targetCol := moved.Status
	if req.Status != nil {
		targetCol = *req.Status
	}

	// Snapshot the destination column without the moved card so neighbor
	// math doesn't trip over a stale position.
	colItems, err := s.todos.List(ctx, TodoFilter{
		Statuses:    []api.TodoStatus{targetCol},
		IncludeDone: true,
	})
	if err != nil {
		return api.Todo{}, "", err
	}
	others := make([]api.Todo, 0, len(colItems))
	for _, t := range colItems {
		if t.ID != id {
			others = append(others, t)
		}
	}
	// Sort DESC by sort_order — same as the frontend bucketize. Tie-break by
	// id so the rebalance pass produces stable values.
	sort.Slice(others, func(i, j int) bool {
		if others[i].SortOrder != others[j].SortOrder {
			return others[i].SortOrder > others[j].SortOrder
		}
		return others[i].ID > others[j].ID
	})

	// Find indices of the requested neighbors. Empty strings or missing IDs
	// mean "edge of column".
	afterIdx := -1
	beforeIdx := -1
	if req.AfterID != "" {
		for i, t := range others {
			if t.ID == req.AfterID {
				afterIdx = i
				break
			}
		}
	}
	if req.BeforeID != "" {
		for i, t := range others {
			if t.ID == req.BeforeID {
				beforeIdx = i
				break
			}
		}
	}

	// Build the post-drop list (others with the moved card spliced into
	// position). The split point is one index after the AfterID, or
	// BeforeID's index if AfterID was unspecified, or the column tail.
	var insertAt int
	switch {
	case afterIdx >= 0 && beforeIdx >= 0:
		insertAt = afterIdx + 1
	case afterIdx >= 0:
		insertAt = afterIdx + 1
	case beforeIdx >= 0:
		insertAt = beforeIdx
	default:
		insertAt = len(others)
	}
	if insertAt < 0 {
		insertAt = 0
	}
	if insertAt > len(others) {
		insertAt = len(others)
	}

	post := make([]api.Todo, 0, len(others)+1)
	post = append(post, others[:insertAt]...)
	movedCopy := moved
	post = append(post, movedCopy)
	post = append(post, others[insertAt:]...)

	// Try the cheap path: average the neighbors of the moved item. Bail to
	// a column-wide rebalance when the gap is too tight to fit a fresh
	// value (legacy data or float-precision drift from many halvings).
	const minGap = 1.0
	const step = 1024.0
	var newSortOrder float64
	rebalance := false
	if insertAt-1 >= 0 && insertAt < len(others) {
		above := others[insertAt-1].SortOrder
		below := others[insertAt].SortOrder
		gap := above - below
		if gap < minGap {
			rebalance = true
		} else {
			newSortOrder = (above + below) / 2
		}
	} else if insertAt-1 >= 0 {
		newSortOrder = others[insertAt-1].SortOrder - step
	} else if insertAt < len(others) {
		newSortOrder = others[insertAt].SortOrder + step
	} else {
		newSortOrder = step
	}
	if !rebalance {
		// Detect collisions elsewhere in the column too, so we don't ship
		// fresh data on top of a tie that'll bite the next drop.
		for i := 1; i < len(others); i++ {
			if others[i-1].SortOrder-others[i].SortOrder < minGap {
				rebalance = true
				break
			}
		}
	}

	siblings := []SortOrderUpdate{}
	if rebalance {
		base := step * float64(len(post)+1)
		for i, t := range post {
			s := base - float64(i+1)*step
			if t.ID == id {
				newSortOrder = s
				continue
			}
			siblings = append(siblings, SortOrderUpdate{ID: t.ID, SortOrder: s})
		}
	}

	// Capture the pre-mutation status inside the tx-bound mutate callback so
	// the value reflects the row at write-time, not a stale Get from before
	// the tx started. This is what handlers use for auto-timer side-effects.
	var oldStatus api.TodoStatus
	mutate := func(t *api.Todo) error {
		oldStatus = t.Status
		t.SortOrder = newSortOrder
		t.UpdatedAt = now
		if t.Status != targetCol {
			// Mirror Service.Update's status-side-effects so the column
			// invariants stay consistent regardless of which entry point
			// edited the row.
			if targetCol == api.StatusDone {
				doneAt := now
				t.DoneAt = &doneAt
			} else {
				t.DoneAt = nil
				t.DismissedAt = nil
			}
			t.Status = targetCol
		}
		return nil
	}

	updated, updatedSiblings, err := s.todos.MoveAndReorder(ctx, id, mutate, siblings)
	if err != nil {
		return api.Todo{}, "", err
	}
	tags, err := s.tags.GetTodoTags(ctx, id)
	if err != nil {
		return api.Todo{}, "", fmt.Errorf("get tags: %w", err)
	}
	updated.Tags = tags

	statusChanged := oldStatus != updated.Status
	if statusChanged {
		_ = s.events.Append(ctx, api.TodoEvent{
			TodoID:    id,
			Kind:      "status_changed",
			FromValue: string(oldStatus),
			ToValue:   string(updated.Status),
			At:        now,
		})
		s.publish(api.EventTodoStatusChanged, api.TodoStatusChangedData{
			TodoID: id,
			From:   oldStatus,
			To:     updated.Status,
		})
	}
	s.publish(api.EventTodoUpdated, api.TodoUpdatedData{
		Todo:          updated,
		ChangedFields: []string{"sortOrder"},
	})
	// Notify subscribers about the rebalanced siblings — using the slice
	// returned from the same tx, so a concurrent soft-delete can't drop a
	// sibling event between commit and the (now-eliminated) post-commit Get.
	for _, sib := range updatedSiblings {
		stags, _ := s.tags.GetTodoTags(ctx, sib.ID)
		sib.Tags = stags
		s.publish(api.EventTodoUpdated, api.TodoUpdatedData{
			Todo:          sib,
			ChangedFields: []string{"sortOrder"},
		})
	}
	return updated, oldStatus, nil
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

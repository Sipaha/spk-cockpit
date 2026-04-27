package tray

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/todo"
)

// EventSource is the slice of the event bus that Subscriber needs.
type EventSource interface {
	Subscribe(buf int) chan api.Event
	Unsubscribe(ch chan api.Event)
}

// TodoLister returns todos matching a filter — used for the overdue count
// and to resolve a timer's todo title.
type TodoLister interface {
	List(ctx context.Context, f todo.TodoFilter) ([]api.Todo, error)
	Get(ctx context.Context, id string) (api.Todo, error)
}

// Subscriber polls the event bus and pushes a fresh State to the tray on every
// relevant change, plus on a 30s tick.
type Subscriber struct {
	bus      EventSource
	tray     Backend
	todos    TodoLister
	mtgFetch func() *api.Meeting

	mu    sync.Mutex
	state State

	timerStartedAt int64
	timerTodoID    string
}

// NewSubscriber wires the bus, tray, todo lister, and meeting fetcher.
// mtgFetch and todos may be nil; the corresponding info disappears.
func NewSubscriber(bus EventSource, t Backend, todos TodoLister, mtgFetch func() *api.Meeting) *Subscriber {
	return &Subscriber{bus: bus, tray: t, todos: todos, mtgFetch: mtgFetch}
}

// Run subscribes and pushes State updates until ctx is done.
func (s *Subscriber) Run(ctx context.Context) {
	ch := s.bus.Subscribe(64)
	defer s.bus.Unsubscribe(ch)
	tick := time.NewTicker(30 * time.Second)
	defer tick.Stop()

	s.refreshAll(ctx)
	s.push()

	for {
		select {
		case <-ctx.Done():
			return
		case e, ok := <-ch:
			if !ok {
				return
			}
			s.handleEvent(ctx, e)
		case <-tick.C:
			s.refreshAll(ctx)
			s.push()
		}
	}
}

func (s *Subscriber) handleEvent(ctx context.Context, e api.Event) {
	switch e.Type {
	case api.EventTimerStarted:
		if d, ok := e.Data.(api.TimerStartedData); ok {
			s.mu.Lock()
			s.timerStartedAt = d.StartedAt
			s.timerTodoID = d.TodoID
			s.mu.Unlock()
			s.refreshTimerLabel(ctx)
		}
	case api.EventTimerStopped:
		s.mu.Lock()
		s.timerStartedAt = 0
		s.timerTodoID = ""
		s.state.TimerActive = false
		s.state.TimerLabel = ""
		s.mu.Unlock()
	case api.EventMeetingUpserted, api.EventMeetingDeleted, api.EventMeetingNotificationFired:
		s.refreshMeeting()
	case api.EventTodoCreated, api.EventTodoUpdated, api.EventTodoStatusChanged, api.EventTodoDeleted:
		s.refreshOverdue(ctx)
	case api.EventSyncStateChanged:
		if d, ok := e.Data.(api.SyncStateChangedData); ok {
			s.mu.Lock()
			if d.Status == "ok" {
				s.state.SyncError = ""
			} else {
				s.state.SyncError = d.LastErr
				if s.state.SyncError == "" {
					s.state.SyncError = "failed"
				}
			}
			s.mu.Unlock()
		}
	}
	s.push()
}

func (s *Subscriber) refreshAll(ctx context.Context) {
	s.refreshMeeting()
	s.refreshOverdue(ctx)
	s.refreshTimerLabel(ctx)
}

func (s *Subscriber) refreshMeeting() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.mtgFetch == nil {
		s.state.NextMeeting = ""
		return
	}
	m := s.mtgFetch()
	if m == nil {
		s.state.NextMeeting = ""
		return
	}
	until := time.Until(time.Unix(m.StartAt, 0))
	if until > 24*time.Hour || until < -10*time.Minute {
		s.state.NextMeeting = ""
		return
	}
	s.state.NextMeeting = fmt.Sprintf("%s in %s", m.Title, until.Round(time.Minute))
}

func (s *Subscriber) refreshOverdue(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.todos == nil {
		s.state.Overdue = 0
		return
	}
	now := time.Now().Unix()
	count := 0
	list, err := s.todos.List(ctx, todo.TodoFilter{
		Statuses:   []api.TodoStatus{api.StatusOpen, api.StatusInProgress},
		Priorities: []api.Priority{api.PriorityHigh, api.PriorityUrgent},
	})
	if err != nil {
		return
	}
	for _, t := range list {
		if t.DueAt != nil && *t.DueAt < now {
			count++
		}
	}
	s.state.Overdue = count
}

func (s *Subscriber) refreshTimerLabel(ctx context.Context) {
	s.mu.Lock()
	startedAt := s.timerStartedAt
	todoID := s.timerTodoID
	s.mu.Unlock()
	if startedAt == 0 || todoID == "" {
		s.mu.Lock()
		s.state.TimerActive = false
		s.state.TimerLabel = ""
		s.mu.Unlock()
		return
	}
	title := shortTodoID(todoID)
	if s.todos != nil {
		if t, err := s.todos.Get(ctx, todoID); err == nil && t.Title != "" {
			title = t.Title
		}
	}
	elapsed := time.Since(time.Unix(startedAt, 0)).Round(time.Second)
	s.mu.Lock()
	s.state.TimerActive = true
	s.state.TimerLabel = fmt.Sprintf("%s on %s", elapsed, title)
	s.mu.Unlock()
}

func (s *Subscriber) push() {
	s.mu.Lock()
	state := s.state
	s.mu.Unlock()
	s.tray.SetState(state)
}

func shortTodoID(id string) string {
	if len(id) <= 6 {
		return id
	}
	return id[len(id)-6:]
}

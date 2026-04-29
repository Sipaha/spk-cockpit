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

	// active holds every running timer keyed by todoID, so handleEvent stays
	// O(1) per started/stopped event without re-querying the timer repo.
	active map[string]int64 // todoID → startedAt
}

// NewSubscriber wires the bus, tray, todo lister, and meeting fetcher.
// mtgFetch and todos may be nil; the corresponding info disappears.
func NewSubscriber(bus EventSource, t Backend, todos TodoLister, mtgFetch func() *api.Meeting) *Subscriber {
	return &Subscriber{bus: bus, tray: t, todos: todos, mtgFetch: mtgFetch, active: map[string]int64{}}
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
			s.active[d.TodoID] = d.StartedAt
			s.mu.Unlock()
			s.refreshTimerLabel(ctx)
		}
	case api.EventTimerStopped:
		if d, ok := e.Data.(api.TimerStoppedData); ok {
			s.mu.Lock()
			delete(s.active, d.TodoID)
			s.mu.Unlock()
		}
		s.refreshTimerLabel(ctx)
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
		s.state.NextMeetingID = ""
		return
	}
	m := s.mtgFetch()
	if m == nil {
		s.state.NextMeeting = ""
		s.state.NextMeetingID = ""
		return
	}
	until := time.Until(time.Unix(m.StartAt, 0))
	if until > 24*time.Hour || until < -10*time.Minute {
		s.state.NextMeeting = ""
		s.state.NextMeetingID = ""
		return
	}
	s.state.NextMeeting = fmt.Sprintf("%s in %s", m.Title, until.Round(time.Minute))
	s.state.NextMeetingID = m.ID
}

func (s *Subscriber) refreshOverdue(ctx context.Context) {
	// Snapshot the dependency under the lock, then release it before doing the
	// SQL roundtrip — concurrent handleEvent calls would otherwise queue up
	// behind a slow DB query. Re-acquire only to mutate s.state.
	s.mu.Lock()
	todos := s.todos
	s.mu.Unlock()
	if todos == nil {
		s.mu.Lock()
		s.state.Overdue = 0
		s.mu.Unlock()
		return
	}
	now := time.Now().Unix()
	count := 0
	list, err := todos.List(ctx, todo.TodoFilter{
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
	s.mu.Lock()
	s.state.Overdue = count
	s.mu.Unlock()
}

func (s *Subscriber) refreshTimerLabel(ctx context.Context) {
	s.mu.Lock()
	if len(s.active) == 0 {
		s.state.TimerActive = false
		s.state.TimerLabel = ""
		s.mu.Unlock()
		return
	}
	// Pick the oldest (lowest startedAt) as the headline timer; that's the
	// session that's been burning the longest and is most likely to want a
	// glance from the user.
	var primary string
	var primaryStart int64
	for id, started := range s.active {
		if primary == "" || started < primaryStart {
			primary, primaryStart = id, started
		}
	}
	count := len(s.active)
	s.mu.Unlock()

	title := shortTodoID(primary)
	if s.todos != nil {
		if t, err := s.todos.Get(ctx, primary); err == nil && t.Title != "" {
			title = t.Title
		}
	}
	elapsed := time.Since(time.Unix(primaryStart, 0)).Round(time.Second)
	label := fmt.Sprintf("%s on %s", elapsed, title)
	if count > 1 {
		label = fmt.Sprintf("%s (+%d)", label, count-1)
	}
	s.mu.Lock()
	s.state.TimerActive = true
	s.state.TimerLabel = label
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

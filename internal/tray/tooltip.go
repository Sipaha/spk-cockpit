package tray

import (
	"context"
	"fmt"
	"time"

	"github.com/spk/spk-cockpit/internal/api"
)

// EventSource is the slice of the bus that Subscriber needs.
type EventSource interface {
	Subscribe(buf int) chan api.Event
	Unsubscribe(ch chan api.Event)
}

// activeTimer holds the state of the currently running timer session.
type activeTimer struct {
	todoID    string
	startedAt int64
}

// Subscriber polls the event bus and updates the tray tooltip on timer state changes.
type Subscriber struct {
	bus     EventSource
	tray    Backend
	current activeTimer
}

// NewSubscriber wires the bus and tray.
func NewSubscriber(bus EventSource, t Backend) *Subscriber {
	return &Subscriber{bus: bus, tray: t}
}

// Run subscribes and updates the tooltip until ctx is done.
// A 30s ticker keeps the elapsed time fresh while a timer is running.
func (s *Subscriber) Run(ctx context.Context) {
	ch := s.bus.Subscribe(32)
	defer s.bus.Unsubscribe(ch)
	tick := time.NewTicker(30 * time.Second)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case e, ok := <-ch:
			if !ok {
				return
			}
			s.handleEvent(e)
		case <-tick.C:
			s.refresh()
		}
	}
}

func (s *Subscriber) handleEvent(e api.Event) {
	switch e.Type {
	case api.EventTimerStarted:
		if d, ok := e.Data.(api.TimerStartedData); ok {
			s.current = activeTimer{todoID: d.TodoID, startedAt: d.StartedAt}
			s.refresh()
		}
	case api.EventTimerStopped:
		s.current = activeTimer{}
		s.tray.SetTooltip("spk-cockpit")
	}
}

func (s *Subscriber) refresh() {
	if s.current.todoID == "" {
		s.tray.SetTooltip("spk-cockpit")
		return
	}
	elapsed := time.Since(time.Unix(s.current.startedAt, 0)).Round(time.Second)
	s.tray.SetTooltip(fmt.Sprintf("spk-cockpit • %s on %s", elapsed, shortTodoID(s.current.todoID)))
}

func shortTodoID(id string) string {
	if len(id) <= 6 {
		return id
	}
	return id[len(id)-6:]
}

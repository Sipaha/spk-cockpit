package tray

import (
	"context"
	"fmt"
	"time"

	"github.com/spk/spk-cockpit/internal/api"
)

// Subscriber polls the event bus and updates the tray tooltip on timer / meeting events.
type Subscriber struct {
	bus      EventSource
	tray     Backend
	timer    activeTimer
	nextMeet nextMeeting
	mtgFetch func() *api.Meeting
}

// EventSource is the slice of the bus that Subscriber needs.
type EventSource interface {
	Subscribe(buf int) chan api.Event
	Unsubscribe(ch chan api.Event)
}

type activeTimer struct {
	todoID    string
	startedAt int64
}

type nextMeeting struct {
	id      string
	title   string
	startAt int64
}

// NewSubscriber wires the bus and tray. mtgFetch may be nil; when present it's
// invoked at startup, on each tick, and after MeetingUpserted/Deleted/NotificationFired.
func NewSubscriber(bus EventSource, t Backend, mtgFetch func() *api.Meeting) *Subscriber {
	return &Subscriber{bus: bus, tray: t, mtgFetch: mtgFetch}
}

// Run subscribes and updates the tooltip until ctx is done.
func (s *Subscriber) Run(ctx context.Context) {
	ch := s.bus.Subscribe(64)
	defer s.bus.Unsubscribe(ch)
	tick := time.NewTicker(30 * time.Second)
	defer tick.Stop()

	s.refreshMeeting()
	s.refresh()

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
			s.refreshMeeting()
			s.refresh()
		}
	}
}

func (s *Subscriber) handleEvent(e api.Event) {
	switch e.Type {
	case api.EventTimerStarted:
		if d, ok := e.Data.(api.TimerStartedData); ok {
			s.timer = activeTimer{todoID: d.TodoID, startedAt: d.StartedAt}
			s.refresh()
		}
	case api.EventTimerStopped:
		s.timer = activeTimer{}
		s.refresh()
	case api.EventMeetingUpserted, api.EventMeetingDeleted, api.EventMeetingNotificationFired:
		s.refreshMeeting()
		s.refresh()
	}
}

func (s *Subscriber) refreshMeeting() {
	if s.mtgFetch == nil {
		s.nextMeet = nextMeeting{}
		return
	}
	m := s.mtgFetch()
	if m == nil {
		s.nextMeet = nextMeeting{}
		return
	}
	s.nextMeet = nextMeeting{id: m.ID, title: m.Title, startAt: m.StartAt}
}

func (s *Subscriber) refresh() {
	switch {
	case s.timer.todoID != "":
		elapsed := time.Since(time.Unix(s.timer.startedAt, 0)).Round(time.Second)
		s.tray.SetTooltip(fmt.Sprintf("spk-cockpit • %s on %s", elapsed, shortTodoID(s.timer.todoID)))
	case s.nextMeet.id != "" && time.Until(time.Unix(s.nextMeet.startAt, 0)) < 24*time.Hour:
		until := time.Until(time.Unix(s.nextMeet.startAt, 0)).Round(time.Minute)
		s.tray.SetTooltip(fmt.Sprintf("spk-cockpit • next: %s in %s", s.nextMeet.title, until))
	default:
		s.tray.SetTooltip("spk-cockpit")
	}
}

func shortTodoID(id string) string {
	if len(id) <= 6 {
		return id
	}
	return id[len(id)-6:]
}

//go:build wails

// Package tray owns the system tray icon, menu, tooltip, and live-state
// update plumbing. The Controller is constructed from desktop.Run via
// OnReady; it subscribes to the eventbus and patches its tray menu items as
// timers start/stop, meetings approach, and CalDAV sync state changes.
package tray

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/spk/spk-cockpit/internal/api"
	"github.com/spk/spk-cockpit/internal/desktop"
	"github.com/spk/spk-cockpit/internal/eventbus"
	"github.com/spk/spk-cockpit/internal/todo"
)

// Actions wires tray menu clicks to daemon-side callbacks. These are populated
// by start.go where they have access to the relevant services.
type Actions struct {
	OpenStandup  func()
	StopTimer    func()
	QuickAddTodo func()
	OpenMeeting  func(meetingID string)
	Quit         func()
}

// TooltipState is the live state the tray menu reflects: active timer label,
// next meeting summary, overdue count, sync errors. Identical fields to v2's
// State struct except the TimerActive bool — derived from TimerLabel != "".
type TooltipState struct {
	TimerLabel    string
	NextMeeting   string
	NextMeetingID string
	Overdue       int
	SyncError     string
}

// Controller owns a v3 *application.SystemTray and patches the menu items as
// state changes. Mirrors the v2 Subscriber pattern but renders via v3's
// SystemTray.SetMenu API (rebuilding the menu cheaply on each refresh —
// systray menus are tiny).
type Controller struct {
	app     *application.App
	wnd     desktop.WindowHandle
	icon    []byte
	tray    *application.SystemTray
	actions Actions

	mu    sync.Mutex
	state TooltipState
	// active tracks current timer sessions keyed by todoID → startedAt.
	// Mirrors v2's Subscriber.active so we can pick the longest-burning
	// session as the headline timer.
	active map[string]int64

	runOnce sync.Once
}

// NewController constructs the tray, sets the menu, and registers the click
// handler that toggles the main window. Returns an error if the v3 systray
// allocation fails.
func NewController(
	app *application.App,
	wnd desktop.WindowHandle,
	icon []byte,
	actions Actions,
) (*Controller, error) {
	c := &Controller{
		app:     app,
		wnd:     wnd,
		icon:    icon,
		actions: actions,
		active:  map[string]int64{},
	}
	c.tray = app.SystemTray.New()
	c.tray.SetIcon(icon)
	c.tray.SetTooltip("spk-cockpit")
	c.refreshMenu()
	c.tray.OnClick(c.toggleWindow)
	return c, nil
}

// toggleWindow is the left-click handler. Three-way matrix:
//
//	visible + focused           → Hide (click-to-dismiss)
//	visible + not focused       → Show (raise to front; user wants it up)
//	visible + minimised         → Show (raise + restore)
//	hidden  (close-to-tray)     → Show
//
// WindowHandle.Show already does the keep-above + Focus dance, so we just
// call it for everything that isn't "Hide".
func (c *Controller) toggleWindow() {
	if c.wnd == nil {
		return
	}
	if c.wnd.IsVisible() && c.wnd.IsFocused() {
		c.wnd.Hide()
		return
	}
	c.wnd.Show()
}

// Run subscribes to the bus and consumes events until ctx is cancelled or the
// channel closes. Safe to call multiple times; only the first call wires up.
func (c *Controller) Run(
	ctx context.Context,
	bus *eventbus.Bus,
	todos *todo.Service,
	mtgFetch func() *api.Meeting,
) {
	c.runOnce.Do(func() {
		go c.consume(ctx, bus, todos, mtgFetch)
	})
}

// refreshMenu rebuilds the v3 menu based on c.state. Cheap — systray menus
// have <10 items and v3 handles this at GTK level fine.
func (c *Controller) refreshMenu() {
	c.mu.Lock()
	st := c.state
	c.mu.Unlock()

	menu := application.NewMenu()

	if st.TimerLabel != "" {
		menu.Add("Timer: " + st.TimerLabel).SetEnabled(false)
	}
	if st.NextMeeting != "" {
		nextID := st.NextMeetingID
		menu.Add("Next: " + st.NextMeeting).OnClick(func(*application.Context) {
			if nextID != "" && c.actions.OpenMeeting != nil {
				c.actions.OpenMeeting(nextID)
			}
		})
	}
	if st.Overdue > 0 {
		menu.Add(fmt.Sprintf("Overdue: %d", st.Overdue)).SetEnabled(false)
	}
	if st.SyncError != "" {
		menu.Add("⚠ Sync: " + st.SyncError).SetEnabled(false)
	}
	if st.TimerLabel != "" || st.NextMeeting != "" || st.Overdue > 0 || st.SyncError != "" {
		menu.AddSeparator()
	}

	menu.Add("Open").OnClick(func(*application.Context) {
		c.toggleWindow()
	})
	if c.actions.OpenStandup != nil {
		menu.Add("Open Standup").OnClick(func(*application.Context) { c.actions.OpenStandup() })
	}
	if c.actions.QuickAddTodo != nil {
		menu.Add("Quick Add Todo").OnClick(func(*application.Context) { c.actions.QuickAddTodo() })
	}
	if c.actions.StopTimer != nil {
		menu.Add("Stop Timer").OnClick(func(*application.Context) { c.actions.StopTimer() })
	}
	menu.AddSeparator()
	if c.actions.Quit != nil {
		menu.Add("Quit").OnClick(func(*application.Context) { c.actions.Quit() })
	}
	c.tray.SetMenu(menu)
}

// setState mutates the tooltip-state under lock and re-renders the menu.
func (c *Controller) setState(mut func(*TooltipState)) {
	c.mu.Lock()
	mut(&c.state)
	c.mu.Unlock()
	c.refreshMenu()
}

// consume is the live-update goroutine. Mirrors v2's Subscriber.Run dispatch.
// Three input sources: ctx cancellation, bus events, and a 30-second tick
// that re-runs all refreshers (keeps the "Standup in 5m" countdown advancing).
func (c *Controller) consume(
	ctx context.Context,
	bus *eventbus.Bus,
	todos *todo.Service,
	mtgFetch func() *api.Meeting,
) {
	ch := bus.Subscribe(64)
	defer bus.Unsubscribe(ch)

	tick := time.NewTicker(30 * time.Second)
	defer tick.Stop()

	c.refreshAll(ctx, todos, mtgFetch)

	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			c.handleEvent(ctx, ev, todos, mtgFetch)
		case <-tick.C:
			c.refreshAll(ctx, todos, mtgFetch)
		}
	}
}

func (c *Controller) refreshAll(
	ctx context.Context,
	todos *todo.Service,
	mtgFetch func() *api.Meeting,
) {
	c.refreshTimerLabel(ctx, todos)
	c.refreshOverdue(ctx, todos)
	c.refreshMeeting(ctx, mtgFetch)
}

func (c *Controller) handleEvent(
	ctx context.Context,
	ev api.Event,
	todos *todo.Service,
	mtgFetch func() *api.Meeting,
) {
	switch ev.Type {
	case api.EventTimerStarted:
		if d, ok := ev.Data.(api.TimerStartedData); ok {
			c.mu.Lock()
			c.active[d.TodoID] = d.StartedAt
			c.mu.Unlock()
			c.refreshTimerLabel(ctx, todos)
		}
	case api.EventTimerStopped:
		if d, ok := ev.Data.(api.TimerStoppedData); ok {
			c.mu.Lock()
			delete(c.active, d.TodoID)
			c.mu.Unlock()
			c.refreshTimerLabel(ctx, todos)
		}
	case api.EventMeetingUpserted, api.EventMeetingDeleted, api.EventMeetingNotificationFired:
		c.refreshMeeting(ctx, mtgFetch)
	case api.EventTodoCreated, api.EventTodoUpdated, api.EventTodoStatusChanged, api.EventTodoDeleted:
		c.refreshOverdue(ctx, todos)
	case api.EventSyncStateChanged:
		if d, ok := ev.Data.(api.SyncStateChangedData); ok {
			c.setState(func(st *TooltipState) {
				if d.Status == "ok" {
					st.SyncError = ""
				} else {
					msg := d.LastErr
					if msg == "" {
						msg = "failed"
					}
					st.SyncError = msg
				}
			})
		}
	}
}

// refreshTimerLabel inlines v2's tooltip.go logic verbatim. Picks the oldest
// active session as the headline timer (longest-burning first), looks up the
// todo title for human readability, formats "elapsed on title" with a "(+N)"
// suffix for additional concurrent timers.
func (c *Controller) refreshTimerLabel(ctx context.Context, todos *todo.Service) {
	c.mu.Lock()
	if len(c.active) == 0 {
		c.mu.Unlock()
		c.setState(func(st *TooltipState) { st.TimerLabel = "" })
		return
	}
	var primary string
	var primaryStart int64
	for id, started := range c.active {
		if primary == "" || started < primaryStart {
			primary, primaryStart = id, started
		}
	}
	count := len(c.active)
	c.mu.Unlock()

	title := shortTodoID(primary)
	if todos != nil {
		if t, err := todos.Get(ctx, primary); err == nil && t.Title != "" {
			title = t.Title
		}
	}
	elapsed := time.Since(time.Unix(primaryStart, 0)).Round(time.Second)
	label := fmt.Sprintf("%s on %s", elapsed, title)
	if count > 1 {
		label = fmt.Sprintf("%s (+%d)", label, count-1)
	}
	c.setState(func(st *TooltipState) { st.TimerLabel = label })
}

// refreshOverdue queries the todo service for high/urgent open tasks and
// counts those past due. The DB roundtrip happens WITHOUT holding c.mu — this
// is the lock-around-IO fix audit-applied to v2's tooltip.go and MUST be
// preserved. The only c.mu acquisition is inside setState (the final
// state.Overdue write).
func (c *Controller) refreshOverdue(ctx context.Context, todos *todo.Service) {
	if todos == nil {
		c.setState(func(st *TooltipState) { st.Overdue = 0 })
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
	c.setState(func(st *TooltipState) { st.Overdue = count })
}

func (c *Controller) refreshMeeting(_ context.Context, mtgFetch func() *api.Meeting) {
	if mtgFetch == nil {
		c.setState(func(st *TooltipState) { st.NextMeeting = ""; st.NextMeetingID = "" })
		return
	}
	m := mtgFetch()
	if m == nil {
		c.setState(func(st *TooltipState) { st.NextMeeting = ""; st.NextMeetingID = "" })
		return
	}
	until := time.Until(time.Unix(m.StartAt, 0))
	if until > 24*time.Hour || until < -10*time.Minute {
		c.setState(func(st *TooltipState) { st.NextMeeting = ""; st.NextMeetingID = "" })
		return
	}
	label := fmt.Sprintf("%s in %s", m.Title, until.Round(time.Minute))
	c.setState(func(st *TooltipState) { st.NextMeeting = label; st.NextMeetingID = m.ID })
}

// shortTodoID is the v2 fallback when a todo title can't be resolved; shows
// the last 6 chars of the ULID.
func shortTodoID(id string) string {
	if len(id) <= 6 {
		return id
	}
	return id[len(id)-6:]
}

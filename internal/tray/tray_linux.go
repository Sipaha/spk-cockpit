//go:build linux

package tray

import (
	"fmt"
	"sync"

	"fyne.io/systray"

	"github.com/spk/spk-cockpit/internal/appfiles"
)

type linuxTray struct {
	actions Actions

	mu sync.Mutex
	// Menu items kept for live updates.
	timerInfo   *systray.MenuItem
	meetingInfo *systray.MenuItem
	overdueInfo *systray.MenuItem
	syncInfo    *systray.MenuItem
	stopTimer   *systray.MenuItem
	openStandup *systray.MenuItem
	openWindow  *systray.MenuItem
	quit        *systray.MenuItem
	// nextMeetingID is the id of the meeting currently summarized by
	// meetingInfo; the click handler reads it under t.mu so the deep-link is
	// always consistent with the visible label.
	nextMeetingID string

	pending *State // last SetState before menu was built
}

// New constructs a Linux tray backend wired to the given Actions.
func New(a Actions) Backend {
	return &linuxTray{actions: a}
}

// Run starts the tray loop. onReady fires after icon/menu are visible.
//
// SetOnTapped MUST be called before systray.Run: fyne.io/systray decides the
// StatusNotifierItem `ItemIsMenu` property at registration time based on
// whether tappedLeft is nil. If we set it inside the onReady callback the
// panel already sees ItemIsMenu=true and routes LMB to the menu instead of
// invoking Activate.
func (t *linuxTray) Run(onReady func(), onExit func()) {
	systray.SetOnTapped(func() {
		if t.actions.OpenWindow != nil {
			go t.actions.OpenWindow()
		}
	})
	systray.Run(func() {
		systray.SetIcon(appfiles.TrayIcon)
		systray.SetTooltip("spk-cockpit")

		t.mu.Lock()
		t.timerInfo = systray.AddMenuItem("⏱  idle", "")
		t.timerInfo.Disable()
		// meetingInfo is intentionally enabled even when no meeting is set:
		// fyne.io/systray suppresses click events on disabled items, and we
		// want the entry to be clickable as soon as a meeting appears.
		t.meetingInfo = systray.AddMenuItem("📅  no upcoming meeting", "")
		t.overdueInfo = systray.AddMenuItem("✓  no overdue", "")
		t.overdueInfo.Disable()
		t.syncInfo = systray.AddMenuItem("✓  sync ok", "")
		t.syncInfo.Disable()
		t.syncInfo.Hide()

		systray.AddSeparator()
		t.openWindow = systray.AddMenuItem("Open window", "")
		t.openStandup = systray.AddMenuItem("Open standup", "")

		systray.AddSeparator()
		t.stopTimer = systray.AddMenuItem("Stop timer", "")
		t.stopTimer.Disable()

		systray.AddSeparator()
		t.quit = systray.AddMenuItem("Quit", "")
		pending := t.pending
		t.mu.Unlock()

		if pending != nil {
			t.applyState(*pending)
		}

		go t.dispatchClicks()

		if onReady != nil {
			onReady()
		}
	}, onExit)
}

// SetState updates the live menu items and tooltip. Safe before Run completes:
// state is buffered and applied once the menu is built.
func (t *linuxTray) SetState(s State) {
	t.mu.Lock()
	if t.timerInfo == nil {
		// menu not ready yet; buffer and apply on Run.
		buf := s
		t.pending = &buf
		t.mu.Unlock()
		return
	}
	t.mu.Unlock()
	t.applyState(s)
}

// Quit terminates the tray loop.
func (t *linuxTray) Quit() { systray.Quit() }

func (t *linuxTray) applyState(s State) {
	if s.TimerActive && s.TimerLabel != "" {
		t.timerInfo.SetTitle("⏱  " + s.TimerLabel)
		t.stopTimer.Enable()
	} else {
		t.timerInfo.SetTitle("⏱  idle")
		t.stopTimer.Disable()
	}

	t.mu.Lock()
	t.nextMeetingID = s.NextMeetingID
	t.mu.Unlock()
	if s.NextMeeting != "" {
		t.meetingInfo.SetTitle("📅  " + s.NextMeeting)
	} else {
		t.meetingInfo.SetTitle("📅  no upcoming meeting")
	}

	if s.Overdue > 0 {
		t.overdueInfo.SetTitle(fmt.Sprintf("⚠  %d overdue", s.Overdue))
	} else {
		t.overdueInfo.SetTitle("✓  no overdue")
	}

	if s.SyncError != "" {
		t.syncInfo.SetTitle("⚠  sync: " + s.SyncError)
		t.syncInfo.Show()
	} else {
		t.syncInfo.Hide()
	}

	systray.SetTooltip(buildTooltip(s))
}

func buildTooltip(s State) string {
	switch {
	case s.TimerActive && s.TimerLabel != "":
		return "spk-cockpit • " + s.TimerLabel
	case s.NextMeeting != "":
		return "spk-cockpit • next: " + s.NextMeeting
	case s.Overdue > 0:
		return fmt.Sprintf("spk-cockpit • %d overdue", s.Overdue)
	case s.SyncError != "":
		return "spk-cockpit • sync error"
	default:
		return "spk-cockpit"
	}
}

func (t *linuxTray) dispatchClicks() {
	for {
		t.mu.Lock()
		owCh := t.openWindow.ClickedCh
		osCh := t.openStandup.ClickedCh
		stCh := t.stopTimer.ClickedCh
		mtCh := t.meetingInfo.ClickedCh
		quitCh := t.quit.ClickedCh
		t.mu.Unlock()

		select {
		case <-owCh:
			if t.actions.OpenWindow != nil {
				go t.actions.OpenWindow()
			}
		case <-osCh:
			if t.actions.OpenStandup != nil {
				go t.actions.OpenStandup()
			}
		case <-stCh:
			if t.actions.StopTimer != nil {
				go t.actions.StopTimer()
			}
		case <-mtCh:
			t.mu.Lock()
			id := t.nextMeetingID
			t.mu.Unlock()
			if id != "" && t.actions.OpenMeeting != nil {
				go t.actions.OpenMeeting(id)
			} else if t.actions.OpenWindow != nil {
				// Click on the placeholder "no upcoming meeting" entry: just
				// surface the window so the user can poke around the calendar.
				go t.actions.OpenWindow()
			}
		case <-quitCh:
			if t.actions.Quit != nil {
				t.actions.Quit()
			}
			systray.Quit()
			return
		}
	}
}

// Package tray wraps the system tray icon and menu behind a small interface so
// the platform-specific implementation can be swapped (Linux only in v1).
package tray

// Actions are the click handlers wired by the daemon at startup. Each may be nil;
// the corresponding menu item is then disabled.
type Actions struct {
	// OpenWindow brings the main Wails window forward (last route).
	OpenWindow func()
	// OpenStandup brings the window forward and navigates to /standup.
	OpenStandup func()
	// StopTimer stops the currently active timer session.
	StopTimer func()
	// OpenMeeting brings the window forward focused on a specific meeting
	// (deep-link to /calendar?focus=<id>). Used by the next-meeting tray entry.
	OpenMeeting func(id string)
	// Quit terminates the daemon.
	Quit func()
}

// State is the live information the tray surfaces in its menu and tooltip.
// Subscriber pushes a fresh State whenever any field changes.
type State struct {
	// TimerActive is true when a timer is running.
	TimerActive bool
	// TimerLabel is the human-readable timer summary, e.g. "5m21s on Refactor parser".
	TimerLabel string
	// NextMeeting is the next meeting summary if any, e.g. "Daily standup in 17m".
	NextMeeting string
	// NextMeetingID is the id of the meeting summarized in NextMeeting; empty
	// when no upcoming meeting is shown. Used to deep-link the menu entry.
	NextMeetingID string
	// Overdue is the number of urgent/high open todos with due_at in the past.
	Overdue int
	// SyncError is non-empty when the most recent CalDAV sync failed.
	SyncError string
}

// Backend is the surface the rest of the app uses to interact with the tray.
type Backend interface {
	// Run blocks for the lifetime of the tray. onReady is invoked once the
	// system tray API is initialized; onExit when the loop is exiting.
	Run(onReady func(), onExit func())
	// SetState updates the live menu items and tooltip from State.
	SetState(s State)
	// Quit asks the tray to exit. The exit callback passed to Run will fire.
	Quit()
}

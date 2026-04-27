// Package tray wraps the system tray icon and menu behind a small interface so
// the platform-specific implementation can be swapped (Linux only in phase 1).
package tray

// Backend is the surface the rest of the app uses to interact with the tray.
type Backend interface {
	// Run blocks for the lifetime of the tray. onReady is invoked once the
	// system tray API is initialized; onExit when the loop is exiting.
	Run(onReady func(), onExit func())
	// SetTooltip updates the tray tooltip.
	SetTooltip(s string)
	// Quit asks the tray to exit. The exit callback passed to Run will fire.
	Quit()
}

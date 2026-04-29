//go:build wails

package desktop

import (
	"net/url"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// runPopup opens a frameless meeting-popup child window navigated to
// /popup-meeting?id=<id>. The frontend already has a popup-meeting route
// (used today by the v2 subprocess version) — we just navigate the new
// window there. AlwaysOnTop ensures the popup floats above the user's
// current workspace; the user dismisses with Esc which calls closeWindow().
func runPopup(app *application.App, _ string, meetingID string) {
	q := url.Values{}
	q.Set("id", meetingID)
	w := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:           "Meeting",
		Width:           520,
		Height:          360,
		URL:             "/popup-meeting?" + q.Encode(),
		AlwaysOnTop:     true,
		Frameless:       true,
		DevToolsEnabled: devToolsEnabled,
	})
	// Force the popup forward — same focus-stealing workaround as runQuickAdd.
	// Without this, the popup can land BEHIND the user's current window on
	// Mutter / KWin / xfwm even with AlwaysOnTop: true on creation.
	w.Show()
	w.SetAlwaysOnTop(true)
	w.Focus()
	w.SetAlwaysOnTop(false)
}

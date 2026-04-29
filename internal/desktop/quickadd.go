//go:build wails

package desktop

import (
	"github.com/wailsapp/wails/v3/pkg/application"
)

// runQuickAdd opens the quick-add child window. The frontend has a
// /quick-add-todo route (registered in web/src/App.tsx) that renders the
// small textarea + Esc-to-close UX; we just navigate the new window there.
// The parent app is shared so the child window picks up the same Asset
// middleware (UDS proxy for /api/*).
//
// The second parameter (socketPath) is currently unused — the asset
// middleware on the parent app already proxies /api/* — but is kept in the
// signature for symmetry with runPopup and to leave room for future
// per-window socket overrides.
func runQuickAdd(app *application.App, _ string) {
	w := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:           "Quick Add",
		Width:           480,
		Height:          200,
		URL:             "/quick-add-todo",
		AlwaysOnTop:     true,
		DevToolsEnabled: devToolsEnabled,
	})
	// Force the keep-above + focus dance so the new window lands above the
	// user's current workspace, not behind it (Linux WM focus-stealing
	// prevention drops a plain Focus() request on a freshly-mapped window).
	w.Show()
	w.SetAlwaysOnTop(true)
	w.Focus()
	w.SetAlwaysOnTop(false)
}

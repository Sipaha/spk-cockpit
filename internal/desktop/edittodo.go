//go:build wails

package desktop

import (
	"net/url"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// runEditTodo opens the full todo editor child window. With todoID empty
// the window renders the create form; with todoID set the page fetches
// the todo and pre-populates. Same focus-stealing-defeat dance as the
// other child windows so the editor lands above the user's workspace.
func runEditTodo(app *application.App, _ string, todoID string) {
	target := "/edit-todo"
	if todoID != "" {
		q := url.Values{}
		q.Set("id", todoID)
		target = target + "?" + q.Encode()
	}
	w := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:           "Edit Todo",
		Width:           640,
		Height:          540,
		URL:             target,
		AlwaysOnTop:     true,
		DevToolsEnabled: devToolsEnabled,
	})
	w.Show()
	w.SetAlwaysOnTop(true)
	w.Focus()
	w.SetAlwaysOnTop(false)
}

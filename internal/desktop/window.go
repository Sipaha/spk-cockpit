//go:build wails

// Package desktop launches the Wails v3 application: a single primary window
// (HideOnClose), an event hookup point for tray + popup, and an asset middleware
// that proxies /api/* to the daemon's UDS.
package desktop

import (
	"context"
	"io/fs"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// Geometry is the persisted window position and size. Mirrors the v2 type.
type Geometry struct {
	X, Y, Width, Height int
}

// WindowHandle is the small surface the tray + popup callbacks need from the
// main window. We expose only what's wired in start.go's tray actions so the
// rest of the project stays decoupled from *application.WebviewWindow.
//
// Show() does the keep-above + Focus() dance internally so callers don't
// need to repeat the workaround. IsVisible() returns false for both hidden
// AND minimised windows (treats minimised as "not really visible").
type WindowHandle interface {
	Show()
	Focus()
	Hide()
	IsVisible() bool
	IsFocused() bool
	Navigate(path string)
}

// Options bundles the dependencies Run needs.
type Options struct {
	FrontendFS   fs.FS
	SocketPath   string
	IconPNG      []byte
	LoadGeometry func() *Geometry
	SaveGeometry func(Geometry)

	// OnReady fires once the main window is constructed and the runtime
	// context is live. Receivers (tray, scheduler-popup callback) capture
	// the handle for later Show()/Focus() calls. The *application.App is
	// exposed so tray.NewController can call app.SystemTray.New() —
	// passing it through the callback keeps the WindowHandle interface
	// itself free of v3-specific types.
	OnReady func(
		app *application.App,
		main WindowHandle,
		openQuickAdd func(),
		openMeetingPopup func(meetingID string),
		openEditTodo func(todoID string),
	)
}

// Run starts the Wails event loop. It blocks until the app quits.
func Run(ctx context.Context, opts Options) error {
	// openEditTodoPtr breaks the initialization cycle: udsMiddleware needs the
	// callback at construction time, but runEditTodo needs app which isn't
	// available until application.New returns. The atomic store below bridges
	// the gap — same pattern as openPopup in start_wails.go.
	var openEditTodoPtr atomic.Pointer[func(string)]
	app := application.New(application.Options{
		Name:        "spk-cockpit",
		Description: "Personal productivity tray app",
		Icon:        opts.IconPNG,
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(opts.FrontendFS),
			Middleware: udsMiddleware(opts.SocketPath, func(id string) {
				if fn := openEditTodoPtr.Load(); fn != nil {
					(*fn)(id)
				}
			}),
		},
	})

	width, height := 1100, 720
	var startX, startY int
	hasPos := false
	if opts.LoadGeometry != nil {
		if g := opts.LoadGeometry(); g != nil {
			if g.Width > 0 && g.Height > 0 {
				width, height = g.Width, g.Height
			}
			if g.X != 0 || g.Y != 0 {
				startX, startY = g.X, g.Y
				hasPos = true
			}
		}
	}

	wnd := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:                      "SPK Cockpit",
		Width:                      width,
		Height:                     height,
		URL:                        "/",
		DevToolsEnabled:            devToolsEnabled,
		DefaultContextMenuDisabled: false,
	})
	if hasPos {
		wnd.SetPosition(startX, startY)
	}

	// Hide-on-close: the X close button hides the window so the tray menu
	// remains the only way back. Replaces v2's HideWindowOnClose option.
	wnd.RegisterHook(events.Common.WindowClosing, func(ev *application.WindowEvent) {
		ev.Cancel()
		wnd.Hide()
	})

	// Geometry persistence — throttled poll, NOT per-event. Wails fires
	// WindowDidMove/Resize at every drag tick which would hit SQLite tens of
	// times a second. v2 ran a 2-second polling tick; we keep the same cadence.
	// The poll goroutine exits when ctx is cancelled.
	if opts.SaveGeometry != nil {
		go pollGeometry(ctx, wnd, opts.SaveGeometry)
	}

	main := windowHandle{wnd: wnd}
	openQuickAdd := func() { runQuickAdd(app, opts.SocketPath) }
	openMeetingPopup := func(id string) { runPopup(app, opts.SocketPath, id) }
	openEditTodo := func(id string) { runEditTodo(app, opts.SocketPath, id) }
	openEditTodoPtr.Store(&openEditTodo)

	if opts.OnReady != nil {
		opts.OnReady(app, main, openQuickAdd, openMeetingPopup, openEditTodo)
	}

	// Wire ctx → app.Quit. The done channel kills the goroutine when
	// app.Run returns naturally (user clicks Quit in tray menu) so we don't
	// leak it to process-end and don't double-call app.Quit() — calling
	// Quit on an already-quit app is best avoided in alpha.
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			app.Quit()
		case <-done:
		}
	}()
	defer close(done)

	return app.Run()
}

// pollGeometry samples wnd.Position()/Size() every 2 seconds and persists
// fresh values via save. Mirrors v2's pollGeometry — same anti-flapping
// guards: skip the (0,0) sample that GTK reports for a hidden window, skip
// minimised state, drop zero/negative size.
func pollGeometry(ctx context.Context, wnd *application.WebviewWindow, save func(Geometry)) {
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	var last Geometry
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			if wnd.IsMinimised() {
				continue
			}
			x, y := wnd.Position()
			w, h := wnd.Size()
			if w <= 0 || h <= 0 {
				continue
			}
			// (0,0) is the canonical "hidden" reply on Linux. Drop it whenever
			// we've previously captured a real position.
			if x == 0 && y == 0 && (last.X != 0 || last.Y != 0) {
				continue
			}
			g := Geometry{X: x, Y: y, Width: w, Height: h}
			if g == last {
				continue
			}
			last = g
			save(g)
		}
	}
}

// windowHandle is the small WindowHandle implementation used by tray/popup.
type windowHandle struct {
	wnd *application.WebviewWindow
}

// Show un-hides + brings forward + focuses, defeating Linux WM focus-stealing
// prevention via the keep-above toggle around Focus(). Mirrors spk-mail's
// raiseToFront pattern verbatim — gtk_window_present alone (which Focus()
// calls under the hood) is treated by Mutter/KWin/Xfwm/Sway as a background
// request without a user-activation timestamp; toggling AlwaysOnTop is an
// ICCCM/EWMH window-state hint that's unconditionally honoured.
func (h windowHandle) Show() {
	switch {
	case !h.wnd.IsVisible():
		h.wnd.Show()
	case h.wnd.IsMinimised():
		h.wnd.Restore()
	}
	h.wnd.SetAlwaysOnTop(true)
	h.wnd.Focus()
	h.wnd.SetAlwaysOnTop(false)
}
func (h windowHandle) Focus()            { h.wnd.Focus() }
func (h windowHandle) Hide()             { h.wnd.Hide() }
func (h windowHandle) IsVisible() bool   { return h.wnd.IsVisible() && !h.wnd.IsMinimised() }
func (h windowHandle) IsFocused() bool   { return h.wnd.IsFocused() }
func (h windowHandle) Navigate(p string) { h.wnd.SetURL(p) }

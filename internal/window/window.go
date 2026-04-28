// Package window wraps Wails to render the embedded React UI in a webview window.
// /api/* requests from the webview are proxied over the cockpit daemon's UDS.
package window

import (
	"context"
	"embed"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/spk/spk-cockpit/internal/appfiles"
)

// App holds the running Wails context and lets external code bring the window forward.
type App struct {
	ctx        context.Context
	socketPath string
}

// NewApp constructs an App. socketPath is where /api/* requests are proxied.
func NewApp(socketPath string) *App { return &App{socketPath: socketPath} }

func (a *App) onStartup(ctx context.Context) { a.ctx = ctx }

// Show brings the main window forward.
func (a *App) Show() {
	if a.ctx != nil {
		wruntime.WindowShow(a.ctx)
	}
}

// Quit stops the Wails event loop so the surrounding process can exit. Used by
// the tray Quit action — without this, cancelling ctx only stops the HTTP/SSE
// server but Wails keeps the main thread alive until the window is closed manually.
func (a *App) Quit() {
	if a.ctx != nil {
		wruntime.Quit(a.ctx)
	}
}

// ShowAt brings the main window forward and navigates the embedded React app
// to `path` (e.g. "/standup"). The navigation is best-effort; if the JS bridge
// is unavailable, the window still surfaces at its last route.
func (a *App) ShowAt(path string) {
	if a.ctx == nil {
		return
	}
	wruntime.WindowShow(a.ctx)
	// Use history.pushState so the React router (BrowserRouter) picks it up
	// without a full reload. We dispatch popstate so listeners refresh.
	js := "history.pushState(null,'',\"" + path + "\");window.dispatchEvent(new PopStateEvent('popstate'));"
	wruntime.WindowExecJS(a.ctx, js)
}

// RunPopup starts a small standalone Wails window that opens directly on the
// meeting popup route. It blocks until the user closes the window, then returns
// (the calling process exits naturally — popup is one-shot).
//
// Used by `cockpit popup --meeting-id=<id>` subprocess to render an OS-native
// pre-meeting popup that is independent from the main spk-cockpit window.
func RunPopup(assets embed.FS, socketPath, meetingID string) error {
	js := "(function(){var u='/popup-meeting?id=" + meetingID + "';if(location.pathname+location.search!==u){history.replaceState(null,'',u);window.dispatchEvent(new PopStateEvent('popstate'));}})();"
	return wails.Run(&options.App{
		Title:  "Meeting",
		Width:  520,
		Height: 360,
		AssetServer: &assetserver.Options{
			Assets:     assets,
			Middleware: udsMiddleware(socketPath),
		},
		HideWindowOnClose: false,
		AlwaysOnTop:       true,
		Linux:             &linux.Options{ProgramName: "spk-cockpit-popup"},
		OnDomReady: func(ctx context.Context) {
			wruntime.WindowExecJS(ctx, js)
		},
	})
}

// Geometry is the persisted window position and size.
type Geometry struct {
	X, Y, Width, Height int
}

// Run starts the Wails event loop. It blocks until the window is closed.
// ready is invoked once the App is constructed (before the loop spins up) so callers
// can capture a handle for tray actions.
//
// loadGeometry returns previously persisted window geometry (or nil for first
// run); saveGeometry is invoked at shutdown so size and position survive
// restarts. Either may be nil to disable persistence.
func Run(assets embed.FS, socketPath string, ready func(*App), loadGeometry func() *Geometry, saveGeometry func(Geometry)) error {
	app := NewApp(socketPath)
	go func() {
		// Give Wails a beat to wire OnStartup; ready receives the app handle either way.
		time.Sleep(200 * time.Millisecond)
		if ready != nil {
			ready(app)
		}
	}()

	width, height := 1100, 720
	var startX, startY int
	hasPos := false
	if loadGeometry != nil {
		if g := loadGeometry(); g != nil {
			if g.Width > 0 && g.Height > 0 {
				width, height = g.Width, g.Height
			}
			if g.X != 0 || g.Y != 0 {
				startX, startY = g.X, g.Y
				hasPos = true
			}
		}
	}

	return wails.Run(&options.App{
		Title:  "SPK Cockpit",
		Width:  width,
		Height: height,
		AssetServer: &assetserver.Options{
			Assets:     assets,
			Middleware: udsMiddleware(socketPath),
		},
		HideWindowOnClose: true,
		Linux:             &linux.Options{ProgramName: "spk-cockpit", Icon: appfiles.AppIcon},
		OnStartup:         app.onStartup,
		OnDomReady: func(ctx context.Context) {
			if hasPos {
				wruntime.WindowSetPosition(ctx, startX, startY)
			}
		},
		OnShutdown: func(ctx context.Context) {
			if saveGeometry == nil {
				return
			}
			x, y := wruntime.WindowGetPosition(ctx)
			w, h := wruntime.WindowGetSize(ctx)
			saveGeometry(Geometry{X: x, Y: y, Width: w, Height: h})
		},
	})
}

func udsMiddleware(socketPath string) assetserver.Middleware {
	transport := &http.Transport{
		DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
			return net.Dial("unix", socketPath)
		},
	}
	target, _ := url.Parse("http://unix")
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
		},
		Transport: transport,
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if len(r.URL.Path) >= 5 && r.URL.Path[:5] == "/api/" {
				proxy.ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Package popup spawns a small standalone window before each meeting,
// independent from the main spk-cockpit Wails window.
//
// Wails v2 doesn't support multi-window in-process, so the daemon spawns a
// short-lived child process (`cockpit popup --meeting-id=<id>`) which runs
// its own Wails window. Closing the window exits the child cleanly. This
// keeps the popup cross-platform: any OS that runs spk-cockpit's Wails build
// can render the popup the same way.
package popup

import (
	"log/slog"
	"os/exec"

	"github.com/spk/spk-cockpit/internal/api"
)

// Subprocess spawns `cockpit popup --meeting-id=<id> --socket=<sock>` for
// each meeting. The child process opens its own Wails window connected to
// the same UDS daemon and renders the meeting popup view.
type Subprocess struct {
	exe        string
	socketPath string
	logger     *slog.Logger
}

// NewSubprocess constructs a Subprocess backend.
//
// `exe` is typically the result of os.Executable(); `socketPath` is the daemon's
// UDS path (the child fetches meeting details over it). When either is empty
// the backend reports Available()==false.
func NewSubprocess(exe, socketPath string, logger *slog.Logger) *Subprocess {
	if logger == nil {
		logger = slog.Default()
	}
	return &Subprocess{exe: exe, socketPath: socketPath, logger: logger}
}

// Available reports whether the backend was given a valid binary path and socket.
func (s *Subprocess) Available() bool { return s.exe != "" && s.socketPath != "" }

// Show forks the popup subprocess in a goroutine. Failure is logged but does
// not propagate — the daemon's notification scheduler keeps running.
func (s *Subprocess) Show(m api.Meeting) {
	if !s.Available() {
		return
	}
	go func() {
		// #nosec G204 — args are flag-only; meeting ID is ULID-shaped from our DB.
		cmd := exec.Command(s.exe, "popup", //nolint:gosec
			"--meeting-id="+m.ID,
			"--socket="+s.socketPath,
		)
		if err := cmd.Run(); err != nil {
			s.logger.Warn("popup subprocess failed", "id", m.ID, "err", err)
		}
	}()
}

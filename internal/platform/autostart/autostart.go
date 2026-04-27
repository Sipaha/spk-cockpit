// Package autostart manages OS-specific user-level autostart for spk-cockpit.
// Linux: systemd-user unit at ~/.config/systemd/user/spk-cockpit.service.
package autostart

// Backend installs/removes a user-level autostart entry.
type Backend interface {
	// Install writes the autostart entry pointing at exePath and enables it.
	Install(exePath string) error
	// Uninstall disables and removes the autostart entry. Idempotent.
	Uninstall() error
	// Status reports whether autostart is currently installed and enabled.
	Status() (Status, error)
}

// Status is the current autostart state.
type Status struct {
	Installed bool
	Enabled   bool
	Detail    string // free-form, e.g. "active" or "disabled"
}

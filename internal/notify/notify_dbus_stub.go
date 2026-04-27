//go:build !linux

package notify

import "errors"

// NewDBus is not supported on non-Linux platforms and always returns an error.
func NewDBus() (*DBusNotifier, error) {
	return nil, errors.New("dbus not supported on this platform")
}

// DBusNotifier is a placeholder for non-Linux builds.
type DBusNotifier struct{}

// Notify is not implemented on non-Linux platforms.
func (d *DBusNotifier) Notify(_, _ string) error { return errors.New("dbus not supported") }

// Close is not implemented on non-Linux platforms.
func (d *DBusNotifier) Close() error { return nil }

// Package notify abstracts system notifications behind a Notifier interface.
// Linux uses DBus (org.freedesktop.Notifications); other platforms use no-op.
package notify

// Notifier sends a system notification.
type Notifier interface {
	// Notify shows a notification with title and body. Returns nil on success.
	// Implementations must NOT block; failures are logged and reported but never panic.
	Notify(title, body string) error

	// Close releases any resources (e.g. DBus connection).
	Close() error
}

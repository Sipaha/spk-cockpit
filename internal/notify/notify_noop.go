package notify

import "log/slog"

// NoopNotifier logs notifications instead of dispatching them. Used when DBus is unavailable.
type NoopNotifier struct {
	Logger *slog.Logger
}

// NewNoop returns a NoopNotifier.
func NewNoop(logger *slog.Logger) *NoopNotifier { return &NoopNotifier{Logger: logger} }

// Notify logs the event.
func (n *NoopNotifier) Notify(title, body string) error {
	if n.Logger != nil {
		n.Logger.Info("(noop notify)", "title", title, "body", body)
	}
	return nil
}

// Close is a no-op.
func (n *NoopNotifier) Close() error { return nil }

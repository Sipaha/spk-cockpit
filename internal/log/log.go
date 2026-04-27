// Package log provides a small slog factory used across spk-cockpit.
package log

import (
	"io"
	"log/slog"
	"os"
)

// New returns a text-format slog.Logger writing to out (defaults to os.Stderr) at the given level.
func New(out io.Writer, level slog.Level) *slog.Logger {
	if out == nil {
		out = os.Stderr
	}
	return slog.New(slog.NewTextHandler(out, &slog.HandlerOptions{
		Level: level,
	}))
}

// ParseLevel converts a string ("debug"|"info"|"warn"|"error") to slog.Level. Unknown → Info.
func ParseLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

//go:build !linux

package autostart

import "errors"

// Noop is a fallback Backend for non-Linux platforms.
type Noop struct{}

// NewNoop returns a Noop backend.
func NewNoop() *Noop { return &Noop{} }

// Install returns ErrUnsupported.
func (Noop) Install(_ string) error { return errors.New("autostart: only supported on Linux in v1") }

// Uninstall returns ErrUnsupported.
func (Noop) Uninstall() error { return errors.New("autostart: only supported on Linux in v1") }

// Status reports as not installed.
func (Noop) Status() (Status, error) { return Status{}, nil }

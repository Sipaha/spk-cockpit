// Package appfiles holds embedded application icons.
package appfiles

import _ "embed"

// TrayIcon is the bytes of icons/tray.png embedded at build time.
//
//go:embed tray.png
var TrayIcon []byte

// Package appfiles holds embedded application icons.
package appfiles

import _ "embed"

// TrayIcon is the bytes of icons/tray.png embedded at build time.
//
//go:embed tray.png
var TrayIcon []byte

// AppIcon is the bytes of icons/appicon.png — used by Wails as the window
// icon (minimized state and dock/taskbar representation).
//
//go:embed appicon.png
var AppIcon []byte

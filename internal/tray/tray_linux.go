//go:build linux

package tray

import (
	"fyne.io/systray"

	"github.com/spk/spk-cockpit/internal/appfiles"
)

type linuxTray struct {
	openWindow func()
	quit       func()
}

// New constructs a Linux tray backend. openWindow is invoked when the user
// chooses "Open window"; quit when they choose "Quit".
func New(openWindow, quit func()) Backend {
	return &linuxTray{openWindow: openWindow, quit: quit}
}

// Run starts the tray loop. onReady fires after icon/menu are visible.
func (t *linuxTray) Run(onReady func(), onExit func()) {
	systray.Run(func() {
		systray.SetIcon(appfiles.TrayIcon)
		systray.SetTooltip("spk-cockpit")

		open := systray.AddMenuItem("Open window", "")
		systray.AddSeparator()
		quit := systray.AddMenuItem("Quit", "")

		go func() {
			for {
				select {
				case <-open.ClickedCh:
					if t.openWindow != nil {
						t.openWindow()
					}
				case <-quit.ClickedCh:
					if t.quit != nil {
						t.quit()
					}
					systray.Quit()
					return
				}
			}
		}()

		if onReady != nil {
			onReady()
		}
	}, onExit)
}

// SetTooltip updates the tray tooltip text.
func (t *linuxTray) SetTooltip(s string) { systray.SetTooltip(s) }

// Quit terminates the tray loop.
func (t *linuxTray) Quit() { systray.Quit() }

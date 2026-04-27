//go:build linux

package notify

import (
	"errors"
	"fmt"

	"github.com/godbus/dbus/v5"
)

const (
	dbusObjectPath = "/org/freedesktop/Notifications"
	dbusInterface  = "org.freedesktop.Notifications"
	dbusName       = "org.freedesktop.Notifications"
)

// DBusNotifier sends notifications via libnotify's DBus interface.
type DBusNotifier struct {
	conn *dbus.Conn
}

// NewDBus connects to the session bus.
func NewDBus() (*DBusNotifier, error) {
	conn, err := dbus.SessionBus()
	if err != nil {
		return nil, fmt.Errorf("dbus session: %w", err)
	}
	return &DBusNotifier{conn: conn}, nil
}

// Notify sends a notification.
func (d *DBusNotifier) Notify(title, body string) error {
	if d == nil || d.conn == nil {
		return errors.New("notifier not initialized")
	}
	obj := d.conn.Object(dbusName, dbus.ObjectPath(dbusObjectPath))
	call := obj.Call(
		dbusInterface+".Notify",
		0,
		"spk-cockpit",
		uint32(0),
		"",
		title,
		body,
		[]string{},
		map[string]dbus.Variant{},
		int32(-1),
	)
	if call.Err != nil {
		return fmt.Errorf("notify call: %w", call.Err)
	}
	return nil
}

// Close closes the DBus connection.
func (d *DBusNotifier) Close() error {
	if d != nil && d.conn != nil {
		return d.conn.Close()
	}
	return nil
}

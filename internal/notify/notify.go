// Package notify shows desktop notifications (GDD 13.1), so that a player
// whose window is hidden still hears of raids and of their island's fate.
// On Linux it uses notify-send, the freedesktop notifier; elsewhere, or
// without it, notifications are silently dropped.
package notify

import (
	"os/exec"
	"sync"
)

// Notifier sends desktop notifications.
type Notifier struct {
	once sync.Once
	path string
	// run starts a command; tests replace it.
	run func(name string, args ...string) error
}

// New returns a notifier using notify-send when it is installed.
func New() *Notifier {
	return &Notifier{run: func(name string, args ...string) error {
		return exec.Command(name, args...).Start()
	}}
}

// Available reports whether notifications can be shown.
func (n *Notifier) Available() bool {
	n.once.Do(func() { n.path, _ = exec.LookPath("notify-send") })
	return n.path != ""
}

// Send shows a notification. It never blocks the game: the notifier runs as
// a separate process.
func (n *Notifier) Send(title, body string) error {
	if !n.Available() {
		return nil
	}
	return n.run(n.path, "--app-name=Apogée", "--expire-time=10000", title, body)
}

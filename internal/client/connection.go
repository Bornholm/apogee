package client

import (
	"fmt"
	"math"
	"time"
)

// When the connection to the sky drops, the game stays on screen: the island
// lives on from the last state heard, a banner says the connection is lost,
// and the app tries again, sooner first, then less and less often. The
// player can give up and go back to the home screen.

// reconnection follows the attempts to get back to the sky.
type reconnection struct {
	since    time.Time // when the connection dropped; zero while connected
	attempts int
	next     time.Time // when to try again
	trying   chan launchResult
}

// retryDelay is the wait after a number of failed attempts: 1, 2, 4… up to
// 30 seconds.
func retryDelay(attempts int) time.Duration {
	return time.Duration(math.Min(math.Pow(2, float64(attempts)), 30)) * time.Second
}

// watchConnection notices a dropped connection and tries to reconnect.
func (a *App) watchConnection() {
	now := time.Now()
	l := &a.lost
	if l.trying != nil {
		select {
		case r := <-l.trying:
			l.trying = nil
			if r.err == nil {
				a.conn = r.conn
				a.game.Resume(Listen(r.conn))
				a.lost = reconnection{}
				a.game.SetConnection("")
				return
			}
			l.attempts++
			l.next = now.Add(retryDelay(l.attempts))
		default:
		}
	}
	if l.since.IsZero() {
		if a.game.src.Err() == nil {
			return
		}
		l.since, l.next = now, now
		if a.conn != nil {
			a.conn.Close()
		}
	}
	if l.trying == nil && !now.Before(l.next) {
		target := a.target
		l.trying = make(chan launchResult, 1)
		ch := l.trying
		go func() { ch <- a.dial(target, nil) }()
	}
	msg := "Connexion au ciel perdue. Reconnexion en cours…"
	if l.trying == nil {
		msg = fmt.Sprintf("Connexion au ciel perdue. Nouvelle tentative dans %d s.",
			int(math.Ceil(l.next.Sub(now).Seconds())))
	}
	a.game.SetConnection(msg)
}

// giveUp stops trying and goes back to the home screen.
func (a *App) giveUp() {
	if ch := a.lost.trying; ch != nil {
		// An attempt still running may yet connect: close what it opens.
		go func() {
			if r := <-ch; r.conn != nil {
				r.conn.Close()
			}
		}()
	}
	a.lost = reconnection{}
	a.Close()
	a.game, a.conn = nil, nil
	a.screen = homeScreen
	a.status = "La connexion au ciel est perdue."
}

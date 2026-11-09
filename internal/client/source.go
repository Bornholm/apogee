package client

import (
	"sync"
	"time"

	"apogee/internal/netclient"
	"apogee/internal/proto"
	"apogee/internal/sim"
)

// Source keeps the latest state received from the server. The network runs in
// its own goroutine; the game loop only reads snapshots.
type Source struct {
	Sky string
	// send writes a message to the server.
	send func(proto.Msg) error

	mu    sync.Mutex
	state *proto.State
	at    time.Time
	err   error
}

// Listen receives states from conn until it fails, then records the error.
func Listen(conn *netclient.Conn) *Source {
	s := &Source{Sky: conn.Welcome.Sky, send: conn.Send}
	go func() {
		for {
			st, err := conn.Next()
			s.mu.Lock()
			if err != nil {
				s.err = err
				s.mu.Unlock()
				return
			}
			s.state, s.at = st, time.Now()
			s.mu.Unlock()
		}
	}()
	return s
}

// Islands returns the islands of the latest state, with their orbits advanced
// to now: states arrive about once a second, orbits must move smoothly.
func (s *Source) Islands(now time.Time) []proto.IslandSummary {
	s.mu.Lock()
	st, at := s.state, s.at
	s.mu.Unlock()
	if st == nil {
		return nil
	}
	dt := now.Sub(at).Seconds()
	out := make([]proto.IslandSummary, len(st.Islands))
	for i, is := range st.Islands {
		is.Angle += sim.AngularSpeed(is.Radius) * dt
		out[i] = is
	}
	return out
}

// Phase returns the time of day of the latest state and the time left before
// the next phase, counted down locally.
func (s *Source) Phase(now time.Time) (string, time.Duration, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == nil {
		return "", 0, false
	}
	left := time.Duration(s.state.PhaseLeftMs)*time.Millisecond - now.Sub(s.at)
	return s.state.Phase, max(left, 0), true
}

// State returns the latest state, or nil before the first one. The caller
// must not modify it.
func (s *Source) State() *proto.State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

// Me returns the player's island, 0 while their people wait for the dawn.
func (s *Source) Me() sim.IslandID {
	if st := s.State(); st != nil {
		return st.You
	}
	return 0
}

// Whisper sends a new pie of intentions (GDD 6.2).
func (s *Source) Whisper(p sim.Pie) error {
	return s.send(proto.Msg{Whisper: &proto.Whisper{Pie: p}})
}

// Answer replies to the pending supplique (GDD 6.5).
func (s *Source) Answer(id uint32, option int) error {
	return s.send(proto.Msg{Answer: &proto.Answer{ID: id, Option: option}})
}

// Err returns the error that ended the connection, if any.
func (s *Source) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

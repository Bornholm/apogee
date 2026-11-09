// Package server runs the authoritative sky (GDD 10.3): it steps the
// simulation at a fixed rate and broadcasts snapshots to connected clients.
// The same package backs the dedicated `apogee-server` and the server embedded
// in the desktop client.
package server

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"log/slog"
	"math"
	"math/rand/v2"
	"net"
	"sync"
	"time"

	"apogee/internal/proto"
	"apogee/internal/sim"
)

// Config tunes a server.
type Config struct {
	// Name is the sky's name, shown to players when they discover it.
	Name string
	// Broadcast is the interval between two state snapshots.
	Broadcast time.Duration
	// HandshakeTimeout bounds the wait for a client's Hello.
	HandshakeTimeout time.Duration
	// DemoRivals adds that many placeholder islands at various distances
	// from the portal, until the rival AIs exist (GDD section 9).
	DemoRivals int
	// Seed drives the placement of the demo rivals.
	Seed uint64
	// MaxIslands caps the population of the sky, humans and AIs together
	// (GDD 9.5). Unknown player keys are refused once it is reached.
	MaxIslands int
	Logger     *slog.Logger
}

func (c *Config) defaults() {
	if c.Name == "" {
		c.Name = "Apogée"
	}
	if c.Broadcast <= 0 {
		c.Broadcast = time.Second
	}
	if c.HandshakeTimeout <= 0 {
		c.HandshakeTimeout = 5 * time.Second
	}
	if c.Logger == nil {
		c.Logger = slog.Default()
	}
	if c.MaxIslands <= 0 {
		c.MaxIslands = 10
	}
}

// Server owns the sky and its client connections.
type Server struct {
	cfg Config

	mu      sync.Mutex
	sky     sim.Sky
	keys    map[string]sim.IslandID // player key → island
	clients map[*client]struct{}
}

// New returns a server with an empty sky.
func New(cfg Config) *Server {
	cfg.defaults()
	s := &Server{cfg: cfg, keys: map[string]sim.IslandID{}, clients: map[*client]struct{}{}}
	r := rand.New(rand.NewPCG(cfg.Seed, 1))
	for i := range cfg.DemoRivals {
		radius := sim.RimRadius * (0.4 + 0.6*r.Float64())
		s.sky.AddIslandAt(fmt.Sprintf("Rival %d", i+1), r.Uint64(), radius, r.Float64()*2*math.Pi)
	}
	return s
}

// client is one connection. Messages to it go through a buffered queue drained
// by its own goroutine, so a slow client never blocks the simulation.
type client struct {
	conn net.Conn
	out  chan proto.Msg
	dead bool // dropped for being too slow; guarded by Server.mu
}

// Run steps the simulation and broadcasts snapshots until ctx is done.
func (s *Server) Run(ctx context.Context) {
	step := time.NewTicker(sim.TickDuration)
	defer step.Stop()
	bcast := time.NewTicker(s.cfg.Broadcast)
	defer bcast.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-step.C:
			s.mu.Lock()
			s.sky.Step()
			s.mu.Unlock()
		case <-bcast.C:
			s.broadcast()
		}
	}
}

// Serve accepts connections on ln until ctx is done or ln fails.
func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	go func() {
		<-ctx.Done()
		ln.Close()
	}()
	var wg sync.WaitGroup
	defer wg.Wait()
	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.handle(ctx, conn)
		}()
	}
}

func (s *Server) handle(ctx context.Context, conn net.Conn) {
	log := s.cfg.Logger.With("remote", conn.RemoteAddr().String())
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()

	dec := proto.NewDecoder(conn)
	enc := proto.NewEncoder(conn)
	conn.SetReadDeadline(time.Now().Add(s.cfg.HandshakeTimeout))
	m, err := dec.Decode()
	if err != nil || m.Hello == nil {
		log.Info("handshake failed", "err", err)
		return
	}
	if m.Hello.Version != proto.Version {
		enc.Encode(proto.Msg{Error: &proto.Error{Message: "incompatible version"}})
		return
	}
	if m.Hello.Key == "" {
		enc.Encode(proto.Msg{Error: &proto.Error{Message: "missing player key"}})
		return
	}
	conn.SetReadDeadline(time.Time{})

	id, ok := s.join(m.Hello)
	if !ok {
		log.Info("sky full, player refused", "name", m.Hello.Name)
		enc.Encode(proto.Msg{Error: &proto.Error{Message: "the sky is full"}})
		return
	}
	c := &client{conn: conn, out: make(chan proto.Msg, 16)}
	c.out <- proto.Msg{Welcome: &proto.Welcome{Version: proto.Version, Island: id, Sky: s.cfg.Name}}
	c.out <- proto.Msg{State: s.snapshot()}

	s.mu.Lock()
	s.clients[c] = struct{}{}
	s.mu.Unlock()
	log.Info("player joined", "name", m.Hello.Name, "island", id)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for msg := range c.out {
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := enc.Encode(msg); err != nil {
				conn.Close()
				for range c.out { // drain until the queue is closed
				}
				return
			}
		}
	}()

	// Clients will send intentions here; for now, reading only detects the
	// end of the connection.
	for {
		if _, err := dec.Decode(); err != nil {
			if !errors.Is(err, io.EOF) && ctx.Err() == nil {
				log.Info("connection lost", "err", err)
			}
			break
		}
	}

	s.mu.Lock()
	delete(s.clients, c)
	close(c.out)
	s.mu.Unlock()
	<-done
	log.Info("player left", "island", id)
}

// join returns the island of the player's key, founding one if needed. The
// island lives on whether the player is connected or not (GDD pilier 3). It
// reports false when a new island would exceed MaxIslands: keys are not
// authenticated on a LAN, so the cap is what bounds the state a peer can make
// the server broadcast.
func (s *Server) join(h *proto.Hello) (sim.IslandID, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id, ok := s.keys[h.Key]; ok {
		return id, true
	}
	if len(s.sky.Islands) >= s.cfg.MaxIslands {
		return 0, false
	}
	hs := fnv.New64a()
	hs.Write([]byte(h.Key))
	seed := hs.Sum64()
	// Spread newcomers around the rim with the golden angle.
	angle := float64(len(s.sky.Islands)) * math.Pi * (3 - math.Sqrt(5))
	id := s.sky.AddIsland(h.Name, seed, angle)
	s.keys[h.Key] = id
	return id, true
}

func (s *Server) snapshot() *proto.State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.snapshotLocked()
}

func (s *Server) snapshotLocked() *proto.State {
	phase, left := s.sky.TimeOfDay()
	return &proto.State{
		Tick:        s.sky.Tick,
		Phase:       phase.String(),
		PhaseLeftMs: left.Milliseconds(),
		Islands:     append([]sim.Island(nil), s.sky.Islands...),
	}
}

// broadcast queues a snapshot for every client. A client whose queue is full
// is too slow to follow: its connection is closed rather than blocking others.
func (s *Server) broadcast() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.clients) == 0 {
		return
	}
	st := s.snapshotLocked()
	for c := range s.clients {
		if c.dead {
			continue // its handler will remove it once its read loop ends
		}
		select {
		case c.out <- proto.Msg{State: st}:
		default:
			c.dead = true
			c.conn.Close()
		}
	}
}

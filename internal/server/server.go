// Package server runs the authoritative sky (GDD 10.3): it steps the
// simulation at a fixed rate, applies the gods' whispers and broadcasts
// snapshots to connected clients. The same package backs the dedicated
// `apogee-server` and the server embedded in the desktop client.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"log/slog"
	"math"
	"math/rand/v2"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

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
	// Rivals is the number of rival AI gods in a new sky (GDD section 9).
	Rivals int
	// Seed drives a new sky: its rivals, its waves.
	Seed uint64
	// MaxIslands caps the population of the sky, humans and AIs together
	// (GDD 9.5). Unknown player keys are refused once it is reached.
	MaxIslands int
	// Speed runs that many simulation steps per tick, to watch days go by
	// in minutes while testing. 1 is real time.
	Speed int
	// DataFile is where the sky is saved and restored (GDD 11.1). Empty
	// keeps the sky in memory only.
	DataFile string
	// SaveEvery is the interval between two saves.
	SaveEvery time.Duration
	Logger    *slog.Logger
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
	if c.Speed <= 0 {
		c.Speed = 1
	}
	if c.SaveEvery <= 0 {
		c.SaveEvery = time.Minute
	}
}

// Server owns the sky and its client connections.
type Server struct {
	cfg Config

	mu      sync.Mutex
	sky     sim.Sky
	clients map[*client]struct{}
}

// client is one connection. Messages to it go through a buffered queue drained
// by its own goroutine, so a slow client never blocks the simulation.
type client struct {
	conn net.Conn
	out  chan proto.Msg
	key  string
	dead bool // dropped for being too slow; guarded by Server.mu
	// look is the island the player looks at; guarded by Server.mu.
	look sim.IslandID
}

// rivalNames name the rival AI gods.
var rivalNames = []string{"Morgal", "Aubépine", "Ysolde", "Corvin", "Thessa", "Ferrand", "Liriel", "Oskar", "Brume"}

// New returns a server, restoring the sky from cfg.DataFile when it exists.
func New(cfg Config) (*Server, error) {
	cfg.defaults()
	s := &Server{cfg: cfg, clients: map[*client]struct{}{}}
	if cfg.DataFile != "" {
		data, err := os.ReadFile(cfg.DataFile)
		switch {
		case err == nil:
			if err := json.Unmarshal(data, &s.sky); err != nil {
				return nil, fmt.Errorf("restoring %s: %w", cfg.DataFile, err)
			}
			cfg.Logger.Info("sky restored", "file", cfg.DataFile, "islands", len(s.sky.Islands))
			return s, nil
		case !errors.Is(err, os.ErrNotExist):
			return nil, err
		}
	}
	s.sky.Seed = cfg.Seed
	r := rand.New(rand.NewPCG(cfg.Seed, 1))
	for i := range min(cfg.Rivals, cfg.MaxIslands-1) {
		name := rivalNames[i%len(rivalNames)]
		id := s.sky.FoundRival(name, sim.Profile(i%int(sim.NumProfiles)), r.Uint64(), r.Float64()*2*math.Pi)
		// Rivals of a new sky are at various stages of their descent, so the
		// sky looks lived in from the start.
		s.sky.Island(id).Radius = sim.RimRadius * (0.5 + 0.5*r.Float64())
	}
	return s, nil
}

// Run steps the simulation, broadcasts snapshots and saves the sky until ctx
// is done; it saves one last time before returning.
func (s *Server) Run(ctx context.Context) {
	step := time.NewTicker(sim.TickDuration)
	defer step.Stop()
	bcast := time.NewTicker(s.cfg.Broadcast)
	defer bcast.Stop()
	save := time.NewTicker(s.cfg.SaveEvery)
	defer save.Stop()
	for {
		select {
		case <-ctx.Done():
			s.save()
			return
		case <-step.C:
			s.mu.Lock()
			for range s.cfg.Speed {
				s.sky.Step()
			}
			s.mu.Unlock()
		case <-bcast.C:
			s.broadcast()
		case <-save.C:
			s.save()
		}
	}
}

// save writes the sky to cfg.DataFile, atomically: a crash mid-write leaves
// the previous save intact.
func (s *Server) save() {
	if s.cfg.DataFile == "" {
		return
	}
	s.mu.Lock()
	data, err := json.Marshal(&s.sky)
	s.mu.Unlock()
	if err == nil {
		err = writeAtomic(s.cfg.DataFile, data)
	}
	if err != nil {
		s.cfg.Logger.Error("saving the sky", "err", err)
	}
}

func writeAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".sky-*.json")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), path)
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

// validKey accepts the keys clients generate: short, plain, and never in the
// namespace of the rival AIs.
func validKey(k string) bool {
	if k == "" || len(k) > 64 || strings.HasPrefix(k, sim.AIPrefix) {
		return false
	}
	for _, r := range k {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r == '-' || r == '_') {
			return false
		}
	}
	return true
}

// cleanName keeps a player name short and printable.
func cleanName(n string) string {
	n = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, strings.TrimSpace(n))
	if utf8.RuneCountInString(n) > 24 {
		n = string([]rune(n)[:24])
	}
	if n == "" {
		n = "Dieu sans nom"
	}
	return n
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
	if !validKey(m.Hello.Key) {
		enc.Encode(proto.Msg{Error: &proto.Error{Message: "invalid player key"}})
		return
	}
	conn.SetReadDeadline(time.Time{})

	key, name := m.Hello.Key, cleanName(m.Hello.Name)
	id, ok := s.join(key, name)
	if !ok {
		log.Info("sky full, player refused", "name", name)
		enc.Encode(proto.Msg{Error: &proto.Error{Message: "the sky is full"}})
		return
	}
	c := &client{conn: conn, out: make(chan proto.Msg, 16), key: key}
	c.out <- proto.Msg{Welcome: &proto.Welcome{Version: proto.Version, Island: id, Sky: s.cfg.Name}}
	s.mu.Lock()
	c.out <- proto.Msg{State: s.stateLocked(s.baseLocked(), c)}
	s.clients[c] = struct{}{}
	s.mu.Unlock()
	log.Info("player joined", "name", name, "island", id)

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

	for {
		m, err := dec.Decode()
		if err != nil {
			if !errors.Is(err, io.EOF) && ctx.Err() == nil {
				log.Info("connection lost", "err", err)
			}
			break
		}
		if m.Look != nil {
			s.mu.Lock()
			c.look = m.Look.Island
			s.mu.Unlock()
			continue
		}
		if err := s.apply(key, m); err != nil {
			log.Info("refused a god's act", "err", err)
			// Tell the god why, without closing the connection.
			s.mu.Lock()
			if !c.dead {
				select {
				case c.out <- proto.Msg{Error: &proto.Error{Message: err.Error()}}:
				default:
				}
			}
			s.mu.Unlock()
		}
	}

	s.mu.Lock()
	delete(s.clients, c)
	close(c.out)
	s.mu.Unlock()
	<-done
	log.Info("player left", "island", id)
}

// apply carries out what a god sends: a whisper or an answer. Nothing else
// can act on the sky (GDD pilier 4).
func (s *Server) apply(key string, m proto.Msg) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, ok := s.sky.IslandOf(key)
	if !ok {
		return errors.New("no living island")
	}
	is := s.sky.Island(id)
	switch {
	case m.Whisper != nil:
		return is.Civ.SetPie(m.Whisper.Pie)
	case m.Answer != nil:
		return is.Civ.Answer(m.Answer.ID, m.Answer.Option, s.sky.Tick)
	case m.Aim != nil:
		if m.Aim.Kind < 0 || m.Aim.Kind >= sim.NumOuter {
			return errors.New("unknown outer intention")
		}
		return s.sky.Whisper(id, m.Aim.Kind, m.Aim.Target)
	}
	return nil
}

// join returns the island of the player's key, founding one if needed. The
// island lives on whether the player is connected or not (GDD pilier 3). The
// island is 0 while a fallen people waits for the dawn. It reports false when
// a new island would exceed MaxIslands: keys are not authenticated on a LAN,
// so the cap is what bounds the state a peer can make the server broadcast.
func (s *Server) join(key, name string) (sim.IslandID, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id, ok := s.sky.IslandOf(key); ok {
		return id, true
	}
	if s.sky.AwaitingRebirth(key) {
		return 0, true
	}
	if len(s.sky.Islands)+s.sky.Rebirths() >= s.cfg.MaxIslands {
		return 0, false
	}
	h := fnv.New64a()
	h.Write([]byte(key))
	// Spread newcomers around the rim with the golden angle.
	angle := float64(s.sky.NextID) * math.Pi * (3 - math.Sqrt(5))
	return s.sky.Found(key, name, h.Sum64(), angle), true
}

// baseLocked builds the part of a state every player shares.
func (s *Server) baseLocked() proto.State {
	phase, left := s.sky.TimeOfDay()
	st := proto.State{
		Tick:        s.sky.Tick,
		Phase:       phase.String(),
		PhaseLeftMs: left.Milliseconds(),
		Islands:     make([]proto.IslandSummary, len(s.sky.Islands)),
		Annals:      proto.Annals(&s.sky, 10),
		BossInMs:    -1,
	}
	for i := range s.sky.Islands {
		st.Islands[i] = proto.Summarize(&s.sky, &s.sky.Islands[i])
	}
	st.Raids = proto.Raids(&s.sky)
	if b := s.sky.Boss; b != nil {
		st.BossInMs = int64(b.In * 1000)
	}
	st.WaveInMs = -1
	if w := s.sky.Wave; w != nil {
		st.WaveTarget, st.WaveInMs, st.WaveSpecies = w.Target, int64(w.In*1000), w.Species
	}
	return st
}

// stateLocked completes the shared state with the player's own island and
// the one they look at.
func (s *Server) stateLocked(base proto.State, c *client) *proto.State {
	st := base
	if id, ok := s.sky.IslandOf(c.key); ok {
		st.You = id
		st.Own = proto.Detail(&s.sky, s.sky.Island(id))
	}
	if c.look != 0 && c.look != st.You {
		if is := s.sky.Island(c.look); is != nil {
			st.Seen = proto.See(is)
		}
	}
	return &st
}

// broadcast queues a snapshot for every client. A client whose queue is full
// is too slow to follow: its connection is closed rather than blocking others.
func (s *Server) broadcast() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.clients) == 0 {
		return
	}
	base := s.baseLocked()
	for c := range s.clients {
		if c.dead {
			continue // its handler will remove it once its read loop ends
		}
		select {
		case c.out <- proto.Msg{State: s.stateLocked(base, c)}:
		default:
			c.dead = true
			c.conn.Close()
		}
	}
}

package server

import (
	"context"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"apogee/internal/netclient"
	"apogee/internal/proto"
	"apogee/internal/sim"
)

func start(t *testing.T) (string, context.CancelFunc) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	srv, err := New(Config{
		Name:      "test",
		Broadcast: 50 * time.Millisecond,
		Logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	go srv.Run(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		srv.Serve(ctx, ln)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})
	return ln.Addr().String(), cancel
}

func TestPlayerGetsAnIslandAndStates(t *testing.T) {
	addr, _ := start(t)
	ctx := context.Background()
	c, err := netclient.Dial(ctx, addr, "Ada", "key-ada")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if c.Welcome.Island == 0 || c.Welcome.Sky != "test" {
		t.Fatalf("unexpected welcome %+v", c.Welcome)
	}
	first, err := c.Next()
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Islands) != 1 || first.Islands[0].Name != "Ada" {
		t.Fatalf("unexpected islands %+v", first.Islands)
	}
	var later *proto.State
	for range 5 {
		if later, err = c.Next(); err != nil {
			t.Fatal(err)
		}
	}
	if later.Tick <= first.Tick {
		t.Errorf("the sky did not advance: tick %d then %d", first.Tick, later.Tick)
	}
}

func TestSameKeyFindsSameIsland(t *testing.T) {
	addr, _ := start(t)
	ctx := context.Background()
	a, err := netclient.Dial(ctx, addr, "Ada", "k1")
	if err != nil {
		t.Fatal(err)
	}
	a.Close()
	b, err := netclient.Dial(ctx, addr, "Bob", "k2")
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	again, err := netclient.Dial(ctx, addr, "Ada", "k1")
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()
	if again.Welcome.Island != a.Welcome.Island {
		t.Errorf("returning player got island %d, want %d", again.Welcome.Island, a.Welcome.Island)
	}
	if b.Welcome.Island == a.Welcome.Island {
		t.Error("two players share an island")
	}
	st, err := again.Next()
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Islands) != 2 {
		t.Errorf("islands: %d, want 2 (an absent player's island lives on)", len(st.Islands))
	}
}

func TestRefusesWrongVersionAndMissingKey(t *testing.T) {
	addr, _ := start(t)
	for name, hello := range map[string]proto.Hello{
		"version": {Version: proto.Version + 1, Name: "x", Key: "k"},
		"key":     {Version: proto.Version, Name: "x"},
	} {
		c, err := net.Dial("tcp", addr)
		if err != nil {
			t.Fatal(err)
		}
		proto.NewEncoder(c).Encode(proto.Msg{Hello: &hello})
		m, err := proto.NewDecoder(c).Decode()
		c.Close()
		if err != nil || m.Error == nil {
			t.Errorf("%s: want an error message, got %+v, %v", name, m, err)
		}
	}
}

func TestSilentClientIsDropped(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv, _ := New(Config{HandshakeTimeout: 100 * time.Millisecond, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	go srv.Serve(ctx, ln)
	c, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := c.Read(make([]byte, 1)); err == nil {
		t.Fatal("the server kept a client that never said hello")
	}
}

func TestFullSkyRefusesNewcomersButNotReturningPlayers(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv, _ := New(Config{MaxIslands: 2, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	go srv.Serve(ctx, ln)
	addr := ln.Addr().String()

	for _, k := range []string{"a", "b"} {
		c, err := netclient.Dial(ctx, addr, k, k)
		if err != nil {
			t.Fatalf("player %s refused: %v", k, err)
		}
		c.Close()
	}
	if c, err := netclient.Dial(ctx, addr, "c", "c"); err == nil {
		c.Close()
		t.Fatal("a third island was founded in a sky capped at two")
	}
	c, err := netclient.Dial(ctx, addr, "a", "a")
	if err != nil {
		t.Fatalf("a returning player was refused: %v", err)
	}
	c.Close()
}

func TestWhisperChangesThePie(t *testing.T) {
	addr, _ := start(t)
	ctx := context.Background()
	c, err := netclient.Dial(ctx, addr, "Ada", "whisperer")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	st, err := c.Next()
	if err != nil {
		t.Fatal(err)
	}
	if st.Own == nil || st.You != c.Welcome.Island {
		t.Fatalf("the player does not see their own island: you %d, own %v", st.You, st.Own)
	}
	want := sim.Pie{sim.Study: 60, sim.Fortify: 40}
	if err := c.Send(proto.Msg{Whisper: &proto.Whisper{Pie: want}}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		st, err = c.Next()
		if err != nil {
			t.Fatal(err)
		}
		if st.Own.Pie == want {
			return
		}
	}
	t.Fatalf("pie still %v after the whisper", st.Own.Pie)
}

func TestInvalidWhisperIsIgnored(t *testing.T) {
	addr, _ := start(t)
	c, err := netclient.Dial(context.Background(), addr, "Ada", "bad-whisper")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.Send(proto.Msg{Whisper: &proto.Whisper{Pie: sim.Pie{sim.Study: 50}}}) // not 100 %
	for range 3 {
		st, err := c.Next()
		if err != nil {
			t.Fatalf("the server dropped the player for an invalid whisper: %v", err)
		}
		if st.Own.Pie != sim.DefaultPie {
			t.Fatalf("an invalid pie was applied: %v", st.Own.Pie)
		}
	}
}

func TestKeysOfTheAIsAreRefused(t *testing.T) {
	addr, _ := start(t)
	if c, err := netclient.Dial(context.Background(), addr, "x", sim.AIPrefix+"Morgal"); err == nil {
		c.Close()
		t.Fatal("a client took the key of a rival AI")
	}
}

func TestTheSkyIsSavedAndRestored(t *testing.T) {
	file := filepath.Join(t.TempDir(), "sky.json")
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv, err := New(Config{DataFile: file, Rivals: 3, Seed: 9, Logger: quiet})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		srv.Run(ctx)
	}()
	time.Sleep(300 * time.Millisecond)
	cancel()
	<-done
	srv.mu.Lock()
	tick, n := srv.sky.Tick, len(srv.sky.Islands)
	srv.mu.Unlock()

	again, err := New(Config{DataFile: file, Rivals: 7, Seed: 1, Logger: quiet})
	if err != nil {
		t.Fatal(err)
	}
	if again.sky.Tick != tick || len(again.sky.Islands) != n || n != 3 {
		t.Fatalf("restored tick %d with %d islands, saved tick %d with %d", again.sky.Tick, len(again.sky.Islands), tick, n)
	}
	if !again.sky.Island(again.sky.Islands[0].ID).IsAI() {
		t.Error("restored rivals are no longer AIs")
	}
}

func TestACorruptSaveIsAnError(t *testing.T) {
	file := filepath.Join(t.TempDir(), "sky.json")
	os.WriteFile(file, []byte("{not json"), 0o600)
	if _, err := New(Config{DataFile: file}); err == nil {
		t.Fatal("a corrupt save was silently replaced by a new sky")
	}
}

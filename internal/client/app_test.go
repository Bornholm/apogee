package client

import (
	"net"
	"testing"
	"time"

	"apogee/internal/server"
)

// The last sky joined over the network is remembered, to come back to it
// from the home screen.
func TestTheLastSkyJoinedIsRemembered(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	a := NewApp(AppOptions{Seed: 1, Rivals: 1, Speed: 1})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	stop, err := a.embedded(server.Config{Name: "Ciel de test"}, ln)
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	addr := ln.Addr().String()
	a.Start(addr)
	deadline := time.Now().Add(10 * time.Second)
	for a.screen != playingScreen {
		if time.Now().After(deadline) || a.failed != nil {
			t.Fatalf("never joined the sky: %v", a.failed)
		}
		a.Update()
		time.Sleep(10 * time.Millisecond)
	}
	a.Close()
	s := LoadSettings()
	if s.LastServer != addr || s.LastSky != "Ciel de test" {
		t.Fatalf("remembered %q (%q), want %q (Ciel de test)", s.LastServer, s.LastSky, addr)
	}
	if b := NewApp(AppOptions{}); b.address.value != addr {
		t.Fatalf("the address field shows %q, want the last sky %q", b.address.value, addr)
	}
}

// When the sky drops the connection, the game stays, says so, and gets
// back to the sky once it answers again.
func TestTheGameReconnectsToALostSky(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	data := t.TempDir() + "/sky.json"
	a := NewApp(AppOptions{Seed: 1, Rivals: 1, Speed: 1})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	stop, err := a.embedded(server.Config{Name: "Ciel de test", DataFile: data}, ln)
	if err != nil {
		t.Fatal(err)
	}
	a.Start(addr)
	wait := func(what string, ok func() bool) {
		t.Helper()
		deadline := time.Now().Add(15 * time.Second)
		for !ok() {
			if time.Now().After(deadline) {
				t.Fatalf("timed out waiting for %s", what)
			}
			if a.screen == playingScreen {
				a.watchConnection()
			} else {
				a.Update()
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	wait("the sky", func() bool { return a.screen == playingScreen && a.game.src.State() != nil })
	island := a.game.src.State().You

	stop() // the server goes down
	wait("the banner", func() bool { return a.game.connection != "" })
	if a.screen != playingScreen {
		t.Fatal("the game left the screen when the connection dropped")
	}

	ln, err = net.Listen("tcp", addr) // and comes back, at the same address
	if err != nil {
		t.Fatal(err)
	}
	stop, err = a.embedded(server.Config{Name: "Ciel de test", DataFile: data}, ln)
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	wait("the reconnection", func() bool { return a.game.connection == "" })
	wait("a new state", func() bool { return a.game.src.State() != nil })
	if you := a.game.src.State().You; you != island {
		t.Fatalf("back on island %d, want %d", you, island)
	}
	a.Close()
}

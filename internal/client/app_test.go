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

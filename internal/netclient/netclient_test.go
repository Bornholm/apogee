package netclient

import (
	"context"
	"net"
	"testing"
	"time"

	"apogee/internal/proto"
)

func TestSilentServerEndsTheSession(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		proto.NewDecoder(c).Decode()
		proto.NewEncoder(c).Encode(proto.Msg{Welcome: &proto.Welcome{Version: proto.Version, Island: 1}})
		time.Sleep(5 * time.Second) // then nothing: a frozen server
	}()

	conn, err := Dial(context.Background(), ln.Addr().String(), "a", "k")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.ReadTimeout = 100 * time.Millisecond
	start := time.Now()
	if _, err := conn.Next(); err == nil {
		t.Fatal("Next returned a state from a silent server")
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("Next took %v to give up, want about the read timeout", d)
	}
}

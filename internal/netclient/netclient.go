// Package netclient connects a client to a server and receives the sky.
package netclient

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"apogee/internal/proto"
)

// Conn is an established session with a server.
type Conn struct {
	Welcome proto.Welcome
	conn    net.Conn
	dec     *proto.Decoder
}

// Dial connects to addr and performs the handshake.
func Dial(ctx context.Context, addr, name, key string) (*Conn, error) {
	// Bound the TCP connection like the handshake: a wrong address or a stale
	// mDNS record must not hang for the OS timeout.
	d := net.Dialer{Timeout: 10 * time.Second}
	c, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	enc, dec := proto.NewEncoder(c), proto.NewDecoder(c)
	c.SetDeadline(time.Now().Add(10 * time.Second))
	if err := enc.Encode(proto.Msg{Hello: &proto.Hello{Version: proto.Version, Name: name, Key: key}}); err != nil {
		c.Close()
		return nil, err
	}
	m, err := dec.Decode()
	if err != nil {
		c.Close()
		return nil, err
	}
	c.SetDeadline(time.Time{})
	switch {
	case m.Error != nil:
		c.Close()
		return nil, fmt.Errorf("server refused: %s", m.Error.Message)
	case m.Welcome == nil:
		c.Close()
		return nil, errors.New("server did not welcome us")
	}
	return &Conn{Welcome: *m.Welcome, conn: c, dec: dec}, nil
}

// Next blocks until the next state snapshot.
func (c *Conn) Next() (*proto.State, error) {
	for {
		m, err := c.dec.Decode()
		if err != nil {
			return nil, err
		}
		if m.State != nil {
			return m.State, nil
		}
	}
}

// Close ends the session. The player's island lives on (GDD pilier 3).
func (c *Conn) Close() error { return c.conn.Close() }

// PlayerKey returns the key identifying this player, generating and storing it
// in the user's configuration directory on first use (GDD 10.6): no account,
// no personal data.
func PlayerKey() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, "apogee", "player.key")
	if b, err := os.ReadFile(path); err == nil {
		if k := strings.TrimSpace(string(b)); k != "" {
			return k, nil
		}
	}
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	key := hex.EncodeToString(buf)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(key+"\n"), 0o600); err != nil {
		return "", err
	}
	return key, nil
}

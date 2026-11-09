// Package proto defines the messages exchanged between the server and its
// clients (GDD 10.5): intentions, seeds and states, never assets. Messages are
// JSON objects, one per line, which keeps the wire easy to inspect.
package proto

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"

	"apogee/internal/sim"
)

// Version is bumped on every incompatible change of the messages.
const Version = 1

// DefaultPort is the single TCP port of a server (GDD 10.6).
const DefaultPort = 47110

// ServiceType is the mDNS service clients browse for (GDD 10.1).
const ServiceType = "_apogee._tcp"

// MaxLine bounds the size of one message, so that a peer cannot make the other
// side buffer without limit.
const MaxLine = 1 << 20

// Msg is the envelope of every message. Exactly one payload is set.
type Msg struct {
	Hello   *Hello   `json:"hello,omitempty"`
	Welcome *Welcome `json:"welcome,omitempty"`
	State   *State   `json:"state,omitempty"`
	Error   *Error   `json:"error,omitempty"`
}

// Hello is the first message of a client. Key is generated locally by the
// client and lets the player find their island again (GDD 10.6).
type Hello struct {
	Version int    `json:"version"`
	Name    string `json:"name"`
	Key     string `json:"key"`
}

// Welcome answers a valid Hello with the player's island.
type Welcome struct {
	Version int          `json:"version"`
	Island  sim.IslandID `json:"island"`
	Sky     string       `json:"sky"`
}

// State is a snapshot of the sky, sent periodically.
type State struct {
	Tick        uint64       `json:"tick"`
	Phase       string       `json:"phase"`
	PhaseLeftMs int64        `json:"phaseLeftMs"`
	Islands     []sim.Island `json:"islands"`
}

// Error is sent before the server closes a connection it refuses.
type Error struct {
	Message string `json:"message"`
}

// Encoder writes messages, one JSON object per line.
type Encoder struct{ enc *json.Encoder }

// NewEncoder returns an encoder writing to w.
func NewEncoder(w io.Writer) *Encoder { return &Encoder{json.NewEncoder(w)} }

// Encode writes one message.
func (e *Encoder) Encode(m Msg) error { return e.enc.Encode(m) }

// Decoder reads messages written by an Encoder.
type Decoder struct{ sc *bufio.Scanner }

// NewDecoder returns a decoder reading from r.
func NewDecoder(r io.Reader) *Decoder {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), MaxLine)
	return &Decoder{sc}
}

// ErrTooLong is returned for a message larger than MaxLine.
var ErrTooLong = bufio.ErrTooLong

// Decode reads the next message. It returns io.EOF at the end of the stream.
func (d *Decoder) Decode() (Msg, error) {
	if !d.sc.Scan() {
		if err := d.sc.Err(); err != nil {
			return Msg{}, err
		}
		return Msg{}, io.EOF
	}
	var m Msg
	if err := json.Unmarshal(d.sc.Bytes(), &m); err != nil {
		return Msg{}, errors.Join(errors.New("proto: malformed message"), err)
	}
	return m, nil
}

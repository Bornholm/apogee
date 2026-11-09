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
const Version = 4

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
	// Whisper and Answer go from a client to the server: the god's only ways
	// to act (GDD pilier 4).
	Whisper *Whisper `json:"whisper,omitempty"`
	Answer  *Answer  `json:"answer,omitempty"`
	Aim     *Aim     `json:"aim,omitempty"`
	Look    *Look    `json:"look,omitempty"`
}

// Look asks to see another island up close (GDD pilier 2: every island is
// visible and zoomable). Island 0 stops looking.
type Look struct {
	Island IslandID `json:"island"`
}

// Seen is what anyone may see of an island up close: its buildings and
// people, never its stores, its pie nor its chronicle.
type Seen struct {
	Island     IslandID       `json:"island"`
	Buildings  []sim.Building `json:"buildings"`
	Features   []sim.Feature  `json:"features"`
	Population float64        `json:"population"`
	Defenders  float64        `json:"defenders"`
}

// See copies the public view of an island. It must be called with the sky
// locked.
func See(is *sim.Island) *Seen {
	c := &is.Civ
	return &Seen{
		Island:     is.ID,
		Buildings:  append([]sim.Building(nil), c.Buildings...),
		Features:   append([]sim.Feature(nil), c.Features...),
		Population: c.Population, Defenders: c.Defenders,
	}
}

// Aim whispers an intention turned towards another island (GDD 6.3).
// Kind 0 is Rappeler: every unit comes home. The server answers a refused aim
// (a protected island, for instance) with an Error, without closing the
// connection.
type Aim struct {
	Kind   sim.Outer `json:"kind"`
	Target IslandID  `json:"target"`
}

// Whisper sets the pie of intentions of the player's island (GDD 6.2).
type Whisper struct {
	Pie sim.Pie `json:"pie"`
}

// Answer replies to the pending supplique of the player's island (GDD 6.5).
type Answer struct {
	ID     uint32 `json:"id"`
	Option int    `json:"option"`
}

// Hello is the first message of a client. Key is generated locally by the
// client and lets the player find their island again (GDD 10.6).
type Hello struct {
	Version int    `json:"version"`
	Name    string `json:"name"`
	Key     string `json:"key"`
}

// Welcome answers a valid Hello with the player's island, which is 0 while
// the player's people wait for the dawn to be reborn.
type Welcome struct {
	Version int          `json:"version"`
	Island  sim.IslandID `json:"island"`
	Sky     string       `json:"sky"`
}

// State is a snapshot of the sky, sent periodically.
type State struct {
	Tick        uint64          `json:"tick"`
	Phase       string          `json:"phase"`
	PhaseLeftMs int64           `json:"phaseLeftMs"`
	Islands     []IslandSummary `json:"islands"`
	// You is the player's island, 0 while their people wait to be reborn.
	You IslandID `json:"you"`
	// Own details the player's island; nil while it does not exist.
	Own *Own `json:"own,omitempty"`
	// Seen is the island the player looks at, when it is not their own.
	Seen *Seen `json:"seen,omitempty"`
	// BossInMs is the time before the next boss strikes the whole sky.
	BossInMs int64      `json:"bossInMs"`
	Raids    []RaidView `json:"raids,omitempty"`
	// WaveTarget and WaveInMs announce the next wave to everyone: the whole
	// sky sees it coming (GDD pilier 2).
	WaveTarget IslandID `json:"waveTarget"`
	WaveInMs   int64    `json:"waveInMs"`
	Annals     []Annal  `json:"annals,omitempty"`
}

// IslandID is re-exported for the clients.
type IslandID = sim.IslandID

// IslandSummary is what every player sees of an island from the sky: enough
// to judge a neighbour at a glance (GDD 5.4).
type IslandSummary struct {
	ID        IslandID `json:"id"`
	Name      string   `json:"name"`
	Seed      uint64   `json:"seed"`
	Radius    float64  `json:"radius"`
	Angle     float64  `json:"angle"`
	Size      float64  `json:"size"`
	AI        bool     `json:"ai,omitempty"`
	Culture   int      `json:"culture"` // dominant family
	Declining bool     `json:"declining,omitempty"`
	FallingMs int64    `json:"fallingMs,omitempty"`
	Heart     float64  `json:"heart"` // core health, 0 to 1
	// Banner shows a coveting or annihilating island's intent to the whole
	// sky (GDD 7.2); Punir leaves no sign.
	Banner       sim.Outer `json:"banner,omitempty"`
	BannerTarget IslandID  `json:"bannerTarget,omitempty"`
	// Protected marks a newcomer no one may raid (GDD 11.3).
	Protected bool    `json:"protected,omitempty"`
	Power     float64 `json:"power"`
}

// RaidView is a group crossing the sky, as everyone sees it.
type RaidView struct {
	ID        uint32       `json:"id"`
	Kind      sim.RaidKind `json:"kind"`
	From      IslandID     `json:"from"`
	To        IslandID     `json:"to"`
	Units     float64      `json:"units"`
	Progress  float64      `json:"progress"`
	Returning bool         `json:"returning,omitempty"`
}

// Summarize copies the public part of an island.
func Summarize(sky *sim.Sky, is *sim.Island) IslandSummary {
	sum := IslandSummary{
		ID: is.ID, Name: is.Name, Seed: is.Seed, Radius: is.Radius, Angle: is.Angle,
		Size: is.Civ.Size, AI: is.IsAI(), Culture: int(is.Civ.Dominant()),
		Declining: is.Declining, FallingMs: int64(is.Falling * 1000), Heart: is.Civ.CoreHealth,
		Protected: sky.Protected(is), Power: is.Civ.Power(),
	}
	if o := is.Civ.Outer; o.Kind == sim.Covet || o.Kind == sim.Annihilate {
		sum.Banner, sum.BannerTarget = o.Kind, o.Target
	}
	return sum
}

// Raids copies the groups crossing the sky.
func Raids(sky *sim.Sky) []RaidView {
	out := make([]RaidView, len(sky.Raids))
	for i, r := range sky.Raids {
		out[i] = RaidView{ID: r.ID, Kind: r.Kind, From: r.From, To: r.To, Units: r.Units,
			Progress: r.Progress, Returning: r.Returning}
	}
	return out
}

// Own is everything the god sees of their own island.
type Own struct {
	Generation int                        `json:"generation"`
	Stock      sim.Stock                  `json:"stock"`
	Cap        float64                    `json:"cap"`
	Population float64                    `json:"population"`
	Housing    int                        `json:"housing"`
	Defenders  float64                    `json:"defenders"`
	Assailants float64                    `json:"assailants"`
	Buildings  []sim.Building             `json:"buildings"`
	Features   []sim.Feature              `json:"features"`
	Pie        sim.Pie                    `json:"pie"`
	Conviction [sim.NumIntentions]float64 `json:"conviction"`
	Culture    [sim.NumFamilies]float64   `json:"culture"`
	Tiers      [sim.NumFamilies]int       `json:"tiers"`
	// Discoveries names the discoveries made, for the knowledge panel.
	Discoveries []string    `json:"discoveries"`
	Progress    float64     `json:"progress"` // towards the next tier, 0 to 1
	Axis        int         `json:"axis"`     // the axis the scholars follow
	Pending     *Supplique  `json:"pending,omitempty"`
	Chronicle   []sim.Entry `json:"chronicle"`
	Power       float64     `json:"power"`
	Defense     float64     `json:"defense"`
	// WaveInMs is the time before the next wave, when it aims at this island
	// (GDD 8.2), or -1.
	WaveInMs int64 `json:"waveInMs"`
	// Outer is the intention the god turned towards another island.
	Outer       sim.Outer `json:"outer,omitempty"`
	OuterTarget IslandID  `json:"outerTarget,omitempty"`
}

// Supplique is a question of the people waiting for an answer.
type Supplique struct {
	ID       uint32   `json:"id"`
	Question string   `json:"question"`
	Options  []string `json:"options"`
	LeftMs   int64    `json:"leftMs"`
}

// Annal is a fallen civilisation, as the Annals of the sky tell it.
type Annal struct {
	Name       string  `json:"name"`
	Generation int     `json:"generation"`
	DaysLived  float64 `json:"daysLived"`
	PeakPower  float64 `json:"peakPower"`
	Tiers      int     `json:"tiers"`
	Culture    int     `json:"culture"`
	Cause      string  `json:"cause"`
}

// chronicleLines is how much of the chronicle a state carries.
const chronicleLines = 20

// Detail copies what the god of an island sees of it. It must be called with
// the sky locked; the result shares nothing with the simulation.
func Detail(sky *sim.Sky, is *sim.Island) *Own {
	c := &is.Civ
	o := &Own{
		Generation: is.Generation,
		Stock:      c.Stock, Cap: c.Cap(), Population: c.Population, Housing: c.Housing(),
		Defenders: c.Defenders, Assailants: c.Assailants,
		Buildings: append([]sim.Building(nil), c.Buildings...),
		Features:  append([]sim.Feature(nil), c.Features...),
		Pie:       c.Pie, Conviction: c.Conviction, Culture: c.Culture,
		Power: c.Power(), Defense: c.Defense(),
		Axis:     int(c.Dominant()),
		WaveInMs: -1,
		Outer:    c.Outer.Kind, OuterTarget: c.Outer.Target,
	}
	for f := range sim.NumFamilies {
		o.Tiers[f] = c.Tech.Tier(f)
		for k, ch := range c.Tech.Chosen[f] {
			if ch >= 0 {
				o.Discoveries = append(o.Discoveries, sim.Discoveries[f][k][ch].Name)
			}
		}
	}
	if t := c.Tech.Tier(c.Dominant()); t < sim.TiersPerAxis {
		o.Progress = c.Tech.Progress / sim.TierCost(t)
	}
	if p := c.Pending; p != nil {
		o.Pending = &Supplique{ID: p.ID, Question: p.Question,
			Options: append([]string(nil), p.Options...), LeftMs: int64(p.Left * 1000)}
	}
	start := max(0, len(c.Chronicle)-chronicleLines)
	o.Chronicle = append([]sim.Entry(nil), c.Chronicle[start:]...)
	if w := sky.Wave; w != nil && w.Target == is.ID {
		o.WaveInMs = int64(w.In * 1000)
	}
	return o
}

// Annals copies the latest entries of the Annals of the sky.
func Annals(sky *sim.Sky, n int) []Annal {
	start := max(0, len(sky.Annals)-n)
	out := make([]Annal, 0, len(sky.Annals)-start)
	for _, a := range sky.Annals[start:] {
		out = append(out, Annal{
			Name: a.Name, Generation: a.Generation,
			DaysLived: float64(a.Fell-a.Born) * sim.TickDuration.Seconds() / 86400,
			PeakPower: a.PeakPower, Tiers: a.Tiers, Culture: int(a.Culture), Cause: a.Cause,
		})
	}
	return out
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

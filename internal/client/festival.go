package client

import (
	"image/color"
	"math"
	"time"

	"apogee/internal/geo"
	"apogee/internal/proto"
	"apogee/internal/sim"
)

// The people feast their milestones (GDD 13.1 bis): the first building of
// each kind, each ten more souls, each day the civilization lasts. For a
// while, lanterns hang in a ring around the heart, a bonfire burns before
// it, the villagers dance a farandole around both, and fireworks burst
// over the island.

// feastLength is how long a feast lasts, in seconds.
const feastLength = 45.0

// feastRing is how far from the heart the farandole goes round.
const feastRing = 11.0

var (
	festiveColours = []color.RGBA{{0xff, 0x6a, 0x5a, 0xff}, {0xff, 0xd2, 0x5a, 0xff}, {0x7a, 0xe0, 0x8a, 0xff}, {0x7a, 0xb8, 0xff, 0xff}, {0xf0, 0x8c, 0xff, 0xff}}
	fireColours    = []color.RGBA{{0xff, 0xd0, 0x5a, 0xff}, {0xff, 0x8c, 0x3c, 0xff}, {0xe0, 0x4a, 0x2a, 0xff}}
)

// feasts tells what the people already feasted.
type feasts struct {
	kinds map[sim.Kind]bool
	tens  int // tens of souls
	days  int // days of the civilization
	left  float64
	// begun is set when a feast starts, for the game to play its tune.
	begun bool
	// primed is set once the milestones of the people were first read:
	// those reached before are not feasted.
	primed bool
}

// watchFeasts starts a feast when the island reaches a milestone.
func (s *signs) watchFeasts(own *proto.Own) {
	f := &s.feasts
	if f.kinds == nil {
		f.kinds = map[sim.Kind]bool{}
	}
	first := false
	for _, b := range own.Buildings {
		if b.Progress >= 1 && b.Kind != sim.Core && !f.kinds[b.Kind] {
			f.kinds[b.Kind] = true
			first = true
		}
	}
	tens := int(own.Population) / 10
	days := int(time.Duration(own.Age) * sim.TickDuration / (24 * time.Hour))
	if f.primed && (first || tens > f.tens || days > f.days || s.rehearse == "feast" && f.left <= 0) {
		s.feast()
	}
	f.tens, f.days, f.primed = max(f.tens, tens), max(f.days, days), true
}

// feast starts the festivities.
func (s *signs) feast() {
	s.feasts.left, s.feasts.begun = feastLength, true
	s.calls = append(s.calls, gathering{key: "feast", at: s.heart, radius: feastRing, share: 0.8, act: dance, left: feastLength})
}

// feasting reports whether a feast is on.
func (s *signs) feasting() bool { return s.feasts.left > 0 }

// bonfire is where the fire of the feast burns, before the heart.
func (s *signs) bonfire() geo.Vec2 { return geo.Vec2{X: s.heart.X, Y: s.heart.Y + 8.5} }

// stepFeast burns the bonfire and lets the fireworks off.
func (s *signs) stepFeast(dt float64, dust *particles, emit bool) {
	if s.feasts.left <= 0 {
		return
	}
	s.feasts.left -= dt
	if !emit {
		return
	}
	r := s.r
	b := s.bonfire()
	if r.Float64() < 14*dt {
		dust.add(particle{x: b.X + (r.Float64()*2-1)*0.8, y: b.Y, z: 0.8, vz: 3 + r.Float64()*2,
			vx: (r.Float64()*2 - 1) * 0.4, life: 0.5 + r.Float64()*0.4, col: pick(r, fireColours)})
	}
	if r.Float64() < 0.7*dt {
		p := s.somewhere(0.5)
		c := festiveColours[r.IntN(len(festiveColours))]
		dust.burst(p.X, p.Y, 34+r.Float64()*10, 26, []color.RGBA{c, c, {0xff, 0xff, 0xff, 0xff}}, r)
	}
}

// festiveLamps are the lanterns hung around the heart, and the bonfire.
func (s *signs) festiveLamps() []lamp {
	if !s.feasting() {
		return nil
	}
	out := []lamp{{x: s.bonfire().X, y: s.bonfire().Y, z: 1, kind: brazierLamp, by: s.bonfire().Y, seed: 7}}
	const n = 14
	for i := range n {
		a := float64(i) * 2 * math.Pi / n
		x, y := s.heart.X+feastRing*1.15*math.Cos(a), s.heart.Y+feastRing*1.15*0.8*math.Sin(a)
		out = append(out, lamp{x: x, y: y, z: 4 + math.Abs(math.Sin(a*2))*0.8, kind: lanternLamp, by: y, seed: uint32(i), festive: true})
	}
	return out
}

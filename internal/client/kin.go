package client

import (
	"image/color"
	"math"

	"apogee/internal/geo"
	"apogee/internal/proto"
	"apogee/internal/sim"
)

// The people live and die on the island (GDD 13.1 bis): a birth gathers the
// neighbours before a house, cheering under a few pink sparks; a death
// takes a slow procession to the graveyard, where a stele rises and stays.
// The steles add up over the life of the civilization and tell its story;
// a new island starts with none.

// maxGraves is how many steles the graveyard shows, the newest.
const maxGraves = 12

var cradleColours = []color.RGBA{{0xff, 0xb8, 0xd8, 0xff}, {0xff, 0xe4, 0xf0, 0xff}, {0xf0, 0x8c, 0xb4, 0xff}}

// watchKin raises the births and deaths of the people since the last state.
func (s *signs) watchKin(own *proto.Own) {
	born, dead := int(own.Births), int(own.Deaths)
	s.placeGraveyard(own)
	// Screenshots: a birth at every state, deaths until the graveyard
	// holds a few steles.
	switch {
	case s.rehearse == "cradle" && s.primed:
		born = s.born + 1
	case s.rehearse == "funeral" && s.primed && s.dead < 7:
		dead = s.dead + 1
	}
	if !s.primed {
		s.born, s.dead = born, dead
		return
	}
	if born > s.born {
		if at, ok := s.someHome(own); ok {
			s.add(cradleSign, at.X, at.Y)
			door := s.shore.inside(geo.Vec2{X: at.X, Y: at.Y + sim.Specs[sim.House].Radius + 1.5})
			s.calls = append(s.calls, gathering{key: "cradle", at: door, radius: 3, share: 0.15, act: cheer, left: 6})
		}
	}
	if dead > s.dead {
		g := s.graves(dead)
		last := g[len(g)-1]
		s.add(mournSign, last.X, last.Y)
		s.calls = append(s.calls, gathering{key: "funeral", at: last, radius: 4, share: 0.4, act: pray, left: 16, pace: 0.45})
	}
	s.born, s.dead = born, dead
}

// someHome picks a house of the island, the newborn's.
func (s *signs) someHome(own *proto.Own) (geo.Vec2, bool) {
	var homes []geo.Vec2
	for _, b := range own.Buildings {
		if b.Kind == sim.House && b.Progress >= 1 {
			homes = append(homes, geo.Vec2{X: b.X, Y: b.Y})
		}
	}
	if len(homes) == 0 {
		return geo.Vec2{}, false
	}
	return homes[s.r.IntN(len(homes))], true
}

// placeGraveyard finds the graveyard a place away from the buildings and
// the natural sites, towards the shore, and moves it only if the people
// later build over it.
func (s *signs) placeGraveyard(own *proto.Own) {
	clear := func(p geo.Vec2) float64 {
		d := math.Inf(1)
		for _, b := range own.Buildings {
			if validKind(b.Kind) {
				d = math.Min(d, p.Dist(geo.Vec2{X: b.X, Y: b.Y})-sim.Specs[b.Kind].Radius)
			}
		}
		for _, f := range own.Features {
			d = math.Min(d, p.Dist(geo.Vec2{X: f.X, Y: f.Y})-f.Radius)
		}
		return d
	}
	if s.yard && clear(s.graveyard) > 5 {
		return
	}
	best, score := geo.Vec2{}, math.Inf(-1)
	for i, p := range s.shore.outline {
		if i%3 != 0 {
			continue
		}
		q := s.shore.inside(p.Scale(s.shore.size * 0.68))
		if c := clear(q); c > score {
			best, score = q, c
		}
	}
	s.graveyard, s.yard = best, true
}

// graves returns where the steles of n dead stand: rows of four around the
// graveyard, the newest last.
func (s *signs) graves(n int) []geo.Vec2 {
	n = min(n, maxGraves)
	out := make([]geo.Vec2, 0, n)
	for i := range n {
		row, col := i/4, i%4
		out = append(out, geo.Vec2{X: s.graveyard.X + (float64(col)-1.5)*2.4, Y: s.graveyard.Y + (float64(row)-1)*2.2})
	}
	return out
}

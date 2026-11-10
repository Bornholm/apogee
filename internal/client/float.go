package client

import (
	"image/color"
	"math"
	"math/rand/v2"

	"apogee/internal/geo"
	"apogee/internal/proto"
	"apogee/internal/sim"
)

// The island floats (GDD 12.6): it rises and sinks a pixel as it drifts,
// streams fall from its springs over the cliff into the void, and stones
// come loose from its underside, more and more as it nears the whirlpool,
// so that the descent shows without a word.

// bobPeriod is how long the island takes to rise and sink back, in seconds.
const bobPeriod = 11.0

// bob returns how many virtual pixels the island floats above its rest.
func bob(clock float64, seed uint64) float64 {
	return math.Round(math.Sin(clock*2*math.Pi/bobPeriod+float64(seed%7)) * 1.4)
}

var waterColours = []color.RGBA{{0xc8, 0xf0, 0xff, 0xff}, {0x8c, 0xd2, 0xf4, 0xff}, {0x5e, 0xa8, 0xe0, 0xff}, {0xff, 0xff, 0xff, 0xff}}

// scenery is what the island in view does as a floating island.
type scenery struct {
	id       proto.IslandID
	size     float64
	falls    []geo.Vec2 // springs on the front shore
	front    []geo.Vec2 // the shore seen from the camera, where stones fall
	exposure float64    // 0 at the rim, 1 at the whirlpool
	r        *rand.Rand
}

// keep follows the island in view.
func (s *scenery) keep(sum proto.IslandSummary) {
	s.exposure = sim.Exposure(sum.Radius)
	if s.r != nil && s.id == sum.ID && s.size == sum.Size {
		return
	}
	s.id, s.size = sum.ID, sum.Size
	s.r = rand.New(rand.NewPCG(sum.Seed, 0xfa11))
	outline := geo.Outline(sum.Seed)
	s.front, s.falls = s.front[:0], s.falls[:0]
	for _, p := range outline {
		if p.Y > 0.35 {
			s.front = append(s.front, p.Scale(sum.Size))
		}
	}
	// One or two springs, never side by side.
	if len(s.front) == 0 {
		return
	}
	n := 1 + int(sum.Seed%2)
	for i := range n {
		k := (int(sum.Seed>>3) + i*len(s.front)/2) % len(s.front)
		s.falls = append(s.falls, s.front[k])
	}
}

// step lets the water and the stones fall.
func (s *scenery) step(dt float64, dust *particles) {
	if s.r == nil || len(dust.ps) >= maxParticles*3/4 {
		return
	}
	r := s.r
	for _, f := range s.falls {
		// A thin stream: a few drops a second, spreading as they fall.
		for range 2 {
			if r.Float64() > 14*dt {
				continue
			}
			out := f.Scale(1 / math.Max(math.Hypot(f.X, f.Y), 1))
			v := 0.6 + r.Float64()*0.8
			dust.add(particle{x: f.X + (r.Float64()*2-1)*0.5, y: f.Y, z: 0,
				vx: out.X*v + (r.Float64()*2-1)*0.2, vy: out.Y * v, vz: 0.5,
				life: 2.2 + r.Float64()*0.8, col: pick(r, waterColours), heavy: true, deep: true})
		}
	}
	// Stones come loose under the island: seldom at the rim, often near
	// the whirlpool.
	if len(s.front) > 0 && r.Float64() < (0.15+2.5*s.exposure*s.exposure)*dt {
		p := s.front[r.IntN(len(s.front))].Scale(0.6 + r.Float64()*0.3)
		dust.add(particle{x: p.X, y: p.Y, z: -s.size * (0.3 + r.Float64()*0.3), vz: -0.5,
			life: 2 + r.Float64(), col: pick(r, rockColours), heavy: true, deep: true})
	}
}

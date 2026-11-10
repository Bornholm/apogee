package client

import (
	"image/color"
	"math"
	"math/rand/v2"

	"apogee/internal/art"
	"apogee/internal/geo"
	"apogee/internal/proto"
	"apogee/internal/sim"
)

// The island is not only its people: smoke rises from the chimneys and
// drifts on the wind, sheep graze by the fields, and birds wheel high over
// the island by day, fleeing when a wave darkens the sky.

// smokeRate is how many puffs a chimney lets out per second.
const smokeRate = 1.6

var chimneyColours = []color.RGBA{{0xd8, 0xd4, 0xdc, 0xff}, {0xb4, 0xb0, 0xbe, 0xff}, {0x94, 0x90, 0xa2, 0xff}}

// islandChimneys lists where smoke leaves the roofs of the homes.
func islandChimneys(sum proto.IslandSummary, own *proto.Own) []art.Vec3 {
	var out []art.Vec3
	for _, b := range own.Buildings {
		if b.Progress < 1 || (b.Kind != sim.House && b.Kind != sim.Woodcutter) {
			continue
		}
		c := art.Chimney(meshFor(b.Kind, sum.Seed*977+uint64(b.ID)))
		out = append(out, art.V(b.X+c.X, b.Y+c.Y, c.Z))
	}
	return out
}

// smoke lets puffs out of the chimneys: they rise, spread and drift east.
func (p *particles) smoke(chimneys []art.Vec3, dt float64) {
	if p.r == nil {
		p.r = rand.New(rand.NewPCG(7, 11))
	}
	if len(p.ps) >= maxParticles*3/4 {
		return
	}
	r := p.r
	for _, c := range chimneys {
		if r.Float64() > smokeRate*dt {
			continue
		}
		p.add(particle{x: c.X + (r.Float64()*2-1)*0.3, y: c.Y, z: c.Z,
			vx: 0.6 + r.Float64()*0.6, vy: -(r.Float64() * 0.3), vz: 1.4 + r.Float64()*0.6,
			sway: 0.4, phase: r.Float64() * 6,
			life: 3 + r.Float64()*2.5, col: chimneyColours[r.IntN(len(chimneyColours))]})
	}
}

// grazer is a sheep by its field.
type grazer struct {
	x, y, tx, ty float64
	pen          geo.Vec2 // the field it grazes around
	wait         float64
	head         bool // head down, grazing
}

// sheepPerField and maxSheep bound the flock.
const (
	sheepPerField = 2
	maxSheep      = 8
	sheepSpeed    = 1.2
)

// keepFlock puts sheep by each finished field, keeping those already there.
func (l *life) keepFlock(own *proto.Own) {
	var pens []geo.Vec2
	for _, b := range own.Buildings {
		if b.Kind == sim.Farm && b.Progress >= 1 {
			pens = append(pens, geo.Vec2{X: b.X, Y: b.Y})
		}
	}
	kept := l.flock[:0]
	for _, s := range l.flock {
		for _, p := range pens {
			if p == s.pen {
				kept = append(kept, s)
				break
			}
		}
	}
	l.flock = kept
	for _, p := range pens {
		have := 0
		for _, s := range l.flock {
			if s.pen == p {
				have++
			}
		}
		for ; have < sheepPerField && len(l.flock) < maxSheep; have++ {
			t := l.pasture(p, nil)
			l.flock = append(l.flock, grazer{x: t.X, y: t.Y, tx: t.X, ty: t.Y, pen: p, wait: l.r.Float64() * 4})
		}
	}
}

// pasture picks a spot beside a field, out of its footprint: anywhere
// around it for a new sheep, a few steps along the edge for one already
// grazing, so that it walks around the crops rather than through them.
func (l *life) pasture(pen geo.Vec2, from *grazer) geo.Vec2 {
	r := sim.Specs[sim.Farm].Radius
	for range 8 {
		a := l.r.Float64() * 2 * math.Pi
		if from != nil {
			a = math.Atan2(from.y-pen.Y, from.x-pen.X) + (l.r.Float64()*2-1)*0.7
		}
		d := r + 1 + l.r.Float64()*3
		p := l.shore.inside(geo.Vec2{X: pen.X + d*math.Cos(a), Y: pen.Y + d*math.Sin(a)})
		if p.Dist(pen) >= r { // the shore did not push it into the field
			return p
		}
	}
	// A field hemmed in by the cliff: they stay on its inner side.
	toward := pen.Scale(-1 / math.Max(math.Hypot(pen.X, pen.Y), 1))
	return geo.Vec2{X: pen.X + toward.X*(r+1), Y: pen.Y + toward.Y*(r+1)}
}

// stepFlock lets the sheep graze, then amble a few steps further.
func (l *life) stepFlock(dt float64) {
	for i := range l.flock {
		s := &l.flock[i]
		dx, dy := s.tx-s.x, s.ty-s.y
		d := math.Hypot(dx, dy)
		if step := sheepSpeed * dt; d > step {
			s.x += dx / d * step
			s.y += dy / d * step
			s.head = false
			continue
		}
		s.x, s.y = s.tx, s.ty
		s.head = true
		if s.wait -= dt; s.wait <= 0 {
			t := l.pasture(s.pen, s)
			s.tx, s.ty, s.wait = t.X, t.Y, 4+l.r.Float64()*8
		}
	}
}

// bird wheels over the island.
type bird struct {
	radius, angle, speed, height, phase float64
}

// birds are a flock over the island in view; away grows from 0 to 1 while
// they flee the night or a wave, and back when the sky is calm.
type birds struct {
	flock []bird
	away  float64
	size  float64
}

// keep sets up the flock for an island of a size.
func (b *birds) keep(size float64, seed uint64) {
	if len(b.flock) > 0 && b.size == size {
		return
	}
	b.size = size
	r := art.Rng(seed + 0xb1)
	b.flock = b.flock[:0]
	n := 4 + r.IntN(4)
	for range n {
		b.flock = append(b.flock, bird{
			radius: size * (0.5 + r.Float64()*0.6),
			angle:  r.Float64() * 2 * math.Pi,
			speed:  (0.12 + r.Float64()*0.1),
			height: 30 + r.Float64()*18,
			phase:  r.Float64() * 6,
		})
	}
	b.away = 1 // they come in when first seen
}

// step moves the flock, fleeing when scared.
func (b *birds) step(dt float64, scared bool) {
	target := 0.0
	if scared {
		target = 1
	}
	rate := 0.25 // a few seconds to come back
	if scared {
		rate = 0.6 // and less to flee
	}
	if b.away < target {
		b.away = math.Min(target, b.away+rate*dt)
	} else {
		b.away = math.Max(target, b.away-rate*dt)
	}
	for i := range b.flock {
		f := &b.flock[i]
		f.angle += f.speed * dt * (1 + 2*b.away)
	}
}

// visible reports whether some bird is still near enough to see.
func (b *birds) visible() bool { return len(b.flock) > 0 && b.away < 0.98 }

// at returns where a bird is, in island units, and whether its wings are up.
func (b *birds) at(i int, clock float64) (x, y, z float64, up bool) {
	f := b.flock[i]
	k := ease(b.away)
	r := f.radius * (1 + 2.5*k)
	x = r * math.Cos(f.angle)
	y = r * 0.6 * math.Sin(f.angle)
	z = f.height + k*60 + math.Sin(clock*0.7+f.phase)*2
	return x, y, z, math.Sin(clock*9+f.phase) > 0
}

var birdColour = color.RGBA{0x3a, 0x30, 0x48, 0xff}

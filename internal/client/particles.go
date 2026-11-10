package client

import (
	"image/color"
	"math"
	"math/rand/v2"
)

// Particles show the island at work: leaves falling from the groves where
// villagers fell trees, sawdust on the building sites, stone chips at the
// quarries, golden chaff over the fields, aether sparkles at the crystals.
// Each is a single virtual pixel, like the rest of the pixel art.

const maxParticles = 400

type particle struct {
	x, y, z    float64 // island units; z is the height above the ground
	vx, vy, vz float64
	life       float64 // seconds left
	col        color.RGBA
	// sway makes falling leaves drift from side to side.
	sway, phase float64
	// heavy particles (sawdust, chips) fall back under gravity; light ones
	// (leaves, chaff, sparkles) keep their pace.
	heavy bool
	// deep particles fall past the ground, down the cliffs.
	deep bool
}

// Particle colours, taken from the palette's ramps.
var (
	leafColours  = []color.RGBA{{0x55, 0x84, 0x5a, 0xff}, {0x3b, 0x64, 0x48, 0xff}, {0x7a, 0xa3, 0x5a, 0xff}}
	dustColours  = []color.RGBA{{0xea, 0xda, 0xb4, 0xff}, {0xc8, 0xb3, 0x8c, 0xff}, {0xa0, 0x71, 0x4a, 0xff}}
	chipColours  = []color.RGBA{{0xcf, 0xc9, 0xbc, 0xff}, {0xa2, 0x9c, 0x90, 0xff}}
	chaffColours = []color.RGBA{{0xec, 0xd0, 0x65, 0xff}, {0xcd, 0xa8, 0x43, 0xff}}
	sparkColours = []color.RGBA{{0xc2, 0xf6, 0xff, 0xff}, {0x6c, 0xcb, 0xf2, 0xff}, {0xff, 0xff, 0xff, 0xff}}
)

// emitRate is how many particles per second a villager raises at each job.
var emitRate = [...]float64{idle: 0, building: 5, felling: 2.5, quarrying: 4, farming: 2, channel: 3}

type particles struct {
	ps []particle
	r  *rand.Rand
}

// emit raises the particles of a villager at work, about rate per second.
func (p *particles) emit(w *walker, dt float64) {
	if p.r == nil {
		p.r = rand.New(rand.NewPCG(7, 11))
	}
	rate := emitRate[w.work.job]
	if rate == 0 || p.r.Float64() > rate*dt || len(p.ps) >= maxParticles {
		return
	}
	r := p.r
	pick := func(cs []color.RGBA) color.RGBA { return cs[r.IntN(len(cs))] }
	q := particle{x: w.x + (r.Float64()*2-1)*1.2, y: w.y - 0.5, z: 1.5}
	switch w.work.job {
	case building: // sawdust puffs out and settles
		q.vx, q.vy, q.vz = (r.Float64()*2-1)*3, (r.Float64()*2-1)*1.5, 2+r.Float64()*3
		q.life, q.col, q.heavy = 1+r.Float64()*0.6, pick(dustColours), true
	case felling: // a leaf falls from the crown of a tree in the grove
		a := r.Float64() * 2 * math.Pi
		d := w.work.radius * 0.7 * math.Sqrt(r.Float64())
		q.x, q.y = w.work.centre.X+d*math.Cos(a), w.work.centre.Y+d*math.Sin(a)
		q.z = 6 + r.Float64()*6
		q.vz, q.sway, q.phase = -(1 + r.Float64()), 0.8+r.Float64()*0.8, r.Float64()*6
		q.life, q.col = 4+r.Float64()*3, pick(leafColours)
	case quarrying: // chips fly from the stone
		q.vx, q.vy, q.vz = (r.Float64()*2-1)*5, (r.Float64()*2-1)*2.5, 3+r.Float64()*4
		q.life, q.col, q.heavy = 0.8+r.Float64()*0.4, pick(chipColours), true
	case farming: // chaff lifts on the wind and drifts away
		q.vx, q.vy, q.vz = 1+r.Float64()*1.5, (r.Float64()*2-1)*0.5, 0.8+r.Float64()
		q.life, q.col = 2+r.Float64()*1.5, pick(chaffColours)
	case channel: // sparkles rise from the aether
		q.x, q.y = w.work.centre.X+(r.Float64()*2-1)*w.work.radius, w.work.centre.Y+(r.Float64()*2-1)*w.work.radius*0.6
		q.z, q.vz = 1+r.Float64()*3, 2+r.Float64()*2
		q.life, q.col = 1.2+r.Float64(), pick(sparkColours)
	}
	p.ps = append(p.ps, q)
}

// add raises one particle, if there is room.
func (p *particles) add(q particle) {
	if len(p.ps) < maxParticles {
		p.ps = append(p.ps, q)
	}
}

// gravity pulls sawdust and chips down; leaves and sparkles keep their pace.
const gravity = 9.0

// step moves the particles and drops the dead ones.
func (p *particles) step(dt float64) {
	kept := p.ps[:0]
	for _, q := range p.ps {
		q.life -= dt
		if q.life <= 0 {
			continue
		}
		if q.sway > 0 { // a falling leaf
			q.phase += dt * 2.5
			q.x += math.Sin(q.phase) * q.sway * dt
		}
		if q.heavy {
			q.vz -= gravity * dt
		}
		q.x += q.vx * dt
		q.y += q.vy * dt
		q.z += q.vz * dt
		if q.z <= 0 && q.vz <= 0 && !q.deep {
			// Landed: leaves and chips rest a moment on the ground, then
			// vanish.
			q.z, q.vx, q.vy, q.vz, q.sway, q.heavy = 0, 0, 0, 0, 0, false
			q.life = math.Min(q.life, 0.6)
		}
		kept = append(kept, q)
	}
	p.ps = kept
}

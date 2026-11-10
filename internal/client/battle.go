package client

import (
	"image/color"
	"math"
	"math/rand/v2"

	"apogee/internal/geo"
	"apogee/internal/proto"
	"apogee/internal/sim"
)

// The battle plays out on the island the last attack it suffered (GDD 13.1
// bis). The simulation resolves an attack at once; the client then shows it
// over a few seconds: the assailants come out of the sky, hover over the
// island while the heart, the towers and the guards fire at them, then fall
// if the island held, or strike the heart and fly away if it did not.

// Timeline of a battle, in seconds.
const (
	arriveTime = 2.5 // the assailants fly in
	fightTime  = 6.5 // they hover and are shot at
	battleTime = 9.0 // they fall, or leave
	strikeTime = 4.5 // the heart is struck, when the island did not hold
	strikeLen  = 0.8
)

// Sprites of the assailants: three monsters, the colossus, a raider.
const (
	colossusArt = 3
	raiderArt   = 4
	foeArts     = 5
)

// foe is one assailant of the battle, in island units.
type foe struct {
	hx, hy     float64 // the spot it hovers over
	fx, fy, fz float64 // where it came from
	x, y, z    float64
	phase      float64
	art        int  // which sprite
	down       bool // fell and burst
}

// gun is a place the island shoots from: the heart, a tower, a guard.
type gun struct{ x, y, z float64 }

type battle struct {
	tick  uint64 // the attack shown, by its tick
	t     float64
	b     sim.Battle
	foes  []foe
	guns  []gun
	r     *rand.Rand
	fire  float64 // seconds until the next volley
	heart gun
	hit   bool // the heart was struck
	// feared and cheered tell whether the people already fled the blow,
	// or celebrated the victory, of this battle.
	feared, cheered bool
}

func (b *battle) active() bool { return len(b.foes) > 0 && b.t < battleTime }

// start sets up the battle of an attack on an island.
func (b *battle) start(bt sim.Battle, sh shore, own *proto.Own) {
	r := rand.New(rand.NewPCG(bt.Tick, 0xba77))
	*b = battle{tick: bt.Tick, b: bt, r: r}
	for _, bd := range own.Buildings {
		if bd.Progress < 1 {
			continue
		}
		switch bd.Kind {
		case sim.Core:
			b.heart = gun{bd.X, bd.Y, 9}
			b.guns = append(b.guns, b.heart)
		case sim.Tower:
			b.guns = append(b.guns, gun{bd.X, bd.Y, 12})
		}
	}
	if len(b.guns) == 0 {
		b.guns = []gun{{0, 0, 9}}
	}
	// The assailants come together, from one side of the sky.
	bearing := r.Float64() * 2 * math.Pi
	far := math.Max(sh.size, 20) * 1.7
	n := min(max(bt.Foes, 1), 10)
	if bt.By == sim.ColossusFoe {
		n = 1 // the colossus comes alone
	}
	for i := range n {
		a := r.Float64() * 2 * math.Pi
		d := 9 + r.Float64()*16
		h := sh.inside(geo.Vec2{X: b.heart.x + d*math.Cos(a), Y: b.heart.y + d*math.Sin(a)})
		from := bearing + (r.Float64()*2-1)*0.4
		f := foe{hx: h.X, hy: h.Y, fx: far * math.Cos(from), fy: far * math.Sin(from),
			fz: 25 + r.Float64()*15, phase: r.Float64() * 6, art: i % 3}
		switch bt.By {
		case sim.ColossusFoe:
			f.art = colossusArt
		case sim.RaiderFoe:
			f.art = raiderArt
		}
		f.x, f.y, f.z = f.fx, f.fy, f.fz
		b.foes = append(b.foes, f)
	}
}

// hoverZ is how high the assailants float while they fight.
func (b *battle) hoverZ() float64 {
	switch b.b.By {
	case sim.ColossusFoe:
		return 3
	case sim.RaiderFoe:
		return 0
	}
	return 1.5
}

func ease(k float64) float64 {
	k = math.Min(math.Max(k, 0), 1)
	return k * k * (3 - 2*k)
}

// step advances the battle by dt seconds. Shots and bursts go into dust
// when emit is set, that is when the island is in view.
func (b *battle) step(dt float64, dust *particles, emit bool, guards []walker) {
	if !b.active() {
		return
	}
	b.t += dt
	t := b.t
	hz := b.hoverZ()
	for i := range b.foes {
		f := &b.foes[i]
		lag := float64(i) * 0.12
		sx := f.hx + math.Sin(t*2.7+f.phase)*1.2
		sy := f.hy + math.Cos(t*2.1+f.phase)*0.8
		sz := hz + math.Sin(t*4.3+f.phase)*0.5*math.Min(hz, 1)
		switch {
		case t < arriveTime+lag:
			k := ease((t - lag) / arriveTime)
			f.x, f.y, f.z = f.fx+(sx-f.fx)*k, f.fy+(sy-f.fy)*k, f.fz+(sz-f.fz)*k
		case t < fightTime+lag || f.down:
			f.x, f.y, f.z = sx, sy, sz
		case b.b.Repelled:
			// Shot down: it drops, and bursts on the ground.
			e := t - fightTime - lag
			f.x, f.y, f.z = sx, sy, sz-e*e*14
			if f.z <= 0 && !f.down {
				f.down = true
				if emit {
					dust.burst(f.x, f.y, 0.5, 24, monsterColours, b.r)
				}
			}
		default:
			// The island did not hold: they leave the way they came.
			k := ease((t - fightTime - lag) / (battleTime - fightTime - 0.5))
			f.x, f.y, f.z = sx+(f.fx-sx)*k, sy+(f.fy-sy)*k, sz+(f.fz-sz)*k
		}
	}
	if !emit {
		return
	}
	// Volleys from the heart, the towers and the guards while they fight.
	if t > arriveTime*0.7 && t < fightTime+0.4 {
		if b.fire -= dt; b.fire <= 0 {
			b.fire = 0.08 + b.r.Float64()*0.14
			guns := b.guns
			for _, w := range guards {
				guns = append(guns, gun{w.x, w.y, 2.5})
			}
			g := guns[b.r.IntN(len(guns))]
			if f := b.target(); f != nil {
				dust.bolt(g.x, g.y, g.z, f.x, f.y, f.z+7, b.r)
			}
		}
	}
	if !b.b.Repelled && !b.hit && t >= strikeTime {
		b.hit = true
		dust.burst(b.heart.x, b.heart.y, 6, 40, append(append([]color.RGBA(nil), monsterColours...), bloodColours...), b.r)
	}
}

// reaction returns how the people answer the battle at this moment, once:
// they flee home when the heart is struck, and cheer around it when the
// assailants have fallen. The cues follow the animation's own clock.
func (b *battle) reaction() (gathering, bool) {
	switch {
	case b.hit && !b.feared:
		b.feared = true
		return gathering{key: "fear", atHome: true, share: 1, act: hide, left: 10}, true
	case b.b.Repelled && !b.cheered && b.t >= fightTime+1:
		b.cheered = true
		return gathering{key: "victory", at: geo.Vec2{X: b.heart.x, Y: b.heart.y}, radius: 8, share: 0.6, act: cheer, left: 6}, true
	}
	return gathering{}, false
}

// target picks an assailant still in the air.
func (b *battle) target() *foe {
	n := b.r.IntN(len(b.foes))
	for i := range b.foes {
		if f := &b.foes[(n+i)%len(b.foes)]; !f.down {
			return f
		}
	}
	return nil
}

// strike returns how strongly the heart flashes red, from 0 to 1.
func (b *battle) strike() float64 {
	if !b.active() || b.b.Repelled || b.t < strikeTime || b.t > strikeTime+strikeLen {
		return 0
	}
	return 1 - (b.t-strikeTime)/strikeLen
}

// rally returns where the guards run to: the assailants, while they fight.
func (b *battle) rally() []geo.Vec2 {
	if !b.active() || b.t > fightTime {
		return nil
	}
	pts := make([]geo.Vec2, 0, len(b.foes))
	for _, f := range b.foes {
		pts = append(pts, geo.Vec2{X: f.hx, Y: f.hy})
	}
	return pts
}

var (
	monsterColours = []color.RGBA{{0xf0, 0x8c, 0xff, 0xff}, {0xa8, 0x3f, 0xd0, 0xff}, {0x4a, 0x14, 0x68, 0xff}}
	bloodColours   = []color.RGBA{{0xf0, 0x67, 0x5a, 0xff}, {0xbf, 0x3b, 0x36, 0xff}}
)

// burst throws n particles of the given colours out of a point.
func (p *particles) burst(x, y, z float64, n int, cols []color.RGBA, r *rand.Rand) {
	for range n {
		if len(p.ps) >= maxParticles {
			return
		}
		a := r.Float64() * 2 * math.Pi
		v := 2 + r.Float64()*5
		p.ps = append(p.ps, particle{x: x, y: y, z: z,
			vx: v * math.Cos(a), vy: v * math.Sin(a) * 0.6, vz: 2 + r.Float64()*5,
			life: 0.6 + r.Float64()*0.6, col: cols[r.IntN(len(cols))], heavy: true})
	}
}

// bolt shoots a spark of aether from a gun to a point: a short trail of
// particles that reach it together.
func (p *particles) bolt(x0, y0, z0, x1, y1, z1 float64, r *rand.Rand) {
	const flight = 0.3
	col := sparkColours[r.IntN(len(sparkColours))]
	for i := range 6 {
		if len(p.ps) >= maxParticles {
			return
		}
		lag := float64(i) * 0.02
		p.ps = append(p.ps, particle{x: x0, y: y0, z: z0,
			vx: (x1 - x0) / (flight + lag), vy: (y1 - y0) / (flight + lag), vz: (z1 - z0) / (flight + lag),
			life: flight + lag, col: col})
	}
}

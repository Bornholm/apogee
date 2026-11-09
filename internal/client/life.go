package client

import (
	"math"
	"math/rand/v2"

	"apogee/internal/geo"
	"apogee/internal/proto"
	"apogee/internal/sim"
)

// The island must feel alive when the god looks at it: villagers walk from
// their homes to the building sites, fields, quarries and groves, work there
// a while and come back; defenders patrol around the towers and the heart.
// This is pure decoration, computed by the client: the simulation only knows
// how many people there are and where the buildings stand (GDD 10.3).

const (
	walkSpeed  = 4.0 // island units per second
	maxWalkers = 24
	maxGuards  = 16
)

type walkerState int

const (
	resting walkerState = iota
	going
	working
	returning
)

// walker is one villager or defender of the island view.
type walker struct {
	x, y   float64
	tx, ty float64
	home   geo.Vec2
	state  walkerState
	wait   float64 // seconds left resting or working
	guard  bool
	// Guards circle a post.
	post          geo.Vec2
	angle, radius float64
	speed         float64 // radians per second
}

// life animates the people of an island.
type life struct {
	island  proto.IslandID
	walkers []walker
	r       *rand.Rand
}

// places lists where villagers may go: building sites first, then the
// producers and the natural sites they work. Each place is the front of a
// footprint, on its south side, as a door or the foot of a building site:
// people stand by buildings, never on them.
func places(own *proto.Own) (sites, works, homes []geo.Vec2) {
	front := func(x, y, radius float64) geo.Vec2 { return geo.Vec2{X: x, Y: y + radius + 1} }
	for _, b := range own.Buildings {
		p := front(b.X, b.Y, sim.Specs[b.Kind].Radius)
		switch {
		case b.Progress < 1:
			sites = append(sites, p)
		case b.Kind == sim.House || b.Kind == sim.Core:
			homes = append(homes, p)
		case sim.Specs[b.Kind].Jobs > 0 || b.Kind == sim.Warehouse:
			works = append(works, p)
		}
	}
	for _, f := range own.Features {
		works = append(works, front(f.X, f.Y, f.Radius))
	}
	return sites, works, homes
}

// sync matches the walkers to the island: as many villagers as the people
// (up to maxWalkers) and guards as the defenders.
func (l *life) sync(id proto.IslandID, own *proto.Own) {
	if l.r == nil || l.island != id {
		*l = life{island: id, r: rand.New(rand.NewPCG(uint64(id), 0x11fe))}
	}
	_, _, homes := places(own)
	if len(homes) == 0 {
		homes = []geo.Vec2{{}}
	}
	var posts []geo.Vec2
	for _, b := range own.Buildings {
		if b.Progress >= 1 && (b.Kind == sim.Tower || b.Kind == sim.Core) {
			posts = append(posts, geo.Vec2{X: b.X, Y: b.Y})
		}
	}
	if len(posts) == 0 {
		posts = []geo.Vec2{{}}
	}
	wantPeople := min(int(own.Population), maxWalkers)
	wantGuards := min(int(own.Defenders), maxGuards)
	var people, guards []walker
	for _, w := range l.walkers {
		if w.guard && len(guards) < wantGuards {
			guards = append(guards, w)
		} else if !w.guard && len(people) < wantPeople {
			people = append(people, w)
		}
	}
	for len(people) < wantPeople {
		h := homes[l.r.IntN(len(homes))]
		people = append(people, walker{x: h.X, y: h.Y, home: h, wait: l.r.Float64() * 5})
	}
	for len(guards) < wantGuards {
		p := posts[l.r.IntN(len(posts))]
		g := walker{guard: true, post: p, angle: l.r.Float64() * 2 * math.Pi,
			radius: 6 + l.r.Float64()*4, speed: (0.08 + l.r.Float64()*0.08) * float64(1-2*l.r.IntN(2))}
		g.x, g.y = p.X+g.radius*math.Cos(g.angle), p.Y+g.radius*math.Sin(g.angle)
		guards = append(guards, g)
	}
	l.walkers = append(people, guards...)
}

// step moves everyone by dt seconds. At night, villagers linger at home.
func (l *life) step(own *proto.Own, dt float64, night bool) {
	sites, works, _ := places(own)
	for i := range l.walkers {
		w := &l.walkers[i]
		if w.guard {
			w.angle += w.speed * dt
			w.x = w.post.X + w.radius*math.Cos(w.angle)
			w.y = w.post.Y + w.radius*math.Sin(w.angle)
			continue
		}
		switch w.state {
		case resting:
			if w.wait -= dt; w.wait > 0 {
				continue
			}
			// Off to work: building sites draw the most hands.
			var to geo.Vec2
			switch {
			case len(sites) > 0 && l.r.Float64() < 0.6:
				to = sites[l.r.IntN(len(sites))]
			case len(works) > 0:
				to = works[l.r.IntN(len(works))]
			default:
				to = geo.Vec2{X: w.home.X + (l.r.Float64()*2-1)*10, Y: w.home.Y + (l.r.Float64()*2-1)*10}
			}
			// Spread along the front, never into the footprint.
			w.tx, w.ty = to.X+(l.r.Float64()*2-1)*3, to.Y+l.r.Float64()*1.5
			w.state = going
		case going, returning:
			if w.walk(dt) {
				if w.state == going {
					w.state, w.wait = working, 8+l.r.Float64()*14
				} else {
					w.state, w.wait = resting, 3+l.r.Float64()*6
					if night {
						w.wait += 40 + l.r.Float64()*40
					}
				}
			}
		case working:
			if w.wait -= dt; w.wait <= 0 {
				w.tx, w.ty = w.home.X+(l.r.Float64()*2-1)*2, w.home.Y+l.r.Float64()
				w.state = returning
			}
		}
	}
}

// walk moves the walker towards its target and reports its arrival.
func (w *walker) walk(dt float64) bool {
	dx, dy := w.tx-w.x, w.ty-w.y
	d := math.Hypot(dx, dy)
	step := walkSpeed * dt
	if d <= step {
		w.x, w.y = w.tx, w.ty
		return true
	}
	w.x += dx / d * step
	w.y += dy / d * step
	return false
}

// working reports whether the walker is busy on the spot: it then bobs, as
// if hammering or harvesting.
func (w *walker) busy() bool { return !w.guard && w.state == working }

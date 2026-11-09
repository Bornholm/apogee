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
	// cliffMargin keeps everyone this far from the edge of the island.
	cliffMargin = 2.5
)

type walkerState int

const (
	resting walkerState = iota
	going
	working
	returning
)

// job tells what a villager does where it works, which decides the
// particles it raises.
type job int

const (
	idle      job = iota
	building      // sawdust
	felling       // leaves falling from the grove
	quarrying     // stone chips
	farming       // golden chaff
	channel       // aether sparkles
)

// spot is a place villagers walk to: the front of a footprint, on its south
// side, as a door or the foot of a building site.
type spot struct {
	p      geo.Vec2
	job    job
	centre geo.Vec2 // the footprint's centre, for leaves falling from a grove
	radius float64
}

// walker is one villager or defender of the island view.
type walker struct {
	x, y   float64
	tx, ty float64
	home   geo.Vec2
	state  walkerState
	wait   float64 // seconds left resting or working
	work   spot
	guard  bool
	// Guards circle a post.
	post          geo.Vec2
	angle, radius float64
	speed         float64 // radians per second
}

// shore is the outline of the island in view, at its size.
type shore struct {
	outline []geo.Vec2
	size    float64
}

// inside pulls a point towards the centre until it stands on the island,
// away from the cliff.
func (s shore) inside(p geo.Vec2) geo.Vec2 {
	if s.outline == nil {
		return p
	}
	for range 32 {
		if geo.EdgeDistance(s.outline, s.size, p) >= cliffMargin {
			return p
		}
		p = p.Scale(0.9)
	}
	return geo.Vec2{}
}

// life animates the people of an island.
type life struct {
	island  proto.IslandID
	walkers []walker
	shore   shore
	r       *rand.Rand
}

// featureJob is what villagers do at each natural site.
var featureJob = [...]job{sim.NoFeature: idle, sim.Grove: felling, sim.Rock: quarrying, sim.Crystal: channel}

func jobOf(k sim.Kind) job {
	switch k {
	case sim.Farm:
		return farming
	case sim.Woodcutter:
		return building // sawing logs
	case sim.Quarry:
		return quarrying
	case sim.Extractor, sim.Library:
		return channel
	}
	return idle
}

// places lists where villagers may go: building sites first, then the
// producers and the natural sites they work. People stand by buildings,
// never on them, and never beyond the cliff.
func (l *life) places(own *proto.Own) (sites, works []spot, homes []geo.Vec2) {
	front := func(x, y, radius float64, j job) spot {
		return spot{p: l.shore.inside(geo.Vec2{X: x, Y: y + radius + 1}), job: j,
			centre: geo.Vec2{X: x, Y: y}, radius: radius}
	}
	for _, b := range own.Buildings {
		r := sim.Specs[b.Kind].Radius
		switch {
		case b.Progress < 1:
			sites = append(sites, front(b.X, b.Y, r, building))
		case b.Kind == sim.House || b.Kind == sim.Core:
			homes = append(homes, front(b.X, b.Y, r, idle).p)
		case sim.Specs[b.Kind].Jobs > 0 || b.Kind == sim.Warehouse:
			works = append(works, front(b.X, b.Y, r, jobOf(b.Kind)))
		}
	}
	for _, f := range own.Features {
		works = append(works, front(f.X, f.Y, f.Radius, featureJob[f.Kind]))
	}
	return sites, works, homes
}

// sync matches the walkers to the island: as many villagers as the people
// (up to maxWalkers) and guards as the defenders.
func (l *life) sync(sum proto.IslandSummary, own *proto.Own) {
	if l.r == nil || l.island != sum.ID {
		*l = life{island: sum.ID, r: rand.New(rand.NewPCG(uint64(sum.ID), 0x11fe))}
	}
	l.shore = shore{geo.Outline(sum.Seed), sum.Size}
	_, _, homes := l.places(own)
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
			radius: l.patrol(p, 6+l.r.Float64()*4), speed: (0.08 + l.r.Float64()*0.08) * float64(1-2*l.r.IntN(2))}
		g.x, g.y = p.X+g.radius*math.Cos(g.angle), p.Y+g.radius*math.Sin(g.angle)
		guards = append(guards, g)
	}
	l.walkers = append(people, guards...)
}

// patrol shrinks a round around a post until the whole circle stays on the
// island: a tower by the cliff gets a tight round.
func (l *life) patrol(post geo.Vec2, radius float64) float64 {
	for ; radius > 1; radius *= 0.85 {
		ok := true
		for k := range 16 {
			a := float64(k) * math.Pi / 8
			p := geo.Vec2{X: post.X + radius*math.Cos(a), Y: post.Y + radius*math.Sin(a)}
			if l.shore.outline != nil && geo.EdgeDistance(l.shore.outline, l.shore.size, p) < cliffMargin {
				ok = false
				break
			}
		}
		if ok {
			return radius
		}
	}
	return 1
}

// step moves everyone by dt seconds. At night, villagers linger at home.
func (l *life) step(own *proto.Own, dt float64, night bool) {
	sites, works, _ := l.places(own)
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
			switch {
			case len(sites) > 0 && l.r.Float64() < 0.6:
				w.work = sites[l.r.IntN(len(sites))]
			case len(works) > 0:
				w.work = works[l.r.IntN(len(works))]
			default:
				w.work = spot{p: geo.Vec2{X: w.home.X + (l.r.Float64()*2-1)*10, Y: w.home.Y + (l.r.Float64()*2-1)*10}}
			}
			// Spread along the front, never into the footprint nor off the
			// island.
			t := l.shore.inside(geo.Vec2{X: w.work.p.X + (l.r.Float64()*2-1)*3, Y: w.work.p.Y + l.r.Float64()*1.5})
			w.tx, w.ty = t.X, t.Y
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
				t := l.shore.inside(geo.Vec2{X: w.home.X + (l.r.Float64()*2-1)*2, Y: w.home.Y + l.r.Float64()})
				w.tx, w.ty = t.X, t.Y
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

// busy reports whether the walker is at work on the spot: it then bobs, as
// if hammering or harvesting, and raises particles.
func (w *walker) busy() bool { return !w.guard && w.state == working }

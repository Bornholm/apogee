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
	answering // called to a gathering, walking there or gathered
	idling    // at leisure (leisure.go)
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
	// via is set while the walker heads for a waypoint; fx, fy is then its
	// final target.
	via    bool
	fx, fy float64
	guard  bool
	// Guards circle a post.
	post          geo.Vec2
	angle, radius float64
	speed         float64 // radians per second
	away          bool    // off its round, to fight or coming back
	// call is the gathering the villager answers, act what it does there,
	// phase offsets its movements from its neighbours'.
	call  string
	act   act
	phase float64
	// child walkers play rather than work; leisure is what an idling
	// villager does.
	child   bool
	leisure leisure
	// pace is the gait of a villager answering a gathering, or 0.
	pace float64
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
	// The places and the shore are cached until the island changes.
	sig          uint64
	sites, works []spot
	homes        []geo.Vec2
	// rally, during a battle, is where the guards run to fight.
	rally []geo.Vec2
	// calls are the gatherings of the villagers (gather.go).
	calls []gathering
	// slow makes the villagers drag their feet, during a famine.
	slow bool
	// pet is the island's dog, once it has a few homes; flock the sheep
	// by the fields (fauna.go).
	pet   *pet
	flock []grazer
}

// featureJob returns what villagers do at a natural site. Kinds come from
// the wire: an unknown one is simply idle.
func featureJob(k sim.FeatureKind) job {
	switch k {
	case sim.Grove:
		return felling
	case sim.Rock:
		return quarrying
	case sim.Crystal:
		return channel
	}
	return idle
}

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
		if !validKind(b.Kind) {
			continue // from the wire: never index with an unknown kind
		}
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
		works = append(works, front(f.X, f.Y, f.Radius, featureJob(f.Kind)))
	}
	return sites, works, homes
}

// sync matches the walkers to the island: as many villagers as the people
// (up to maxWalkers) and guards as the defenders.
func (l *life) sync(sum proto.IslandSummary, own *proto.Own, sig uint64) {
	if l.r == nil || l.island != sum.ID {
		*l = life{island: sum.ID, r: rand.New(rand.NewPCG(uint64(sum.ID), 0x11fe))}
	}
	if sig != l.sig || l.shore.outline == nil {
		// The island changed: new shore, new places. Otherwise reuse them.
		l.sig = sig
		l.shore = shore{geo.Outline(sum.Seed), sum.Size}
		l.sites, l.works, l.homes = l.places(own)
		l.keepFlock(own)
	}
	homes := l.homes
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
		people = append(people, walker{x: h.X, y: h.Y, home: h, wait: l.r.Float64() * 5,
			child: len(people)%childEvery == childEvery-1})
	}
	for len(guards) < wantGuards {
		p := posts[l.r.IntN(len(posts))]
		g := walker{guard: true, post: p, angle: l.r.Float64() * 2 * math.Pi,
			radius: l.patrol(p, 6+l.r.Float64()*4), speed: (0.08 + l.r.Float64()*0.08) * float64(1-2*l.r.IntN(2))}
		g.x, g.y = p.X+g.radius*math.Cos(g.angle), p.Y+g.radius*math.Sin(g.angle)
		guards = append(guards, g)
	}
	l.walkers = append(people, guards...)
	l.keepPet()
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
func (l *life) step(dt float64, night bool) {
	l.gather(dt)
	defer l.stepPet(dt)
	defer l.stepFlock(dt)
	sites, works := l.sites, l.works
	pace := 1.0
	if l.slow {
		pace = 0.45
	}
	for i := range l.walkers {
		w := &l.walkers[i]
		if w.guard {
			if len(l.rally) > 0 {
				// To the fight, each to an assailant, at a run.
				t := l.rally[i%len(l.rally)]
				w.tx, w.ty = t.X+float64(i%3)-1, t.Y+1.5
				w.walk(dt * 1.8)
				w.away = true
				continue
			}
			w.angle += w.speed * dt
			px := w.post.X + w.radius*math.Cos(w.angle)
			py := w.post.Y + w.radius*math.Sin(w.angle)
			if w.away {
				// Back to the round before walking it again.
				w.tx, w.ty = px, py
				w.away = !w.walk(dt * 1.8)
				continue
			}
			w.x, w.y = px, py
			continue
		}
		switch w.state {
		case answering:
			// Hurry to the gathering, then stay there.
			pace := 1.6
			if w.pace > 0 {
				pace = w.pace
			}
			if w.walk(dt*pace) && w.via {
				w.via = false
				w.tx, w.ty = w.fx, w.fy
			}
			continue
		case idling:
			l.idle(w, dt*pace)
			continue
		case resting:
			if w.wait -= dt; w.wait > 0 {
				continue
			}
			if night {
				// Nobody goes out for nothing at night, and children
				// sleep.
				if w.child {
					w.wait = 20 + l.r.Float64()*20
					continue
				}
			} else if l.unwind(i) {
				continue
			}
			// Off to work: building sites draw the most hands.
			switch {
			case len(sites) > 0 && (len(works) == 0 || l.r.Float64() < 0.6):
				w.work = sites[l.r.IntN(len(sites))]
			case len(works) > 0:
				w.work = works[l.r.IntN(len(works))]
			default:
				// Nothing to work at: some leisure instead.
				l.leisureOnly(i)
				continue
			}
			// Spread along the front, never into the footprint nor off the
			// island.
			t := l.shore.inside(geo.Vec2{X: w.work.p.X + (l.r.Float64()*2-1)*3, Y: w.work.p.Y + l.r.Float64()*1.5})
			l.route(w, t)
			w.state = going
		case going, returning:
			if w.walk(dt*pace) && w.via {
				// Reached the waypoint: on to the real target.
				w.via = false
				w.tx, w.ty = w.fx, w.fy
			} else if !w.via && w.arrived() {
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
				l.route(w, t)
				w.state = returning
			}
		}
	}
}

// route sets a walker's target. The island is not convex: a straight line
// between two points on it may cross a bay, so a path that leaves the island
// goes through a waypoint closer to the centre.
func (l *life) route(w *walker, t geo.Vec2) {
	w.via = false
	w.tx, w.ty = t.X, t.Y
	if l.shore.outline == nil || l.clear(geo.Vec2{X: w.x, Y: w.y}, t) {
		return
	}
	// Try waypoints ever closer to the centre, checking both legs. The island
	// is star-shaped around its centre (its outline is a radius per angle),
	// so the centre itself always works: both legs then run along radii.
	from := geo.Vec2{X: w.x, Y: w.y}
	mid := geo.Vec2{X: (w.x + t.X) / 2, Y: (w.y + t.Y) / 2}
	via := geo.Vec2{}
	for _, k := range []float64{0.6, 0.35, 0.15} {
		if c := l.shore.inside(mid.Scale(k)); l.clear(from, c) && l.clear(c, t) {
			via = c
			break
		}
	}
	w.via, w.fx, w.fy = true, t.X, t.Y
	w.tx, w.ty = via.X, via.Y
}

// clear reports whether the straight path between two points stays on the
// island.
func (l *life) clear(a, b geo.Vec2) bool {
	for k := 1; k < 8; k++ {
		t := float64(k) / 8
		p := geo.Vec2{X: a.X + (b.X-a.X)*t, Y: a.Y + (b.Y-a.Y)*t}
		if geo.EdgeDistance(l.shore.outline, l.shore.size, p) < 1 {
			return false
		}
	}
	return true
}

// arrived reports whether the walker stands on its target.
func (w *walker) arrived() bool { return w.x == w.tx && w.y == w.ty }

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

// gathered reports whether the villager stands at its gathering.
func (w *walker) gathered() bool {
	return w.state == answering && !w.via && w.arrived()
}

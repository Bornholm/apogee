package client

import (
	"math"

	"apogee/internal/geo"
)

// Between two tasks, the people of the island live: two villagers meet and
// talk, one sits by the door, another walks to the shore to look at the sky;
// children run after each other around the houses, and a dog follows its
// master or noses about. This is what the island shows when nothing
// happens, which is most of the time.

type leisure int

const (
	chat   leisure = iota + 1 // two villagers talk, face to face
	sit                       // by the door of one's home
	stroll                    // to the shore, to look at the sky
	play                      // children chasing each other
)

// workShare is how often a rested villager goes back to work rather than
// to some leisure; building sites call more hands.
const (
	workShare     = 0.6
	siteWorkShare = 0.75
)

// childEvery makes one villager in so many a child.
const childEvery = 5

// unwind picks what a rested villager does next, other than work: true
// when it found something, and then the walker is on its way.
func (l *life) unwind(i int) bool {
	w := &l.walkers[i]
	if w.child {
		l.startPlay(w)
		return true
	}
	share := workShare
	if len(l.sites) > 0 {
		share = siteWorkShare
	}
	if l.r.Float64() < share {
		return false
	}
	l.leisureOnly(i)
	return true
}

// leisureOnly sends an adult to some leisure: a chat, a seat by the door, a
// walk to the shore.
func (l *life) leisureOnly(i int) {
	w := &l.walkers[i]
	switch k := l.r.IntN(3); {
	case k == 0:
		if j := l.idleMate(i); j >= 0 {
			l.startChat(i, j)
			return
		}
		fallthrough
	case k == 1:
		w.leisure, w.wait = sit, 15+l.r.Float64()*20
		l.route(w, l.shore.inside(geo.Vec2{X: w.home.X + (l.r.Float64()*2-1)*1.5, Y: w.home.Y + 0.8}))
	default:
		// Out to the shore, a little inside the cliff.
		a := l.r.Float64() * 2 * math.Pi
		edge := geo.Vec2{X: math.Cos(a) * l.shore.size, Y: math.Sin(a) * l.shore.size}
		w.leisure, w.wait = stroll, 10+l.r.Float64()*12
		l.route(w, l.shore.inside(edge))
	}
	w.state = idling
}

// idleMate finds another adult at rest to talk with, or -1.
func (l *life) idleMate(i int) int {
	n := len(l.walkers)
	start := l.r.IntN(max(n, 1))
	for k := range n {
		j := (start + k) % n
		o := l.walkers[j]
		if j != i && !o.guard && !o.child && o.call == "" && o.state == resting {
			return j
		}
	}
	return -1
}

// startChat sends two villagers to meet halfway between their homes.
func (l *life) startChat(i, j int) {
	a, b := &l.walkers[i], &l.walkers[j]
	m := l.shore.inside(geo.Vec2{X: (a.x+b.x)/2 + (l.r.Float64()*2-1)*2, Y: (a.y+b.y)/2 + l.r.Float64()*2})
	wait := 12 + l.r.Float64()*16
	for k, w := range []*walker{a, b} {
		side := float64(2*k - 1)
		w.state, w.leisure, w.wait = idling, chat, wait
		w.phase = float64(k) * math.Pi // they speak in turn
		l.route(w, geo.Vec2{X: m.X + side*1.1, Y: m.Y})
	}
}

// startPlay sends a child running around its home, for a while.
func (l *life) startPlay(w *walker) {
	w.state, w.leisure, w.wait = idling, play, 20+l.r.Float64()*25
	l.playTarget(w)
}

// playTarget picks where a playing child runs next: after another child,
// or somewhere around its home.
func (l *life) playTarget(w *walker) {
	if l.r.Float64() < 0.5 {
		for _, o := range l.walkers {
			if o.child && o.state == idling && o.leisure == play && (o.x != w.x || o.y != w.y) {
				l.route(w, l.shore.inside(geo.Vec2{X: o.x + (l.r.Float64()*2 - 1), Y: o.y + (l.r.Float64()*2 - 1)}))
				return
			}
		}
	}
	a, d := l.r.Float64()*2*math.Pi, 3+l.r.Float64()*6
	l.route(w, l.shore.inside(geo.Vec2{X: w.home.X + d*math.Cos(a), Y: w.home.Y + d*0.7*math.Sin(a)}))
}

// idle advances a villager at leisure; at the end it goes home.
func (l *life) idle(w *walker, dt float64) {
	speed := dt
	if w.leisure == play {
		speed *= 1.5
	}
	arrived := w.walk(speed)
	if arrived && w.via {
		w.via = false
		w.tx, w.ty = w.fx, w.fy
		return
	}
	if w.leisure == play && !w.via && w.arrived() {
		l.playTarget(w)
	}
	if w.leisure == play || (!w.via && w.arrived()) {
		w.wait -= dt
	}
	if w.wait <= 0 {
		w.leisure = 0
		l.route(w, l.shore.inside(geo.Vec2{X: w.home.X + (l.r.Float64()*2-1)*2, Y: w.home.Y + l.r.Float64()}))
		w.state = returning
	}
}

// at reports whether a villager is at leisure and arrived where it meant
// to be: the drawing then shows what it does there.
func (w *walker) at(k leisure) bool {
	return w.state == idling && w.leisure == k && !w.via && w.arrived()
}

// pet is the dog of the island: it follows its master and, when the master
// stops, noses about around them.
type pet struct {
	x, y   float64
	tx, ty float64
	master int
	wait   float64
}

const petSpeed = 6.0

// stepPet moves the dog after its master.
func (l *life) stepPet(dt float64) {
	if l.pet == nil {
		return
	}
	p := l.pet
	if p.master >= len(l.walkers) || l.walkers[p.master].guard {
		p.master = 0
	}
	if len(l.walkers) == 0 || l.walkers[p.master].guard {
		return
	}
	m := l.walkers[p.master]
	far := math.Hypot(m.x-p.x, m.y-p.y) > 4
	if p.wait -= dt; p.wait <= 0 || far {
		// Back to heel, or off to sniff something nearby.
		a := l.r.Float64() * 2 * math.Pi
		d := 1.5 + l.r.Float64()*2.5
		if far {
			d = 1.2
		}
		t := l.shore.inside(geo.Vec2{X: m.x + d*math.Cos(a), Y: m.y + d*0.7*math.Sin(a)})
		p.tx, p.ty, p.wait = t.X, t.Y, 1+l.r.Float64()*3
	}
	dx, dy := p.tx-p.x, p.ty-p.y
	d := math.Hypot(dx, dy)
	step := petSpeed * dt
	if d <= step {
		p.x, p.y = p.tx, p.ty
		return
	}
	p.x += dx / d * step
	p.y += dy / d * step
}

// keepPet gives the island a dog once it has a few homes, by a villager.
func (l *life) keepPet() {
	if len(l.homes) < 2 {
		l.pet = nil
		return
	}
	if l.pet != nil {
		return
	}
	for i, w := range l.walkers {
		if !w.guard && !w.child {
			l.pet = &pet{x: w.x + 1, y: w.y, tx: w.x + 1, ty: w.y, master: i}
			return
		}
	}
}

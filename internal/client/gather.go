package client

import (
	"math"

	"apogee/internal/geo"
)

// Gatherings are how the people react to what happens on their island: the
// villagers drop their work and go somewhere to act together, to cheer a
// new building, pray around the heart, or hide at home before a wave. They
// make the events of the island visible in the world itself.

// act is what gathered villagers do.
type act int

const (
	cheer act = iota // they jump for joy
	pray             // they bow slowly
	hide             // they shelter at home, out of sight
	watch            // they stand and look
	dance            // they go round in a farandole
)

// gathering calls a share of the villagers to a place.
type gathering struct {
	key    string
	at     geo.Vec2
	radius float64 // they stand in a ring this wide around the place
	share  float64 // of the villagers, 0 to 1
	act    act
	// where: inPlace stops them where they stand; atHome sends each home.
	inPlace, atHome bool
	delay           float64 // seconds before it starts
	// pace is how fast they walk there, 1.6 times their gait when unset:
	// a funeral goes slowly.
	pace   float64
	left   float64 // seconds it lasts once started; 0 lasts until lifted
	lifted bool
}

// call starts a gathering, or renews the one with the same key.
func (l *life) call(g gathering) {
	for i := range l.calls {
		if l.calls[i].key == g.key {
			if !l.calls[i].lifted {
				l.calls[i].left = math.Max(l.calls[i].left, g.left)
				return
			}
			l.calls = append(l.calls[:i], l.calls[i+1:]...)
			break
		}
	}
	if len(l.calls) < 8 {
		l.calls = append(l.calls, g)
	}
}

// lift ends a lasting gathering: its villagers go back to their day.
func (l *life) lift(key string) {
	for i := range l.calls {
		if l.calls[i].key == key {
			l.calls[i].lifted = true
		}
	}
}

// gather advances the gatherings: it releases the villagers of those over,
// and calls new ones to those starting.
func (l *life) gather(dt float64) {
	kept := l.calls[:0]
	for _, g := range l.calls {
		if g.delay > 0 {
			g.delay -= dt
			kept = append(kept, g)
			continue
		}
		if !g.lifted && g.left > 0 {
			if g.left -= dt; g.left <= 0 {
				g.lifted = true
			}
		}
		if g.lifted {
			l.release(g.key)
			continue
		}
		kept = append(kept, g)
	}
	l.calls = kept
	villagers := 0
	for _, w := range l.walkers {
		if !w.guard {
			villagers++
		}
	}
	for _, g := range l.calls {
		if g.delay > 0 {
			continue
		}
		want := int(math.Ceil(g.share * float64(villagers)))
		have := 0
		for _, w := range l.walkers {
			if w.call == g.key {
				have++
			}
		}
		for i := range l.walkers {
			if have >= want {
				break
			}
			w := &l.walkers[i]
			if w.guard || w.call != "" {
				continue
			}
			l.join(w, g, have)
			have++
		}
	}
}

// join sends a villager to a gathering, to the slot-th place of its ring.
func (l *life) join(w *walker, g gathering, slot int) {
	w.call, w.act, w.phase = g.key, g.act, l.r.Float64()*6
	w.pace = g.pace
	w.via = false
	switch {
	case g.inPlace:
		w.tx, w.ty = w.x, w.y
	case g.atHome:
		l.route(w, w.home)
	default:
		a := float64(slot) * 2.39996 // the golden angle spreads the ring
		r := g.radius * (0.8 + 0.4*l.r.Float64())
		if g.act == dance {
			// One ring, everyone on it, going round.
			a, r = float64(slot)*0.55, g.radius
			w.phase, w.ring, w.ringR = a, g.at, r
		}
		l.route(w, l.shore.inside(geo.Vec2{X: g.at.X + r*math.Cos(a), Y: g.at.Y + r*0.7*math.Sin(a)}))
	}
	w.state = answering
}

// release sends the villagers of a gathering back to their day.
func (l *life) release(key string) {
	for i := range l.walkers {
		w := &l.walkers[i]
		if w.call != key {
			continue
		}
		w.call = ""
		w.state, w.wait = resting, 1+l.r.Float64()*3
	}
}

package client

import (
	"math"
	"testing"
	"time"

	"apogee/internal/sim"
)

func TestTheMilestonesAreFeasted(t *testing.T) {
	sum, own := kinIsland()
	var s signs
	s.observe(sum, own) // what was there at first look is not feasted
	if s.feasting() {
		t.Fatal("a feast at first look")
	}
	s.observe(sum, own)
	if s.feasting() {
		t.Fatal("a feast without a milestone")
	}
	own.Buildings = append(own.Buildings, sim.Building{ID: 9, Kind: sim.Library, X: -15, Progress: 1})
	s.observe(sum, own)
	if !s.feasting() || !hasCall(s.calls, "feast") {
		t.Fatal("no feast for the first library")
	}
	s.feasts.left, s.calls = 0, nil
	own.Population = 21
	s.observe(sum, own)
	if !s.feasting() {
		t.Fatal("no feast for twenty souls")
	}
	s.feasts.left = 0
	own.Age = uint64(25 * time.Hour / sim.TickDuration)
	s.observe(sum, own)
	if !s.feasting() {
		t.Fatal("no feast for the first day")
	}
}

func TestTheFarandoleGoesRound(t *testing.T) {
	l, _ := leisureIsland()
	l.call(gathering{key: "feast", at: l.homes[0], radius: feastRing, share: 1, act: dance, left: 60})
	for range 60 * 15 {
		l.step(1.0/60, false)
	}
	var w *walker
	for i := range l.walkers {
		if l.walkers[i].act == dance && l.walkers[i].gathered() {
			w = &l.walkers[i]
			break
		}
	}
	if w == nil {
		t.Fatal("nobody dances")
	}
	a0 := w.phase
	for range 60 * 3 {
		l.step(1.0/60, false)
	}
	if d := math.Hypot(w.x-w.ring.X, (w.y-w.ring.Y)/0.8); math.Abs(d-feastRing) > 0.5 {
		t.Fatalf("a dancer %.1f from the centre of the ring", d)
	}
	if w.phase <= a0 {
		t.Fatal("the farandole does not turn")
	}
}

func TestANewPeopleFeastsAnew(t *testing.T) {
	sum, own := kinIsland()
	var s signs
	own.Buildings = append(own.Buildings, sim.Building{ID: 9, Kind: sim.Library, X: -15, Progress: 1})
	own.Deaths = 5
	s.observe(sum, own)
	s.observe(sum, own)
	// The island fell; a new one rises from the clouds, its heart alone.
	sum.Seed, own.Deaths = 99, 0
	own.Buildings = own.Buildings[:1]
	s.observe(sum, own)
	if s.dead != 0 || s.feasting() {
		t.Fatalf("the new island starts with %d steles, feasting %v", s.dead, s.feasting())
	}
	own.Buildings = append(own.Buildings, sim.Building{ID: 12, Kind: sim.Library, X: 15, Y: 15, Progress: 1})
	s.observe(sum, own)
	if !s.feasting() {
		t.Fatal("the new people does not feast its first library")
	}
}

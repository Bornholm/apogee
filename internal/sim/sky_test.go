package sim

import (
	"math"
	"math/rand/v2"
	"reflect"
	"testing"
	"time"

	"apogee/internal/geo"
)

func TestDayCycle(t *testing.T) {
	var s Sky
	cases := []struct {
		at   time.Duration
		want Phase
	}{
		{0, Dawn}, {2 * time.Minute, Dawn}, {3 * time.Minute, Day},
		{17 * time.Minute, Day}, {18 * time.Minute, Dusk}, {21 * time.Minute, Night},
		{29 * time.Minute, Night}, {30 * time.Minute, Dawn}, {48 * time.Minute, Dusk},
	}
	for _, c := range cases {
		s.Tick = uint64(c.at / TickDuration)
		if got, _ := s.TimeOfDay(); got != c.want {
			t.Errorf("at %v: phase %v, want %v", c.at, got, c.want)
		}
	}
	s.Tick = uint64(10 * time.Minute / TickDuration)
	if _, left := s.TimeOfDay(); left != 8*time.Minute {
		t.Errorf("time before dusk: %v, want 8m", left)
	}
}

func TestOrbitsTurnAndInnerOnesFaster(t *testing.T) {
	var s Sky
	outer := s.AddIsland("outer", 1, 0)
	inner := s.AddIsland("inner", 2, 0)
	s.Islands[inner-1].Radius = RimRadius / 4
	for range TickRate * 60 {
		s.Step()
	}
	o, i := s.Islands[outer-1].Angle, s.Islands[inner-1].Angle
	if o <= 0 {
		t.Fatalf("outer island did not move: angle %v", o)
	}
	if math.Abs(i/o-8) > 0.01 { // (1/4)^-1.5 = 8, the descent shifts it slightly
		t.Errorf("inner/outer angular ratio %v, want 8", i/o)
	}
}

func TestStepIsDeterministic(t *testing.T) {
	run := func() Sky {
		var s Sky
		s.AddIsland("a", 1, 0.5)
		s.AddIsland("b", 2, 2.5)
		for range 5000 {
			s.Step()
		}
		return s
	}
	a, b := run(), run()
	for k := range a.Islands {
		if !reflect.DeepEqual(a.Islands[k], b.Islands[k]) {
			t.Fatalf("island %d differs: %+v vs %+v", k, a.Islands[k], b.Islands[k])
		}
	}
}

func TestNothingIsBuiltOnTheStreams(t *testing.T) {
	for seed := uint64(1); seed <= 6; seed++ {
		c := NewCiv(seed)
		rivers := geo.Rivers(seed, c.Size)
		for _, f := range c.Features {
			if d := geo.RiverDistance(rivers, geo.Vec2{X: f.X, Y: f.Y}); d < f.Radius {
				t.Fatalf("seed %d: a natural site on a stream (%.1f)", seed, d)
			}
		}
		r := rand.New(rand.NewPCG(seed, 1))
		for range 20 {
			p, ok := c.place(House, r)
			if !ok {
				break
			}
			if d := geo.RiverDistance(rivers, p); d < Specs[House].Radius {
				t.Fatalf("seed %d: a house on a stream (%.1f)", seed, d)
			}
			c.Buildings = append(c.Buildings, Building{ID: uint32(len(c.Buildings) + 10), Kind: House, X: p.X, Y: p.Y, Progress: 1})
		}
	}
}

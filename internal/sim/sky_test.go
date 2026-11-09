package sim

import (
	"math"
	"testing"
	"time"
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
	if math.Abs(i/o-8) > 1e-6 { // (1/4)^-1.5 = 8
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
		if a.Islands[k] != b.Islands[k] {
			t.Fatalf("island %d differs: %+v vs %+v", k, a.Islands[k], b.Islands[k])
		}
	}
}

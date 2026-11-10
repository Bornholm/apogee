package client

import (
	"testing"

	"apogee/internal/proto"
	"apogee/internal/sim"
)

func TestTheIslandBobsByAPixel(t *testing.T) {
	seen := map[float64]bool{}
	for i := range 1200 {
		b := bob(float64(i)/60, 3)
		if b < -1 || b > 1 {
			t.Fatalf("bob %v", b)
		}
		seen[b] = true
	}
	if len(seen) != 3 {
		t.Fatalf("the island takes %d heights in 20 s, want 3", len(seen))
	}
}

func TestWaterFallsFromTheFrontShore(t *testing.T) {
	var s scenery
	s.keep(proto.IslandSummary{ID: 1, Seed: 5, Size: 40, Radius: sim.RimRadius})
	if len(s.falls) == 0 {
		t.Fatal("no spring")
	}
	for _, f := range s.falls {
		if f.Y <= 0 {
			t.Fatalf("a spring at %+v, behind the island", f)
		}
	}
}

func TestStonesFallMoreNearTheWhirlpool(t *testing.T) {
	count := func(radius float64) int {
		var s scenery
		s.keep(proto.IslandSummary{ID: 1, Seed: 6, Size: 40, Radius: radius})
		s.falls = nil // the stones only
		var p particles
		n := 0
		for range 60 * 60 {
			before := len(p.ps)
			s.step(1.0/60, &p)
			n += len(p.ps) - before
			p.ps = p.ps[:0]
		}
		return n
	}
	rim, deep := count(sim.RimRadius), count(sim.PortalRadius)
	if deep < 5*rim || rim == 0 {
		t.Fatalf("stones in a minute: %d at the rim, %d by the whirlpool", rim, deep)
	}
}

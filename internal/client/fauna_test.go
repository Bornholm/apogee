package client

import (
	"math"
	"testing"

	"apogee/internal/geo"
	"apogee/internal/proto"
	"apogee/internal/sim"
)

func TestSheepGrazeBesideTheirField(t *testing.T) {
	l, own := leisureIsland()
	if len(l.flock) != sheepPerField {
		t.Fatalf("%d sheep by one field", len(l.flock))
	}
	for range 60 * 60 {
		l.step(1.0/60, false)
		for _, s := range l.flock {
			d := math.Hypot(s.x-s.pen.X, s.y-s.pen.Y)
			if d < sim.Specs[sim.Farm].Radius*0.8 {
				t.Fatalf("a sheep in the field, %.1f from its centre", d)
			}
		}
	}
	own.Buildings = own.Buildings[:3] // the field is gone
	l.sync(proto.IslandSummary{ID: 1, Seed: 3, Size: 50}, own, 2)
	if len(l.flock) != 0 {
		t.Fatalf("%d sheep left without a field", len(l.flock))
	}
}

func TestBirdsFleeAndComeBack(t *testing.T) {
	var b birds
	b.keep(40, 1)
	for range 60 * 10 {
		b.step(1.0/60, false)
	}
	if !b.visible() || b.away != 0 {
		t.Fatalf("calm sky: away %.2f", b.away)
	}
	for range 60 * 3 {
		b.step(1.0/60, true)
	}
	if b.visible() {
		t.Fatalf("still over the island when scared: away %.2f", b.away)
	}
}

func TestSmokeRisesFromTheHomes(t *testing.T) {
	own := &proto.Own{Buildings: []sim.Building{{ID: 2, Kind: sim.House, X: 10, Progress: 1}, {ID: 3, Kind: sim.Farm, Progress: 1}}}
	cs := islandChimneys(proto.IslandSummary{Seed: 3}, own)
	if len(cs) != 1 || cs[0].Z < 6 || (geo.Vec2{X: cs[0].X, Y: cs[0].Y}).Dist(geo.Vec2{X: 10}) > 5 {
		t.Fatalf("chimneys %+v", cs)
	}
	var p particles
	for range 60 * 5 {
		p.smoke(cs, 1.0/60)
		p.step(1.0 / 60)
	}
	if len(p.ps) == 0 {
		t.Fatal("no smoke in five seconds")
	}
}

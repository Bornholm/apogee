package client

import (
	"testing"

	"apogee/internal/geo"
	"apogee/internal/proto"
	"apogee/internal/sim"
)

func kinIsland() (proto.IslandSummary, *proto.Own) {
	return proto.IslandSummary{ID: 1, Seed: 3, Size: 50, Heart: 1}, &proto.Own{
		Population: 10, WaveInMs: -1,
		Buildings: []sim.Building{
			{ID: 1, Kind: sim.Core, Progress: 1},
			{ID: 2, Kind: sim.House, X: 15, Progress: 1},
		},
	}
}

func TestABirthGathersTheNeighbours(t *testing.T) {
	sum, own := kinIsland()
	var s signs
	own.Births = 3.5
	s.observe(sum, own) // what was before the god looked is not shown
	if hasCall(s.calls, "cradle") {
		t.Fatal("a birth shown at first look")
	}
	own.Births = 4.1
	s.observe(sum, own)
	if !hasCall(s.calls, "cradle") {
		t.Fatal("no gathering for a birth")
	}
}

func TestADeathRaisesAStele(t *testing.T) {
	sum, own := kinIsland()
	var s signs
	s.observe(sum, own)
	own.Deaths = 1.2
	s.observe(sum, own)
	if !hasCall(s.calls, "funeral") {
		t.Fatal("no funeral")
	}
	graves := s.graves(s.dead)
	if len(graves) != 1 {
		t.Fatalf("%d steles after one death", len(graves))
	}
	for _, b := range own.Buildings {
		if d := graves[0].Dist(geo.Vec2{X: b.X, Y: b.Y}); d < sim.Specs[b.Kind].Radius+2 {
			t.Fatalf("a stele %.1f from %s", d, sim.Specs[b.Kind].Name)
		}
	}
	if !geo.Inside(s.shore.outline, s.shore.size, graves[0]) {
		t.Fatal("the graveyard is off the island")
	}
	if n := len(s.graves(40)); n != maxGraves {
		t.Fatalf("%d steles shown for 40 dead", n)
	}
}

func hasCall(cs []gathering, key string) bool {
	for _, c := range cs {
		if c.key == key {
			return true
		}
	}
	return false
}

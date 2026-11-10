package client

import (
	"math/rand/v2"
	"testing"

	"apogee/internal/art"

	"apogee/internal/geo"
	"apogee/internal/proto"
	"apogee/internal/sim"
)

func TestWalkersFollowThePeopleAndStandBesideBuildings(t *testing.T) {
	own := &proto.Own{
		Population: 30, Defenders: 3,
		Buildings: []sim.Building{
			{ID: 1, Kind: sim.Core, Progress: 1},
			{ID: 2, Kind: sim.House, X: 15, Y: 0, Progress: 1},
			{ID: 3, Kind: sim.Farm, X: -15, Y: 5, Progress: 0.4},
		},
	}
	var l life
	l.sync(proto.IslandSummary{ID: 1, Seed: 3, Size: 50}, own, 1)
	people, guards := 0, 0
	for _, w := range l.walkers {
		if w.guard {
			guards++
		} else {
			people++
		}
	}
	if people != maxWalkers || guards != 3 {
		t.Fatalf("%d villagers and %d guards, want %d and 3", people, guards, maxWalkers)
	}
	for range 600 { // ten seconds of walking
		l.step(1.0/60, false)
	}
	for _, w := range l.walkers {
		if w.guard || w.state != working {
			continue
		}
		for _, b := range own.Buildings {
			c := geo.Vec2{X: b.X, Y: b.Y}
			if d := c.Dist(geo.Vec2{X: w.x, Y: w.y}); d < sim.Specs[b.Kind].Radius {
				t.Fatalf("a villager works inside the footprint of %s (%.1f from its centre)", sim.Specs[b.Kind].Name, d)
			}
		}
	}
	own.Population = 5
	l.sync(proto.IslandSummary{ID: 1, Seed: 3, Size: 50}, own, 1)
	if n := len(l.walkers) - 3; n != 5 {
		t.Fatalf("%d villagers after the people shrank to 5", n)
	}
}

func TestNobodyWalksOffTheIsland(t *testing.T) {
	for seed := uint64(1); seed <= 12; seed++ {
		nobodyWalksOff(t, seed)
	}
}

func nobodyWalksOff(t *testing.T, seed uint64) {
	// A tower right by the cliff and a grove near the rim.
	outline := geo.Outline(seed)
	edge := outline[0].Scale(50 * 0.92)
	own := &proto.Own{
		Population: 24, Defenders: 8,
		Buildings: []sim.Building{
			{ID: 1, Kind: sim.Core, Progress: 1},
			{ID: 2, Kind: sim.Tower, X: edge.X, Y: edge.Y, Progress: 1},
		},
		Features: []sim.Feature{{Kind: sim.Grove, X: edge.X * 0.9, Y: -edge.Y, Radius: 7}},
	}
	var l life
	l.sync(proto.IslandSummary{ID: 1, Seed: seed, Size: 50}, own, 1)
	for range 60 * 120 { // two minutes
		l.step(1.0/60, false)
		for _, w := range l.walkers {
			if !geo.Inside(outline, 50, geo.Vec2{X: w.x, Y: w.y}) {
				t.Fatalf("seed %d: a %s stands off the island at (%.1f, %.1f)", seed, map[bool]string{true: "guard", false: "villager"}[w.guard], w.x, w.y)
			}
		}
	}
}

func TestUnknownKindsFromTheWireAreIgnored(t *testing.T) {
	own := &proto.Own{Population: 5,
		Buildings: []sim.Building{{ID: 1, Kind: sim.Core, Progress: 1}, {ID: 2, Kind: 99, Progress: 1}, {ID: 3, Kind: -1}},
		Features:  []sim.Feature{{Kind: 42, X: 5, Radius: 3}}}
	var l life
	l.sync(proto.IslandSummary{ID: 1, Seed: 3, Size: 50}, own, 1)
	for range 600 {
		l.step(1.0/60, false)
	}
}

func TestIslandSceneIgnoresUnknownKinds(t *testing.T) {
	own := &proto.Own{Buildings: []sim.Building{
		{ID: 1, Kind: sim.Core, Progress: 1}, {ID: 2, Kind: 99, Progress: 0.5}, {ID: 3, Kind: -4, Progress: 0.2},
	}}
	sc := islandScene(art.DefaultCamera, proto.IslandSummary{ID: 1, Seed: 3, Size: 50}, own)
	if len(sc.Objects) != 1 {
		t.Fatalf("%d objects, want only the core", len(sc.Objects))
	}
}

func TestRoutesNeverLeaveTheIsland(t *testing.T) {
	for seed := uint64(1); seed <= 30; seed++ {
		var l life
		l.shore = shore{geo.Outline(seed), 50}
		r := rand.New(rand.NewPCG(seed, 1))
		for range 200 {
			a := l.shore.inside(geo.Vec2{X: (r.Float64()*2 - 1) * 50, Y: (r.Float64()*2 - 1) * 50})
			b := l.shore.inside(geo.Vec2{X: (r.Float64()*2 - 1) * 50, Y: (r.Float64()*2 - 1) * 50})
			w := walker{x: a.X, y: a.Y}
			l.route(&w, b)
			legs := [][2]geo.Vec2{{a, {X: w.tx, Y: w.ty}}}
			if w.via {
				legs = append(legs, [2]geo.Vec2{{X: w.tx, Y: w.ty}, b})
			}
			for _, leg := range legs {
				if !l.clear(leg[0], leg[1]) {
					t.Fatalf("seed %d: a leg from %v to %v crosses the cliff", seed, leg[0], leg[1])
				}
			}
		}
	}
}

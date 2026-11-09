package client

import (
	"testing"

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
	l.sync(1, own)
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
		l.step(own, 1.0/60, false)
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
	l.sync(1, own)
	if n := len(l.walkers) - 3; n != 5 {
		t.Fatalf("%d villagers after the people shrank to 5", n)
	}
}

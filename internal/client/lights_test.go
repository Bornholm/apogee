package client

import (
	"testing"
	"time"

	"apogee/internal/proto"
	"apogee/internal/sim"
)

func TestDarknessFollowsTheDay(t *testing.T) {
	cases := []struct {
		phase string
		left  time.Duration
		want  float64
	}{
		{"day", 5 * time.Minute, 0},
		{"night", 5 * time.Minute, 1},
		{"dusk", phaseLength["dusk"], 0}, // dusk begins: still daylight
		{"dusk", 0, 1},                   // dusk ends: the lights are on
		{"dawn", phaseLength["dawn"], 1},
		{"dawn", 0, 0},
	}
	for _, c := range cases {
		if got := darkness(c.phase, c.left); got != c.want {
			t.Errorf("darkness(%s, %v) = %v, want %v", c.phase, c.left, got, c.want)
		}
	}
}

func TestIslandLampsLightHousesTowersAndTheHeart(t *testing.T) {
	own := &proto.Own{Buildings: []sim.Building{
		{ID: 1, Kind: sim.Core, Progress: 1},
		{ID: 2, Kind: sim.House, X: 10, Progress: 1},
		{ID: 3, Kind: sim.Tower, X: -10, Progress: 1},
		{ID: 4, Kind: sim.House, Y: 10, Progress: 0.5}, // a site: dark
		{ID: 5, Kind: sim.Farm, Y: -10, Progress: 1},   // no windows
	}}
	kinds := map[lampKind]int{}
	for _, l := range islandLamps(proto.IslandSummary{Seed: 3, Size: 40}, own) {
		kinds[l.kind]++
		if l.kind == windowLamp && (l.x < 6 || l.x > 14) {
			t.Errorf("a window at x=%v, away from its house", l.x)
		}
	}
	if kinds[aetherLamp] != 1 || kinds[windowLamp] == 0 || kinds[brazierLamp] == 0 {
		t.Fatalf("lamps by kind: %v", kinds)
	}
}

func TestLanternsGoWithThePeopleAbout(t *testing.T) {
	people := []walker{{state: resting}, {state: going}, {guard: true}}
	if n := len(lanterns(people)); n != 2 {
		t.Fatalf("%d lanterns, want 2: none at rest at home", n)
	}
}

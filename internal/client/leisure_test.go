package client

import (
	"math"
	"testing"

	"apogee/internal/proto"
	"apogee/internal/sim"
)

func leisureIsland() (*life, *proto.Own) {
	own := &proto.Own{
		Population: 20,
		Buildings: []sim.Building{
			{ID: 1, Kind: sim.Core, Progress: 1},
			{ID: 2, Kind: sim.House, X: 15, Progress: 1},
			{ID: 3, Kind: sim.House, X: -15, Y: 5, Progress: 1},
			{ID: 4, Kind: sim.Farm, Y: 15, Progress: 1},
		},
	}
	var l life
	l.sync(proto.IslandSummary{ID: 1, Seed: 3, Size: 50}, own, 1)
	return &l, own
}

func TestTheIslandLivesBetweenTasks(t *testing.T) {
	l, _ := leisureIsland()
	children := 0
	for _, w := range l.walkers {
		if w.child {
			children++
		}
	}
	if children != 20/childEvery {
		t.Fatalf("%d children among 20 villagers", children)
	}
	seen := map[leisure]bool{}
	for range 60 * 120 { // two minutes
		l.step(1.0/60, false)
		for _, w := range l.walkers {
			if w.child && (w.state == working || w.state == going) {
				t.Fatal("a child went to work")
			}
			if w.state == idling {
				seen[w.leisure] = true
			}
		}
	}
	for _, k := range []leisure{chat, sit, stroll, play} {
		if !seen[k] {
			t.Errorf("in two minutes, nobody took leisure %d", k)
		}
	}
}

func TestTalkersFaceEachOther(t *testing.T) {
	l, _ := leisureIsland()
	for i := range l.walkers {
		l.walkers[i].state, l.walkers[i].wait = resting, 100
	}
	l.startChat(0, 1)
	for range 60 * 30 {
		l.step(1.0/60, false)
		a, b := l.walkers[0], l.walkers[1]
		if a.at(chat) && b.at(chat) {
			if d := math.Hypot(a.x-b.x, a.y-b.y); d > 3 {
				t.Fatalf("talking %.1f apart", d)
			}
			return
		}
	}
	t.Fatal("the two never met")
}

func TestTheDogStaysByItsMaster(t *testing.T) {
	l, _ := leisureIsland()
	if l.pet == nil {
		t.Fatal("no dog on an island of two houses")
	}
	for range 60 * 60 {
		l.step(1.0/60, false)
	}
	m := l.walkers[l.pet.master]
	if d := math.Hypot(m.x-l.pet.x, m.y-l.pet.y); d > 8 {
		t.Fatalf("the dog is %.1f from its master", d)
	}
}

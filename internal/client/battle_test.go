package client

import (
	"testing"

	"apogee/internal/geo"
	"apogee/internal/proto"
	"apogee/internal/sim"
)

func playBattle(t *testing.T, bt sim.Battle) (*battle, *particles) {
	t.Helper()
	own := &proto.Own{Buildings: []sim.Building{
		{ID: 1, Kind: sim.Core, Progress: 1},
		{ID: 2, Kind: sim.Tower, X: 12, Y: 4, Progress: 1},
	}}
	var b battle
	b.start(bt, shore{geo.Outline(3), 50}, own)
	if len(b.foes) != bt.Foes {
		t.Fatalf("%d assailants, want %d", len(b.foes), bt.Foes)
	}
	var dust particles
	for range int(battleTime*60) + 60 {
		b.step(1.0/60, &dust, true, nil)
	}
	if b.active() {
		t.Fatal("the battle never ends")
	}
	return &b, &dust
}

func TestARepelledWaveFallsOnTheIsland(t *testing.T) {
	b, _ := playBattle(t, sim.Battle{Tick: 10, By: sim.WaveFoe, Foes: 6, Repelled: true})
	for i, f := range b.foes {
		if !f.down {
			t.Errorf("assailant %d was not shot down", i)
		}
	}
	if b.hit {
		t.Error("the heart was struck though the island held")
	}
}

func TestAWaveThatWinsStrikesTheHeartAndLeaves(t *testing.T) {
	b, _ := playBattle(t, sim.Battle{Tick: 10, By: sim.WaveFoe, Foes: 4})
	if !b.hit {
		t.Error("the heart was never struck")
	}
	for i, f := range b.foes {
		if f.down || f.z < 10 {
			t.Errorf("assailant %d stayed on the island (z %.1f)", i, f.z)
		}
	}
}

// An attack heard of long after it happened is not played out: the god was
// away, the chronicle tells it.
func TestOldBattlesAreNotPlayedOut(t *testing.T) {
	st := &proto.State{Tick: 1000, You: 1,
		Islands: []proto.IslandSummary{{ID: 1, Seed: 3, Size: 50}},
		Own:     &proto.Own{Battle: sim.Battle{Tick: 100, Foes: 3, Repelled: true}}}
	g := &Game{}
	g.watchBattle(st)
	if g.battle.active() {
		t.Fatal("an attack 90 s old was played out")
	}
	st.Own.Battle.Tick = 995
	g.watchBattle(st)
	if !g.battle.active() {
		t.Fatal("a fresh attack was not played out")
	}
}

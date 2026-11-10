package client

import (
	"testing"

	"apogee/internal/geo"
	"apogee/internal/proto"
	"apogee/internal/sim"
)

func village(t *testing.T) *life {
	t.Helper()
	own := &proto.Own{Population: 10, Buildings: []sim.Building{
		{ID: 1, Kind: sim.Core, Progress: 1},
		{ID: 2, Kind: sim.House, X: 15, Y: 0, Progress: 1},
	}}
	var l life
	l.sync(proto.IslandSummary{ID: 1, Seed: 3, Size: 50}, own, 1)
	return &l
}

func run(l *life, seconds float64) {
	for range int(seconds * 60) {
		l.step(1.0/60, false)
	}
}

func TestVillagersAnswerAGatheringThenGoBack(t *testing.T) {
	l := village(t)
	run(l, 5)
	l.call(gathering{key: "supplique", at: geo.Vec2{}, radius: 6, share: 0.5, act: pray})
	run(l, 20)
	gathered := 0
	for _, w := range l.walkers {
		if w.gathered() && w.act == pray {
			gathered++
			if d := (geo.Vec2{X: w.x, Y: w.y}).Len(); d > 9 {
				t.Errorf("a villager prays %.1f away from the heart", d)
			}
		}
	}
	if gathered != 5 {
		t.Fatalf("%d villagers pray, want half of 10", gathered)
	}
	l.lift("supplique")
	run(l, 1)
	for _, w := range l.walkers {
		if w.call != "" || w.state == answering {
			t.Fatal("a villager stayed at a lifted gathering")
		}
	}
}

func TestATimedGatheringEndsByItself(t *testing.T) {
	l := village(t)
	l.call(gathering{key: "whisper", inPlace: true, share: 1, act: cheer, delay: 0.5, left: 2})
	run(l, 1)
	cheering := 0
	for _, w := range l.walkers {
		if w.gathered() && w.act == cheer {
			cheering++
		}
	}
	if cheering != 10 {
		t.Fatalf("%d villagers cheer, want all 10", cheering)
	}
	run(l, 3)
	if len(l.calls) != 0 {
		t.Fatalf("%d gatherings left after their time", len(l.calls))
	}
}

func TestOurWarriorsLeaveAndComeHome(t *testing.T) {
	sum := proto.IslandSummary{ID: 1, Seed: 3, Size: 50}
	own := &proto.Own{Buildings: []sim.Building{{ID: 1, Kind: sim.Core, Progress: 1}}}
	st := &proto.State{You: 1, Islands: []proto.IslandSummary{sum}}
	var tr troops
	st.Raids = []proto.RaidView{{ID: 4, From: 1, To: 2, Units: 5}}
	tr.observe(st, sum, own)
	if len(tr.marches) != 0 {
		t.Fatal("a raid already under way when the god looked was shown leaving")
	}
	st.Raids = append(st.Raids, proto.RaidView{ID: 5, From: 1, To: 2, Units: 3})
	tr.observe(st, sum, own)
	if len(tr.marches) != 1 || tr.marches[0].home || tr.marches[0].n != 3 {
		t.Fatalf("marches %+v, want one departure of 3", tr.marches)
	}
	st.Raids = st.Raids[1:] // raid 4 is back
	tr.observe(st, sum, own)
	if len(tr.marches) != 2 || !tr.marches[1].home || tr.marches[1].n != 5 {
		t.Fatalf("marches %+v, want a homecoming of 5", tr.marches)
	}
	for range int(troopTime*60) + 1 {
		tr.step(1.0 / 60)
	}
	if len(tr.marches) != 0 {
		t.Fatal("marches never end")
	}
}

package client

import (
	"math"
	"testing"

	"apogee/internal/proto"
	"apogee/internal/sim"
)

func tradeSky(tick uint64) *proto.State {
	st := &proto.State{Tick: tick}
	for i := range 8 {
		st.Islands = append(st.Islands, proto.IslandSummary{ID: proto.IslandID(i + 1), Radius: 700, Angle: float64(i) * math.Pi / 4})
	}
	return st
}

func TestTradersFlyBetweenNeighbours(t *testing.T) {
	spell := uint64(tradeSpell / sim.TickDuration)
	flying, spells := 0, 0
	for n := range uint64(40) {
		ts := trades(tradeSky(n*spell + 10))
		if len(ts) > 0 {
			spells++
		}
		for _, tr := range ts {
			flying++
			d := int(tr.to) - int(tr.from)
			if d != 1 && d != -1 && d != 7 && d != -7 {
				t.Fatalf("a trader from %d to %d, not a neighbour", tr.from, tr.to)
			}
		}
		if trades(tradeSky(n*spell+uint64(tradeFlight/sim.TickDuration)+5)) != nil {
			t.Fatal("traders still flying after their flight")
		}
	}
	if spells < 30 || flying < 40 {
		t.Fatalf("%d traders over %d spells in 40", flying, spells)
	}
}

func TestNoTradeWithAnEnemy(t *testing.T) {
	st := tradeSky(10)
	for i := range st.Islands {
		st.Islands[i].Banner = sim.Annihilate
		st.Islands[i].BannerTarget = st.Islands[(i+1)%8].ID
		st.Islands[(i+1)%8].BannerTarget = st.Islands[i].ID
	}
	for n := range uint64(20) {
		st.Tick = n*uint64(tradeSpell/sim.TickDuration) + 10
		for _, tr := range trades(st) {
			for _, is := range st.Islands {
				if is.ID == tr.from && is.BannerTarget == tr.to {
					t.Fatalf("a trader flies from %d to its enemy %d", tr.from, tr.to)
				}
			}
		}
	}
}

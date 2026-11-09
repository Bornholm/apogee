package client

import (
	"testing"

	"apogee/internal/sim"
)

func TestAdjustKeepsThePieWhole(t *testing.T) {
	g := &Game{draft: sim.DefaultPie} // Prosper 60, Fortify 40
	g.adjust(sim.Study, +10)()
	if g.draft[sim.Study] != 10 || g.draft[sim.Prosper] != 50 || !g.drafting {
		t.Fatalf("after +10 Study: %v", g.draft)
	}
	if err := g.draft.Validate(); err != nil {
		t.Fatal(err)
	}
	g.adjust(sim.Fortify, -10)()
	if g.draft[sim.Fortify] != 30 || g.draft.Validate() != nil {
		t.Fatalf("after -10 Fortify: %v", g.draft)
	}
}

func TestAdjustRefusesImpossibleMoves(t *testing.T) {
	g := &Game{draft: sim.Pie{sim.Prosper: 40, sim.Fortify: 30, sim.Study: 30}}
	if g.adjust(sim.Arm, +10) != nil {
		t.Error("a fourth slice was allowed")
	}
	if g.adjust(sim.Arm, -10) != nil {
		t.Error("an empty slice went below zero")
	}
	g = &Game{draft: sim.Pie{sim.Study: 100}}
	if g.adjust(sim.Study, -10) != nil {
		t.Error("a single slice gave 10 % to nobody")
	}
	// Room is made from the other side: + on another intention takes it.
	g.adjust(sim.Prosper, +10)()
	if g.draft != (sim.Pie{sim.Study: 90, sim.Prosper: 10}) {
		t.Errorf("after +10 Prosper: %v", g.draft)
	}
}

func TestWrapKeepsFrenchPunctuationWithItsWord(t *testing.T) {
	p := painter{k: 1}
	text := "Sans signe de son dieu, ton peuple a choisi : « Charrue : Vivres +30 % »."
	for w := 60; w < 400; w += 7 {
		for _, l := range p.wrap(text, w) {
			if l == "«" || l == "»." || l == "%" || l == ":" || l[0] == ':' {
				t.Fatalf("width %d: a line holds punctuation alone: %q", w, l)
			}
		}
	}
}

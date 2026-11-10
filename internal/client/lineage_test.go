package client

import (
	"testing"

	"apogee/internal/art"
	"apogee/internal/sim"
)

// The simulation bounds a lineage with its own constants; the art draws it
// from its tables: they must agree, or a valid lineage would panic.
func TestTheLineageBoundsMatchTheArt(t *testing.T) {
	if len(art.LineageColours) != sim.NumColours || len(art.ColourNames) != sim.NumColours {
		t.Errorf("%d colours in the art, %d in the sky", len(art.LineageColours), sim.NumColours)
	}
	if len(art.Emblems) != sim.NumEmblems || len(art.EmblemNames) != sim.NumEmblems {
		t.Errorf("%d emblems in the art, %d in the sky", len(art.Emblems), sim.NumEmblems)
	}
	if int(art.NumStyles) != sim.NumStyles {
		t.Errorf("%d styles in the art, %d in the sky", art.NumStyles, sim.NumStyles)
	}
}

// A lineage read from a settings file is bounded before it is drawn.
func TestALineageFromTheSettingsIsBounded(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	a := NewApp(AppOptions{})
	a.settings.Lineage = &sim.Lineage{Colour: 99, Emblem: -4, Style: 7}
	a.openLineage()
	if a.editing.Colour != 0 || a.editing.Emblem != 0 || a.editing.Style != 0 {
		t.Fatalf("editing %+v", a.editing)
	}
}

// Closing the book gives the name up: the shortcuts work again.
func TestClosingTheBookGivesUpTheName(t *testing.T) {
	g := &Game{book: civTab}
	g.startRenaming("Les Cendres")
	g.book = noBook
	g.dropHiddenName()
	if g.renaming {
		t.Fatal("still naming with the book closed")
	}
}

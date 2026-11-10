package sim

import (
	"strings"
	"testing"
)

func TestALineageIsBoundedAndKept(t *testing.T) {
	l := Lineage{Name: "  Les\tCendres" + strings.Repeat("x", 60), Colour: 42, Emblem: -3, Style: 9}.Clean()
	if l.Colour != 0 || l.Emblem != 0 || l.Style != 0 {
		t.Fatalf("out of bounds kept: %+v", l)
	}
	if strings.ContainsRune(l.Name, '\t') || len([]rune(l.Name)) > MaxName {
		t.Fatalf("name %q", l.Name)
	}
	var s Sky
	drawn := s.LineageOf("someone")
	if drawn != s.LineageOf("someone") {
		t.Fatal("the drawn lineage changes")
	}
	s.SetLineage("someone", Lineage{Name: "Les Cendres", Colour: 5, Emblem: 3, Style: 1})
	if got := s.LineageOf("someone"); got.Colour != 5 || got.Name != "Les Cendres" {
		t.Fatalf("lineage %+v", got)
	}
}

func TestCivilisationsAreNamedAfterTheirLand(t *testing.T) {
	var s Sky
	id := s.Found("god", "Dieu", 9, 0)
	is := s.Island(id)
	if is.CivName == "" || !strings.HasPrefix(is.CivName, "Les ") {
		t.Fatalf("named %q", is.CivName)
	}
	names := map[string]bool{}
	for n := range 20 {
		names[CivName(is.Civ.Trait, 9, n)] = true
	}
	if len(names) < 5 {
		t.Fatalf("only %d names in 20 draws", len(names))
	}
	if !s.Rename("god", " Les Enfants du Cristal ") || s.Island(id).CivName != "Les Enfants du Cristal" {
		t.Fatalf("renamed %q", s.Island(id).CivName)
	}
	if s.Rename("god", "   ") {
		t.Fatal("an empty name was taken")
	}
}

package sim

import (
	"testing"
)

func TestEveryTraitIsDrawn(t *testing.T) {
	seen := map[Trait]int{}
	for seed := range uint64(600) {
		tr := TraitOf(seed)
		if tr <= NoTrait || tr >= NumTraits {
			t.Fatalf("seed %d drew trait %d", seed, tr)
		}
		seen[tr]++
	}
	for tr := NoTrait + 1; tr < NumTraits; tr++ {
		if seen[tr] < 50 {
			t.Errorf("%s drawn %d times in 600", tr.Name(), seen[tr])
		}
	}
}

func TestTraitsShapeTheLand(t *testing.T) {
	count := func(c Civ, k FeatureKind) int {
		n := 0
		for _, f := range c.Features {
			if f.Kind == k {
				n++
			}
		}
		return n
	}
	for seed := range uint64(200) {
		c := NewCiv(seed)
		switch c.Trait {
		case CrystalIsle:
			if count(c, Crystal) < 4 {
				t.Errorf("a crystal isle with %d crystals", count(c, Crystal))
			}
		case ForestIsle:
			if count(c, Grove) < 4 {
				t.Errorf("a forest isle with %d groves", count(c, Grove))
			}
		case HotSpring:
			if count(c, Spring) != 1 {
				t.Errorf("a hot spring isle with %d springs", count(c, Spring))
			}
		default:
			if count(c, Spring) != 0 {
				t.Errorf("a %s with a spring", c.Trait.Name())
			}
		}
	}
}

func TestTraitsChangeTheRules(t *testing.T) {
	base := NewCiv(1)
	base.Trait = NoTrait
	high, fertile := base, base
	high.Trait, fertile.Trait = HighIsle, FertileIsle
	if high.DescentSpeed() >= base.DescentSpeed() {
		t.Error("a high isle sinks as fast")
	}
	if high.ExpandCost()[Wood] <= base.ExpandCost()[Wood] {
		t.Error("a high isle grows as cheaply")
	}
	if fertile.Defense() >= base.Defense() {
		t.Error("fertile lands defend as well")
	}
	if fertile.Mods().Prod[Food] <= 0 {
		t.Error("fertile lands grow no more food")
	}
}

package sim

import (
	"math/rand/v2"
	"testing"
	"time"
)

func fortress() Civ {
	c := NewCiv(1)
	c.Trait = NoTrait
	c.Defenders = 3
	c.Buildings = append(c.Buildings,
		Building{ID: 10, Kind: Tower, Progress: 1}, Building{ID: 11, Kind: Tower, Progress: 1},
		Building{ID: 12, Kind: Library, Progress: 1}, Building{ID: 13, Kind: Farm, Progress: 1})
	return c
}

func TestEachSpeciesMeetsADifferentDefense(t *testing.T) {
	c := fortress()
	base := c.defenseAgainst(Monsters)
	if d := c.defenseAgainst(Monsters); d != c.Defense() {
		t.Fatalf("the classic wave meets %v, not the defense %v", d, c.Defense())
	}
	if c.defenseAgainst(Swarm) >= base {
		t.Error("towers hold a swarm as well as a wave")
	}
	if c.defenseAgainst(Shades) <= base {
		t.Error("the scholars' wards do not hold the shades better")
	}
	// Without towers, a swarm meets more: the defenders count more.
	c.Buildings = c.Buildings[:1]
	if c.defenseAgainst(Swarm) <= c.defenseAgainst(Monsters) {
		t.Error("defenders do not count more against a swarm")
	}
}

func TestSpeciesRavageTheirWay(t *testing.T) {
	c := fortress()
	c.Stock[Food] = 100
	share, _ := c.ravage(Devourers)
	if share >= 1 || c.Stock[Food] > 41 {
		t.Errorf("devourers: blow share %v, food %v", share, c.Stock[Food])
	}
	towers := func() int {
		n := 0
		for _, b := range c.Buildings {
			if b.Kind == Tower {
				n++
			}
		}
		return n
	}
	before := towers()
	c.ravage(Breakers)
	if towers() != before-1 {
		t.Errorf("breakers left %d towers of %d", towers(), before)
	}
}

func TestShadesComeOnlyByNight(t *testing.T) {
	var s Sky
	r := rand.New(rand.NewPCG(1, 2))
	for range 500 {
		in := r.Float64() * float64(DayLength/time.Second)
		sp := s.drawSpecies(r, in)
		if sp == Shades && phaseAt(time.Duration(in*float64(time.Second))) != Night {
			t.Fatal("shades drawn for a wave by day")
		}
	}
}

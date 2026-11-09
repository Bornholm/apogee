package sim

import (
	"math"
	"testing"

	"apogee/internal/geo"
)

// run steps a civilisation alone for the given simulated duration, in days
// with no night.
func run(c *Civ, seconds float64) {
	dt := TickDuration.Seconds()
	for t := 0; t < int(seconds/dt); t++ {
		c.Step(dt, Day, uint64(t))
	}
}

func TestPieRules(t *testing.T) {
	valid := []Pie{DefaultPie, {Study: 100}, {Prosper: 30, Fortify: 30, Arm: 40}}
	for _, p := range valid {
		if err := p.Validate(); err != nil {
			t.Errorf("%v: unexpected error %v", p, err)
		}
	}
	invalid := []Pie{
		{Prosper: 50},            // not 100 %
		{Prosper: 55, Study: 45}, // not by 10 %
		{Prosper: 40, Expand: 20, Fortify: 20, Arm: 20}, // four slices
		{Prosper: 110, Study: -10},
	}
	for _, p := range invalid {
		if p.Validate() == nil {
			t.Errorf("%v: accepted an invalid pie", p)
		}
	}
}

func TestConvictionFollowsThePieSlowly(t *testing.T) {
	c := NewCiv(1)
	if err := c.SetPie(Pie{Study: 100}); err != nil {
		t.Fatal(err)
	}
	run(&c, 10)
	if c.Conviction[Study] > 0.2 {
		t.Errorf("conviction jumped to %.2f in 10 s: the people must not change their mind at once", c.Conviction[Study])
	}
	run(&c, 600)
	if c.Conviction[Study] < 0.9 || c.Conviction[Prosper] > 0.1 {
		t.Errorf("after 10 min, conviction study %.2f prosper %.2f", c.Conviction[Study], c.Conviction[Prosper])
	}
}

func TestDefaultCivilisationGrows(t *testing.T) {
	c := NewCiv(7)
	pop := c.Population
	run(&c, 4*3600)
	if c.Population <= pop {
		t.Errorf("population %v after an hour, started at %v", c.Population, pop)
	}
	if c.Count(Farm) == 0 || c.Count(House) == 0 {
		t.Errorf("a prosperous people built %d farms and %d houses", c.Count(Farm), c.Count(House))
	}
	if len(c.Chronicle) == 0 {
		t.Error("nothing was written in the chronicle")
	}
}

func TestIntentionsShapeWhatIsBuilt(t *testing.T) {
	cases := []struct {
		pie  Pie
		kind Kind
	}{
		{Pie{Fortify: 70, Prosper: 30}, Tower},
		{Pie{Study: 70, Prosper: 30}, Library},
		{Pie{Arm: 70, Prosper: 30}, Barracks},
	}
	for _, tc := range cases {
		c := NewCiv(3)
		c.SetPie(tc.pie)
		run(&c, 8*3600)
		if c.Count(tc.kind) == 0 {
			t.Errorf("pie %v built no %s: %+v", tc.pie, Specs[tc.kind].Name, kinds(&c))
		}
	}
}

func kinds(c *Civ) map[string]int {
	m := map[string]int{}
	for _, b := range c.Buildings {
		m[Specs[b.Kind].Name]++
	}
	return m
}

func TestBuildingsNeverOverlapNorLeaveTheIsland(t *testing.T) {
	c := NewCiv(11)
	c.SetPie(Pie{Prosper: 40, Expand: 30, Fortify: 30})
	run(&c, 4*3600)
	outline := geo.Outline(c.Seed)
	for i, a := range c.Buildings {
		pa := geo.Vec2{X: a.X, Y: a.Y}
		ra := Specs[a.Kind].Radius
		if !geo.Inside(outline, c.Size, pa) {
			t.Errorf("%s at %v is off the island", Specs[a.Kind].Name, pa)
		}
		for _, b := range c.Buildings[i+1:] {
			if d := pa.Dist(geo.Vec2{X: b.X, Y: b.Y}); d < ra+Specs[b.Kind].Radius {
				t.Errorf("%s and %s overlap (%.1f apart)", Specs[a.Kind].Name, Specs[b.Kind].Name, d)
			}
		}
		for _, f := range c.Features {
			if d := pa.Dist(geo.Vec2{X: f.X, Y: f.Y}); d < ra+f.Radius {
				t.Errorf("%s stands on a natural site (%.1f apart)", Specs[a.Kind].Name, d)
			}
		}
	}
}

func TestSurvivalOverridesTheGodsWishes(t *testing.T) {
	for _, p := range []Pie{{Fortify: 100}, {Study: 100}, {Arm: 100}} {
		c := NewCiv(8)
		c.SetPie(p)
		for i := range c.Conviction { // the people already share the wish
			c.Conviction[i] = float64(p[i]) / 100
		}
		run(&c, 2*3600)
		if c.Count(Farm) == 0 {
			t.Errorf("pie %v: no farm in two hours, the people starve", p)
		}
		if c.Population < 6 {
			t.Errorf("pie %v: population fell to %.1f", p, c.Population)
		}
	}
}

func TestAnUnaffordableExpansionDoesNotStopBuilding(t *testing.T) {
	c := NewCiv(8)
	c.SetPie(Pie{Expand: 100})
	for i := range c.Conviction {
		c.Conviction[i] = float64(c.Pie[i]) / 100
	}
	// Enough for a house, not for an expansion; the people are not cramped,
	// so expanding scores higher than the house.
	c.Stock = Stock{Wood: 90, Stone: 25, Food: 60}
	c.Population = 2
	if c.expandScore() <= c.score(House) {
		t.Fatalf("setup: expand %.2f must outscore a house %.2f", c.expandScore(), c.score(House))
	}
	before := len(c.Buildings)
	c.decide(0)
	if len(c.Buildings) == before {
		t.Fatal("the people waited for an expansion instead of building a house")
	}
}

func TestFamineShrinksThePeople(t *testing.T) {
	c := NewCiv(5)
	c.Stock[Food] = 0
	c.Population = 40
	c.Buildings = c.Buildings[:1] // only the core: far too little food
	pop := c.Population
	run(&c, 120)
	if c.Population >= pop {
		t.Errorf("population %v after two minutes of famine, started at %v", c.Population, pop)
	}
}

func TestStockNeverExceedsTheCap(t *testing.T) {
	c := NewCiv(9)
	run(&c, 2*3600)
	for r, v := range c.Stock {
		if v > c.Cap()+1e-9 || v < 0 || math.IsNaN(v) {
			t.Errorf("%s: %v with a cap of %v", Resource(r).Name(), v, c.Cap())
		}
	}
}

func TestCultureFollowsTheConvictionOverHours(t *testing.T) {
	c := NewCiv(2)
	if c.Dominant() != Industry {
		t.Fatalf("a newborn people with Prosper 60 is not industrious: %v", c.Dominant())
	}
	c.SetPie(Pie{Arm: 100})
	run(&c, 600)
	if c.Dominant() == Weapons {
		t.Error("the culture changed in ten minutes: it must take hours")
	}
	run(&c, 8*3600)
	if c.Dominant() != Weapons {
		t.Errorf("after eight hours of S'armer, dominant culture is %s", c.Dominant().Name())
	}
}

func TestHungerBuildsAnAffordableFarmBeforeAnUnaffordableTower(t *testing.T) {
	c := NewCiv(8)
	c.SetPie(Pie{Fortify: 100})
	for i := range c.Conviction {
		c.Conviction[i] = float64(c.Pie[i]) / 100
	}
	c.Stock = Stock{Wood: 70, Stone: 10} // a farm, not a tower
	c.Population = 20                    // far more mouths than the core feeds
	if c.score(Tower) <= c.score(Farm) {
		t.Fatalf("setup: tower %.2f must outscore the farm %.2f", c.score(Tower), c.score(Farm))
	}
	c.decide(0)
	if c.Count(Farm) == 0 {
		t.Fatal("a starving people waited for a tower instead of building a farm")
	}
}

func TestAgreedExpansionIsKeptOnceAffordable(t *testing.T) {
	c := NewCiv(8)
	c.Stock = Stock{Food: 80} // fed, but no wood nor stone to expand
	c.Crowded = true
	c.SinceSupplique = 3 * suppliqueGap
	run(&c, 1)
	if c.Pending == nil || c.Pending.Kind != AskRoom {
		t.Fatal("no room supplique")
	}
	c.Answer(c.Pending.ID, 0, 0)
	if !c.ExpandPledge || c.Expanding > 0 {
		t.Fatalf("pledge %v, expanding %v: the people cannot pay yet", c.ExpandPledge, c.Expanding)
	}
	c.Stock = Stock{Wood: 200, Stone: 200, Food: 100}
	c.decide(1)
	if c.Expanding == 0 || c.ExpandPledge {
		t.Fatal("the promised expansion did not start once affordable")
	}
}

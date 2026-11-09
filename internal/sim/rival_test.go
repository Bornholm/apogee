package sim

import "testing"

func TestToPieAlwaysGivesAValidPie(t *testing.T) {
	cases := [][NumIntentions]float64{
		{Prosper: 1},
		{Prosper: 33, Expand: 33, Fortify: 33},
		{Prosper: 10, Expand: 20, Fortify: 30, Arm: 40, Study: 50},
		{Arm: 0.4, Fortify: 0.35, Study: 0.25},
		{Prosper: 97, Study: 3},
	}
	for _, w := range cases {
		if err := toPie(w).Validate(); err != nil {
			t.Errorf("%v → %v: %v", w, toPie(w), err)
		}
	}
}

func TestRivalsShapeTheirCultureByProfile(t *testing.T) {
	var s Sky
	war := s.FoundRival("Morgal", Warlike, 21, 0)
	build := s.FoundRival("Aubépine", Builder, 22, 3)
	steps(&s, 10*3600)
	if f := s.Island(war).Civ.Dominant(); f != Weapons {
		t.Errorf("a warlike god's people revere %s", f.Name())
	}
	if f := s.Island(build).Civ.Dominant(); f != Industry {
		t.Errorf("a builder god's people revere %s", f.Name())
	}
	if !s.Island(war).IsAI() {
		t.Error("a rival island does not say it is an AI")
	}
}

func TestRivalsAnswerTheirPeople(t *testing.T) {
	var s Sky
	id := s.FoundRival("Ysolde", Cautious, 23, 0)
	s.Island(id).Civ.Tech.Progress = tierCost[0] - 0.01
	steps(&s, rivalInterval+10)
	if p := s.Island(id).Civ.Pending; p != nil && p.Kind == AskDiscovery {
		t.Error("the AI god left its scholars waiting")
	}
}

func TestRivalsAreDeterministic(t *testing.T) {
	run := func() Pie {
		var s Sky
		id := s.FoundRival("Corvin", Opportunist, 24, 0)
		steps(&s, 2*3600)
		return s.Island(id).Civ.Pie
	}
	if a, b := run(), run(); a != b {
		t.Fatalf("two runs gave %v and %v", a, b)
	}
}

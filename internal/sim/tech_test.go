package sim

import (
	"strings"
	"testing"
)

// untilPending steps until a supplique of the kind waits, or fails.
func untilPending(t *testing.T, c *Civ, kind SuppliqueKind, maxSeconds float64) {
	t.Helper()
	dt := TickDuration.Seconds()
	for i := 0; i < int(maxSeconds/dt); i++ {
		c.Step(dt, Day, uint64(i))
		if c.Pending != nil && c.Pending.Kind == kind {
			return
		}
	}
	t.Fatalf("no supplique of kind %d after %.0f s", kind, maxSeconds)
}

func TestScholarsAskBetweenTwoDiscoveries(t *testing.T) {
	c := NewCiv(4)
	c.Tech.Progress = tierCost[0] - 0.01
	untilPending(t, &c, AskDiscovery, 60)
	s := c.Pending
	if s.Axis != Industry || s.Tier != 0 || len(s.Options) != 2 {
		t.Fatalf("unexpected supplique %+v", s)
	}
	if err := c.Answer(s.ID, 1, 0); err != nil {
		t.Fatal(err)
	}
	if c.Tech.Chosen[Industry][0] != 1 || c.Pending != nil {
		t.Fatalf("answer not applied: chosen %d, pending %v", c.Tech.Chosen[Industry][0], c.Pending)
	}
	if got := c.Tech.Mods().Prod[Wood]; got != 0.3 {
		t.Errorf("Scie should give wood +30 %%, got %v", got)
	}
}

func TestUnansweredSuppliqueIsSettledByThePeople(t *testing.T) {
	c := NewCiv(4)
	c.Tech.Progress = tierCost[0] - 0.01
	untilPending(t, &c, AskDiscovery, 60)
	run(&c, suppliqueTimeout+1)
	if c.Pending != nil && c.Pending.Kind == AskDiscovery {
		t.Fatal("the supplique outlived its timeout")
	}
	if c.Tech.Tier(Industry) != 1 {
		t.Fatal("the people did not choose a discovery themselves")
	}
	last := ""
	for _, e := range c.Chronicle {
		if strings.Contains(e.Text, "Sans signe de son dieu") {
			last = e.Text
		}
	}
	if last == "" {
		t.Error("the chronicle does not say the people chose alone")
	}
}

func TestAnswerRejectsStaleOrInvalidReplies(t *testing.T) {
	c := NewCiv(4)
	if c.Answer(1, 0, 0) == nil {
		t.Error("answered a supplique that does not exist")
	}
	c.Tech.Progress = tierCost[0] - 0.01
	untilPending(t, &c, AskDiscovery, 60)
	if c.Answer(c.Pending.ID+1, 0, 0) == nil {
		t.Error("answered with a wrong id")
	}
	if c.Answer(c.Pending.ID, 2, 0) == nil {
		t.Error("answered with an option out of range")
	}
}

func TestScholarsWaitForTheChoice(t *testing.T) {
	c := NewCiv(4)
	c.Tech.Progress = tierCost[0] - 0.01
	untilPending(t, &c, AskDiscovery, 60)
	run(&c, 60)
	if c.Tech.Progress != 0 {
		t.Errorf("knowledge piled up (%v) while a discovery waited", c.Tech.Progress)
	}
}

func TestTheCapForgetsTheNeglectedAxis(t *testing.T) {
	c := NewCiv(4)
	c.Tech.Chosen[Industry] = [TiersPerAxis]int8{0, 0}
	c.Tech.Chosen[Walls][0] = 0
	c.Culture = [NumFamilies]float64{Weapons: 0.9, Walls: 0.05, Industry: 0.3}
	c.Tech.Progress = tierCost[0] - 0.01
	untilPending(t, &c, AskDiscovery, 60)
	if c.Pending.Axis != Weapons {
		t.Fatalf("scholars follow %s, want Armes", c.Pending.Axis.Name())
	}
	if c.Tech.Tier(Walls) != 0 {
		t.Error("the least cherished axis (Murailles) was not forgotten")
	}
	if c.Tech.Tier(Industry) != 2 {
		t.Error("a cherished axis was forgotten")
	}
	c.Answer(c.Pending.ID, 0, 0)
	if c.Tech.Total() > TechCap {
		t.Errorf("%d tiers mastered, cap is %d", c.Tech.Total(), TechCap)
	}
}

func TestDominantFamilyBuildsCheaper(t *testing.T) {
	c := NewCiv(4) // Industry dominates
	if got, full := c.Cost(House)[Wood], Specs[House].Cost[Wood]*CostScale; got >= full {
		t.Errorf("house costs %v wood, full price %v: no culture bonus", got, full)
	}
	if got, full := c.Cost(Tower)[Stone], Specs[Tower].Cost[Stone]*CostScale; got != full {
		t.Errorf("tower costs %v stone, want the full %v", got, full)
	}
}

func TestRefusingRoomDelaysTheNextAsk(t *testing.T) {
	c := NewCiv(4)
	c.Crowded = true
	c.SinceSupplique = 3 * suppliqueGap
	run(&c, 1)
	if c.Pending == nil || c.Pending.Kind != AskRoom {
		t.Fatal("a crowded people did not ask for room")
	}
	c.Answer(c.Pending.ID, 1, 0)
	first := c.RoomCooldown
	c.Crowded = true
	c.RoomCooldown = 0
	c.SinceSupplique = 3 * suppliqueGap
	run(&c, 1)
	c.Answer(c.Pending.ID, 1, 0)
	if c.RoomCooldown <= first {
		t.Errorf("second refusal cooldown %v, first %v: it must grow", c.RoomCooldown, first)
	}
}

func TestNoExpansionPledgeBeyondTheMaximum(t *testing.T) {
	c := NewCiv(4)
	c.Crowded = true
	c.SinceSupplique = 3 * suppliqueGap
	c.Stock[Food] = 80
	run(&c, 1)
	if c.Pending == nil || c.Pending.Kind != AskRoom {
		t.Fatal("no room supplique")
	}
	c.Size = maxSize // the island reached its full extent meanwhile
	c.Answer(c.Pending.ID, 0, 0)
	if c.ExpandPledge {
		t.Fatal("a pledge was made for an island that cannot grow")
	}
	c.ExpandPledge = true // even a stale pledge must not be paid
	c.Stock = Stock{Wood: 200, Stone: 200, Food: 100}
	c.decide(1)
	if c.Expanding > 0 || c.ExpandPledge {
		t.Fatal("an expansion was paid for an island at its maximum")
	}
}

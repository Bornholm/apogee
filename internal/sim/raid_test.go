package sim

import (
	"strings"
	"testing"
)

// twoIslands returns a sky with an attacker and a target, both old enough
// to be fair game, close to each other.
func twoIslands(t *testing.T) (*Sky, *Island, *Island) {
	t.Helper()
	s := &Sky{}
	a := s.Found("a", "Ada", 1, 0)
	b := s.Found("b", "Bob", 2, 0.2)
	// Fast-forward the clock past the newcomers' shield without simulating.
	s.Tick = uint64(newcomerShield/TickDuration.Seconds()) + 1
	return s, s.Island(a), s.Island(b)
}

func untilNoRaids(s *Sky, max float64) {
	for i := 0; i < int(max/TickDuration.Seconds()) && (len(s.Raids) > 0 || anyOuter(s)); i++ {
		s.raids(TickDuration.Seconds())
		s.Tick++
	}
}

func anyOuter(s *Sky) bool {
	for _, is := range s.Islands {
		if is.Civ.Outer.Kind != NoOuter {
			return true
		}
	}
	return false
}

func TestPunishPillagesAtMostAQuarter(t *testing.T) {
	s, a, b := twoIslands(t)
	a.Civ.Assailants = 50
	b.Civ.Defenders = 0
	b.Civ.Buildings = b.Civ.Buildings[:1]
	b.Civ.Stock = Stock{Wood: 400, Stone: 400, Food: 400, Aether: 400}
	a.Civ.Stock = Stock{}
	if err := s.Whisper(a.ID, Punish, b.ID); err != nil {
		t.Fatal(err)
	}
	untilNoRaids(s, 3600)
	for r, v := range b.Civ.Stock {
		if v < 400*(1-pillageShare)-1e-6 {
			t.Errorf("%s: %v left of 400, more than a quarter was taken", Resource(r).Name(), v)
		}
	}
	if a.Civ.Stock[Wood] <= 0 {
		t.Error("the raiders came home empty-handed")
	}
	if a.Civ.Assailants <= 0 {
		t.Error("no raider came home")
	}
}

func TestAShieldFollowsAPillage(t *testing.T) {
	s, a, b := twoIslands(t)
	a.Civ.Assailants = 50
	b.Civ.Defenders = 0
	s.Whisper(a.ID, Punish, b.ID)
	untilNoRaids(s, 3600)
	if err := s.Whisper(a.ID, Punish, b.ID); err != ErrShielded {
		t.Fatalf("second raid right after a pillage: %v, want ErrShielded", err)
	}
}

func TestNewcomersAreProtectedUntilTheyRaid(t *testing.T) {
	s := &Sky{}
	a := s.Found("a", "Ada", 1, 0)
	b := s.Found("b", "Bob", 2, 1)
	if err := s.Whisper(a, Punish, b); err != ErrProtected {
		t.Fatalf("raiding a newborn: %v, want ErrProtected", err)
	}
	// Helping a neighbour is always allowed, and is no aggression.
	if err := s.Whisper(b, Rescue, a); err != nil {
		t.Fatalf("rescuing is always allowed: %v", err)
	}
	if s.Island(b).Aggressor {
		t.Error("rescuing made Bob an aggressor")
	}
}

func TestAnnihilationOnlyAgainstTheDeclining(t *testing.T) {
	s, a, b := twoIslands(t)
	if err := s.Whisper(a.ID, Annihilate, b.ID); err != ErrNotDeclining {
		t.Fatalf("annihilating a healthy island: %v", err)
	}
	b.Declining = true
	if err := s.Whisper(a.ID, Annihilate, b.ID); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(allText(b.Civ.Chronicle), "anéantissement") {
		t.Error("the target was not warned")
	}
}

func TestAnnihilationCanBreakTheHeart(t *testing.T) {
	s, a, b := twoIslands(t)
	b.Declining = true
	b.Civ.Defenders = 0
	b.Civ.Buildings = b.Civ.Buildings[:1]
	b.Civ.CoreHealth = 0.1
	a.Civ.Assailants = 80
	s.Whisper(a.ID, Annihilate, b.ID)
	untilNoRaids(s, 3*3600)
	if b.Falling <= 0 {
		t.Fatalf("heart at %.2f, island not falling", b.Civ.CoreHealth)
	}
	if !strings.Contains(b.Cause, "Ada") {
		t.Errorf("cause %q does not name the destroyer", b.Cause)
	}
}

func TestRescueStrengthensTheNeighbour(t *testing.T) {
	s, a, b := twoIslands(t)
	a.Civ.Defenders = 12
	before := b.Civ.Defense() + s.relief(b.ID)
	s.Whisper(a.ID, Rescue, b.ID)
	for range int(600 / TickDuration.Seconds()) {
		s.raids(TickDuration.Seconds())
	}
	if after := b.Civ.Defense() + s.relief(b.ID); after <= before {
		t.Fatalf("defense %v after relief, %v before", after, before)
	}
	if a.Civ.Defenders < minimalGuard {
		t.Error("the rescuer left itself without guard")
	}
}

func TestRecallBringsEveryoneHome(t *testing.T) {
	s, a, b := twoIslands(t)
	a.Civ.Defenders = 12
	s.Whisper(a.ID, Rescue, b.ID)
	for range int(600 / TickDuration.Seconds()) {
		s.raids(TickDuration.Seconds())
	}
	s.Whisper(a.ID, NoOuter, 0)
	untilNoRaids(s, 3600)
	if a.Civ.Defenders != 12 {
		t.Fatalf("%v defenders home after the recall, want 12", a.Civ.Defenders)
	}
}

func TestOwnIslandAndUnknownTargetsAreRefused(t *testing.T) {
	s, a, _ := twoIslands(t)
	if err := s.Whisper(a.ID, Punish, a.ID); err != ErrOwnIsland {
		t.Errorf("raiding oneself: %v", err)
	}
	if err := s.Whisper(a.ID, Punish, 999); err != ErrNoTarget {
		t.Errorf("raiding nowhere: %v", err)
	}
}

func TestPunishingWithoutWarriorsIsExplained(t *testing.T) {
	s, a, b := twoIslands(t)
	a.Civ.Assailants = 0
	if err := s.Whisper(a.ID, Punish, b.ID); err != nil {
		t.Fatal(err)
	}
	untilNoRaids(s, 60)
	if !strings.Contains(allText(a.Civ.Chronicle), "aucun guerrier") {
		t.Fatal("the intention vanished without a word")
	}
}

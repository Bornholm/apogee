package sim

import (
	"math"
	"strings"
	"testing"
)

func steps(s *Sky, seconds float64) {
	for range int(seconds / TickDuration.Seconds()) {
		s.Step()
	}
}

func TestIslandsDescendFasterWhenPowerful(t *testing.T) {
	var s Sky
	weak := s.AddIsland("faible", 1, 0)
	strong := s.AddIsland("forte", 2, 3)
	s.Island(strong).Civ.Population = 400 // power through sheer numbers
	steps(&s, 3600)
	dw := RimRadius - s.Island(weak).Radius
	ds := RimRadius - s.Island(strong).Radius
	if dw <= 0 || ds <= dw {
		t.Fatalf("descent: weak %.2f, strong %.2f: power must hasten the fall", dw, ds)
	}
}

func TestAPowerlessIslandCrossesTheSkyInAWeek(t *testing.T) {
	got := (RimRadius - PortalRadius) / descentSpeed
	if math.Abs(got-Lifetime) > 1 {
		t.Fatalf("a powerless island lives %.0f s, want %.0f", got, Lifetime)
	}
}

func TestWavesGrowCloserToThePortal(t *testing.T) {
	rim := WaveStrength(50, RimRadius, youth, false)
	mid := WaveStrength(50, (RimRadius+PortalRadius)/2, youth, false)
	portal := WaveStrength(50, PortalRadius, youth, false)
	if !(rim < mid && mid < portal) {
		t.Fatalf("strength rim %.1f, mid %.1f, portal %.1f: must grow inwards", rim, mid, portal)
	}
	if WaveStrength(50, 500, youth, true) <= WaveStrength(50, 500, youth, false) {
		t.Error("night waves are not stronger")
	}
	// Calibrated on the target's own power, never the sky's (GDD 8.3).
	if WaveStrength(10, RimRadius, youth, false) >= WaveStrength(200, RimRadius, youth, false) {
		t.Error("a weak island faces waves as strong as a mighty one")
	}
}

func TestAWaveIsAnnouncedThenStrikes(t *testing.T) {
	var s Sky
	id := s.AddIsland("cible", 3, 0)
	s.Step()
	if s.Wave == nil || s.Wave.Target != id || s.Wave.In < waveMin-1 || s.Wave.In > waveMax {
		t.Fatalf("no wave announced: %+v", s.Wave)
	}
	steps(&s, waveMax+1)
	c := s.Island(id).Civ
	found := false
	for _, e := range c.Chronicle {
		if strings.Contains(e.Text, "vague") {
			found = true
		}
	}
	if !found {
		t.Fatal("the wave left no trace in the chronicle")
	}
}

func TestBrokenHeartSwallowsTheIslandAndItsPeopleIsReborn(t *testing.T) {
	var s Sky
	id := s.Found("joueur", "Ada", 5, 0)
	is := s.Island(id)
	is.Radius = PortalRadius
	is.Civ.Defenders = 0
	is.Civ.Buildings = is.Civ.Buildings[:1]
	is.Civ.CoreHealth = 0.01
	s.attack(is, 5, false, "une vague du tourbillon")
	if is.Falling <= 0 {
		t.Fatal("the heart broke but the island does not fall")
	}
	steps(&s, fallDuration+1)
	if s.Island(id) != nil {
		t.Fatal("the whirlpool did not swallow the island")
	}
	if len(s.Annals) != 1 || s.Annals[0].Owner != "joueur" || !strings.Contains(s.Annals[0].Cause, "tourbillon") {
		t.Fatalf("annals: %+v", s.Annals)
	}
	if s.Rebirths() != 1 {
		t.Fatalf("%d rebirths waiting, want 1", s.Rebirths())
	}
	steps(&s, DayLength.Seconds())
	nid, ok := s.IslandOf("joueur")
	if !ok {
		t.Fatal("the people were not reborn at dawn")
	}
	n := s.Island(nid)
	if n.Radius < RimRadius-1 || n.Generation != 2 || n.Seed == 5 {
		t.Errorf("reborn island: radius %v, generation %d, seed %d", n.Radius, n.Generation, n.Seed)
	}
}

func TestRuinsSlowProduction(t *testing.T) {
	c := NewCiv(1)
	full := c.productionFactor(false)
	c.CoreHealth = ruinThreshold / 2
	if got := c.productionFactor(false); got >= full {
		t.Errorf("production factor in ruins %v, healthy %v", got, full)
	}
}

func TestDeclineIsForetold(t *testing.T) {
	var s Sky
	id := s.AddIsland("vieille", 6, 0)
	s.Island(id).Radius = DeclineRadius + 0.01
	steps(&s, 60)
	is := s.Island(id)
	if !is.Declining {
		t.Fatal("an island in the inner ring is not declining")
	}
	if !strings.Contains(is.Civ.Chronicle[len(is.Civ.Chronicle)-1].Text+allText(is.Civ.Chronicle), "déclin") {
		t.Error("no omen of decline in the chronicle")
	}
}

func allText(es []Entry) string {
	var b strings.Builder
	for _, e := range es {
		b.WriteString(e.Text)
	}
	return b.String()
}

func TestWavesAreInexorable(t *testing.T) {
	young := WaveStrength(50, PortalRadius, youth, false)
	week := WaveStrength(50, PortalRadius, Lifetime, false)
	later := WaveStrength(50, PortalRadius, Lifetime+12*3600, false)
	if !(young < week && week < later) || later < 10*young {
		t.Fatalf("at the portal: young %.0f, a week %.0f, later %.0f: waves must keep growing", young, week, later)
	}
}

func TestNewbornsMeetGentleWaves(t *testing.T) {
	if WaveStrength(40, RimRadius, 0, false) >= WaveStrength(40, RimRadius, youth, false) {
		t.Error("a newborn faces waves as strong as a settled civilisation")
	}
}

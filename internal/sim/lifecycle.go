package sim

import (
	"fmt"
	"hash/fnv"
	"math"
	"math/rand/v2"
	"strings"
)

// Rise and fall (GDD 1.3 bis, 3.1, 7.5, 8, 11.5): islands drift towards the
// portal with age and power, waves grow fiercer as they get closer, and an
// island whose heart breaks is swallowed by the whirlpool. Its people found a
// new civilisation at the rim at the next dawn.

// Life-cycle tunables (GDD 🔧).
const (
	Lifetime      = 7 * 24 * 3600.0 // a civilisation lasts a week at most
	DeclineRadius = 300.0           // the inner ring: islands there decline
	powerScale    = 120.0           // power doubling the descent speed
	waveMin       = 5 * 60.0
	waveMax       = 10 * 60.0
	waveWarning   = 40.0 // seconds of warning before a wave (GDD 8.2)
	bossMin       = 45 * 60.0
	bossMax       = 90 * 60.0
	bossWarning   = 5 * 60.0
	bossFactor    = 2.0
	nightWave     = 1.2         // GDD 3.6
	ruinThreshold = 0.3         // core health under which the island lies in ruins
	ruinFactor    = 0.5         // production in ruins
	coreRepair    = 1.0 / 40000 // core health regained per second (about 9 % an hour)
	repairStone   = 0.05        // stone per second while repairing
	fallDuration  = 30.0        // seconds the whirlpool takes to swallow an island
	legendBonus   = 0.15        // culture inherited from the ancestors
)

// descentSpeed is chosen so that a powerless island crosses the sky in
// exactly Lifetime: power only ever shortens a life (GDD 3.1).
var descentSpeed = (RimRadius - PortalRadius) / Lifetime

// Power is the composite measure of an island (GDD 5.3): tiers, buildings,
// people and soldiers.
func (c *Civ) Power() float64 {
	return 10*float64(c.Tech.Total()) + float64(len(c.Buildings)) +
		0.2*c.Population + 0.5*(c.Defenders+c.Assailants)
}

// Exposure is how close to the portal an island is, from 0 at the rim to 1 at
// the portal.
func Exposure(radius float64) float64 {
	return math.Min(math.Max((RimRadius-radius)/(RimRadius-PortalRadius), 0), 1)
}

// waveScale tunes the overall strength of the waves (GDD 🔧, set by the
// balancing bench, cmd/apogee-balance).
const waveScale = 0.42

// youth is how long waves stay gentle on a newborn civilisation (GDD 11.3).
const youth = 6 * 3600.0

// WaveStrength is the strength of a wave against an island (GDD 8.3): it
// follows the island's own power, never the sky's average, grows with
// exposure and, inexorably, with age (GDD 1.3 bis). At a given distance and
// age it is bounded and predictable. age is in seconds.
func WaveStrength(power, radius, age float64, night bool) float64 {
	e := Exposure(radius)
	s := waveScale * (2 + power*0.25) * (0.2 + 2.8*math.Pow(e, 1.5))
	// A young civilisation meets gentler waves while it finds its feet.
	s *= math.Min(1, 0.25+0.75*age/youth)
	// Age makes every wave stronger; past a week, they grow without end, so
	// that even a fortress eventually falls.
	a := age / Lifetime
	s *= 1 + 3*a*a
	if age > Lifetime {
		s *= 1 + (age-Lifetime)/900
	}
	if night {
		s *= nightWave
	}
	return s
}

// Threat is an announced attack.
type Threat struct {
	Boss bool
	// Target is the island a wave aims at; a boss strikes every island.
	Target IslandID
	// In is the time in seconds before the attack.
	In float64
}

// Annal records a civilisation in the Annals of the sky (GDD 11.5).
type Annal struct {
	Name       string
	Owner      string
	Generation int
	Born       uint64 // ticks
	Fell       uint64
	PeakPower  float64
	Tiers      int
	Culture    Family
	Cause      string
}

// Rebirth is a civilisation waiting for the next dawn to rise at the rim.
type Rebirth struct {
	Owner   string
	Name    string
	Legend  Family
	Count   int
	Profile Profile
}

func (s *Sky) rng(salt uint64) *rand.Rand {
	return rand.New(rand.NewPCG(s.Seed^salt, s.Tick))
}

// lifecycle runs once per tick, after the civilisations.
func (s *Sky) lifecycle(dt float64, phase Phase) {
	night := phase == Night
	for i := range s.Islands {
		is := &s.Islands[i]
		if is.Falling > 0 {
			continue
		}
		c := &is.Civ
		is.PeakPower = math.Max(is.PeakPower, c.Power())
		is.Radius = math.Max(PortalRadius, is.Radius-descentSpeed*(1+c.Power()/powerScale)*dt)
		if is.Radius < DeclineRadius && !is.Declining {
			is.Declining = true
			c.chronicleAs(DeclineNews, s.Tick, "Les anciens disent que le tourbillon nous appelle : notre déclin a commencé.")
		}
		c.repairCore(dt)
	}
	s.waves(dt, night)
	s.falls(dt)
	// Rebirths wait for a dawn to begin, not merely for the dawn hours.
	if phase == Dawn && s.LastPhase != Dawn {
		s.rebirths()
	}
	s.LastPhase = phase
}

func (c *Civ) repairCore(dt float64) {
	if c.CoreHealth >= 1 || c.Stock[Stone] < repairStone*dt {
		return
	}
	c.Stock[Stone] -= repairStone * dt
	c.CoreHealth = math.Min(1, c.CoreHealth+coreRepair*dt)
}

// waves announces and resolves waves and bosses.
func (s *Sky) waves(dt float64, night bool) {
	if len(s.Islands) == 0 {
		return
	}
	if s.Wave == nil {
		r := s.rng(0x77)
		s.Wave = &Threat{Target: s.pickTarget(r), In: waveMin + r.Float64()*(waveMax-waveMin)}
	}
	if s.Boss == nil {
		r := s.rng(0xb055)
		s.Boss = &Threat{Boss: true, In: bossMin + r.Float64()*(bossMax-bossMin)}
	}
	s.Wave.In -= dt
	s.Boss.In -= dt
	if s.Wave.In <= 0 {
		if is := s.island(s.Wave.Target); is != nil && is.Falling == 0 {
			s.attack(is, 1, night, "une vague du tourbillon")
		}
		s.Wave = nil
	}
	if s.Boss.In <= 0 {
		for i := range s.Islands {
			if s.Islands[i].Falling == 0 {
				s.attack(&s.Islands[i], bossFactor, night, "le colosse du tourbillon")
			}
		}
		s.Boss = nil
	}
}

// pickTarget draws the island a wave aims at, weighted by exposure
// (GDD 8.2).
func (s *Sky) pickTarget(r *rand.Rand) IslandID {
	total := 0.0
	w := make([]float64, len(s.Islands))
	for i, is := range s.Islands {
		if is.Falling == 0 {
			w[i] = 0.2 + Exposure(is.Radius)
		}
		total += w[i]
	}
	x := r.Float64() * total
	for i, v := range w {
		if x < v {
			return s.Islands[i].ID
		}
		x -= v
	}
	return s.Islands[len(s.Islands)-1].ID
}

// attack resolves an attack against an island, as an autobattler: defense
// against strength (GDD 6.9).
func (s *Sky) attack(is *Island, factor float64, night bool, by string) {
	c := &is.Civ
	age := float64(s.Tick-is.Born) * TickDuration.Seconds()
	strength := WaveStrength(c.Power(), is.Radius, age, night) * factor
	defense := c.Defense() + s.relief(is.ID) // help from neighbours counts too
	if c.CoreHealth < ruinThreshold {
		defense *= ruinFactor
	}
	m := c.Tech.Mods()
	// Defenders fall in proportion to the strength they faced.
	lost := math.Min(c.Defenders, strength*0.03/(1+m.DefenderStrength))
	c.Defenders -= lost
	if defense >= strength {
		c.chronicle(s.Tick, "%s", strings.TrimSpace("Ton peuple a repoussé "+by+". "+losses(lost)))
		return
	}
	harm := (strength - defense) / strength * 0.35 / (1 + m.CoreArmor)
	c.CoreHealth -= harm
	for r := range c.Stock {
		c.Stock[r] *= 1 - 0.15*harm/0.35
	}
	if c.CoreHealth <= 0 {
		c.CoreHealth = 0
		is.Falling = fallDuration
		is.Cause = "aspirée par le tourbillon sous les coups " + of(by)
		c.chronicleAs(FallNews, s.Tick, "Le cœur s'est brisé sous les coups %s. L'île sombre dans le tourbillon.", of(by))
		return
	}
	c.chronicleAs(WaveNews, s.Tick, "%s", strings.TrimSpace(fmt.Sprintf("%s a frappé le cœur (%d %% de vigueur restante). %s",
		capitalize(by), int(math.Round(c.CoreHealth*100)), losses(lost))))
}

func losses(n float64) string {
	switch k := int(math.Round(n)); {
	case k <= 0:
		return ""
	case k == 1:
		return "Un défenseur est tombé."
	default:
		return fmt.Sprintf("%d défenseurs sont tombés.", k)
	}
}

// of turns « une vague » into « d'une vague » and « le colosse » into
// « du colosse ».
func of(s string) string {
	switch {
	case len(s) > 3 && s[:3] == "le ":
		return "du " + s[3:]
	case len(s) > 0 && (s[0] == 'u' || s[0] == 'a' || s[0] == 'e'):
		return "d'" + s
	default:
		return "de " + s
	}
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	if r[0] >= 'a' && r[0] <= 'z' {
		r[0] -= 'a' - 'A'
	}
	return string(r)
}

// falls finishes swallowing the islands whose heart broke: they enter the
// Annals and leave the sky; their people will be reborn at dawn.
func (s *Sky) falls(dt float64) {
	kept := s.Islands[:0]
	for _, is := range s.Islands {
		if is.Falling > 0 {
			is.Falling -= dt
			if is.Falling <= 0 {
				s.Annals = append(s.Annals, Annal{
					Name: is.Name, Owner: is.Owner, Generation: is.Generation, Born: is.Born, Fell: s.Tick,
					PeakPower: is.PeakPower, Tiers: is.Civ.Tech.Total(),
					Culture: is.Civ.Dominant(), Cause: is.Cause,
				})
				s.Waiting = append(s.Waiting, Rebirth{
					Owner: is.Owner, Name: is.OwnerName, Legend: is.Civ.Dominant(), Count: is.Generation + 1,
					Profile: is.Profile,
				})
				continue
			}
		}
		kept = append(kept, is)
	}
	s.Islands = kept
}

// rebirths founds, at dawn, the new civilisations of fallen peoples
// (GDD 11.5).
func (s *Sky) rebirths() {
	for _, w := range s.Waiting {
		h := fnv.New64a()
		fmt.Fprintf(h, "%s/%d", w.Owner, w.Count)
		id := s.Found(w.Owner, w.Name, h.Sum64(), s.rng(uint64(w.Count)).Float64()*2*math.Pi)
		is := s.island(id)
		is.Generation = w.Count
		is.Profile = w.Profile
		is.Civ.Culture[w.Legend] += legendBonus
		is.Civ.chronicle(s.Tick, "Des nuages émerge une île nouvelle. Ton peuple garde la légende de ses ancêtres, qui vénéraient les %s.", w.Legend.Name())
	}
	s.Waiting = s.Waiting[:0]
}

// AwaitingRebirth reports whether an owner's people wait for the dawn.
func (s *Sky) AwaitingRebirth(owner string) bool {
	for _, w := range s.Waiting {
		if w.Owner == owner {
			return true
		}
	}
	return false
}

// Rebirths reports how many fallen peoples wait for the dawn.
func (s *Sky) Rebirths() int { return len(s.Waiting) }

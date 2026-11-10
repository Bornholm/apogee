package sim

import (
	"math/rand/v2"
	"time"
)

// Species is the kind of monsters a wave brings (GDD 11.6): announced with
// the wave, it changes what holds them off, so that the god adapts the pie
// to what the whirlpool sends.
type Species int8

const (
	Monsters  Species = iota // the classic wave
	Swarm                    // many and quick: towers fire into the void
	Devourers                // they go for the fields
	Breakers                 // they bring down towers
	Shades                   // night only; the scholars' wards hold them
	NumSpecies
)

// Name names the monsters of a species, as the chronicle tells them.
func (sp Species) Name() string {
	if sp < 0 || sp >= NumSpecies {
		sp = Monsters
	}
	return [...]string{"une vague du tourbillon", "un essaim du tourbillon", "des dévoreurs du tourbillon",
		"des brise-tours du tourbillon", "des ombres du tourbillon"}[sp]
}

// Short names a species in a few words, for the interface.
func (sp Species) Short() string {
	if sp < 0 || sp >= NumSpecies {
		sp = Monsters
	}
	return [...]string{"Une vague", "Un essaim", "Des dévoreurs", "Des brise-tours", "Des ombres"}[sp]
}

// Hint tells what holds a species off, for the player.
func (sp Species) Hint() string {
	if sp < 0 || sp >= NumSpecies {
		sp = Monsters
	}
	return [...]string{"",
		"Trop vifs pour les tours : seuls les défenseurs les arrêtent.",
		"S'ils passent, ils dévorent les champs plus qu'ils ne frappent le cœur.",
		"S'ils passent, ils abattent une tour.",
		"Ils ne viennent que la nuit ; les gardes des bibliothèques les repoussent.",
	}[sp]
}

// phaseAt returns the time of day at a moment of the sky.
func phaseAt(t time.Duration) Phase {
	t %= DayLength
	for i, end := range phaseEnds {
		if t < end {
			return Phase(i)
		}
	}
	return Night
}

// drawSpecies draws the species of a wave that strikes after in seconds:
// shades only come by night.
func (s *Sky) drawSpecies(r *rand.Rand, in float64) Species {
	night := phaseAt(s.Elapsed()+time.Duration(in*float64(time.Second))) == Night
	for {
		// The classic wave stays the most common.
		sp := Species(r.IntN(int(NumSpecies) + 2))
		if sp >= NumSpecies {
			sp = Monsters
		}
		if sp != Shades || night {
			return sp
		}
	}
}

// defenseAgainst is the defense of a civilisation against a species: the
// same men and walls, counted otherwise.
func (c *Civ) defenseAgainst(sp Species) float64 {
	m := c.Mods()
	men := c.Defenders * (1 + m.DefenderStrength)
	walls := 0.0
	for _, b := range c.Buildings {
		if !b.Done() {
			continue
		}
		v := Specs[b.Kind].Defense
		switch {
		case b.Kind == Tower:
			v *= 1 + m.TowerDefense
			if sp == Swarm {
				v *= 0.5
			}
		case b.Kind == Library && sp == Shades:
			v *= 3
		}
		walls += v
	}
	if sp == Swarm {
		men *= 4.0 / 3
	}
	return (men + walls) * (1 + m.Defense)
}

// ravage is what a species does to an island it was not held off from,
// beyond the blow on the heart: it returns the share of that blow it deals,
// and a line for the chronicle.
func (c *Civ) ravage(sp Species) (float64, string) {
	switch sp {
	case Devourers:
		c.Stock[Food] *= 0.4
		return 0.5, "Les champs sont dévorés."
	case Breakers:
		for i, b := range c.Buildings {
			if b.Kind == Tower && b.Done() {
				c.Buildings = append(c.Buildings[:i], c.Buildings[i+1:]...)
				return 1, "Une tour s'est effondrée."
			}
		}
	}
	return 1, ""
}

// swarmSize is how many monsters show in a wave of a strength.
func swarmSize(sp Species, strength float64) int {
	n := min(max(3+int(strength/4), 3), 10)
	switch sp {
	case Swarm:
		return min(n*2, 16)
	case Breakers:
		return max(n/2, 2)
	}
	return n
}

package sim

import "hash/fnv"

// Trait is the character of an island (GDD 11.6): drawn from its seed at
// birth, known to the whole sky, it changes how the civilisation is
// played, through the same modifiers as the discoveries.
type Trait int8

const (
	NoTrait     Trait = iota // islands founded before traits existed
	CrystalIsle              // aether, little wood
	ForestIsle               // wood and quick building, a frailer heart
	RockIsle                 // stone and towers, less food
	HotSpring                // many births, deadlier famines
	HighIsle                 // sinks slowly, grows dearly
	FertileIsle              // food, a peaceful people
	NumTraits
)

// TraitOf draws the trait of an island from its seed.
func TraitOf(seed uint64) Trait {
	h := fnv.New64a()
	var b [8]byte
	for i := range b {
		b[i] = byte(seed >> (8 * i))
	}
	h.Write(b[:])
	h.Write([]byte("trait"))
	return Trait(1 + h.Sum64()%uint64(NumTraits-1))
}

// Name names a trait.
func (t Trait) Name() string {
	if t <= NoTrait || t >= NumTraits {
		return ""
	}
	return [...]string{"", "Île de cristal", "Île forestière", "Île de roc", "Source chaude", "Île haute", "Terres fertiles"}[t]
}

// Text tells what a trait does, for the player.
func (t Trait) Text() string {
	if t <= NoTrait || t >= NumTraits {
		return ""
	}
	return [...]string{"",
		"Aether +40 %, bois −20 %, et des cristaux en nombre.",
		"Bois +30 %, construction plus rapide, mais un cœur plus fragile.",
		"Pierre +30 %, tours plus solides, mais moins de vivres.",
		"Naissances +50 %, mais les famines sont plus meurtrières.",
		"Descend moins vite vers le tourbillon, mais s'étendre coûte plus cher.",
		"Vivres +35 %, mais un peuple paisible se défend moins bien.",
	}[t]
}

// Mods are the effects of a trait.
func (t Trait) Mods() Mods {
	var m Mods
	switch t {
	case CrystalIsle:
		m.Prod[Aether], m.Prod[Wood] = 0.4, -0.2
	case ForestIsle:
		m.Prod[Wood], m.BuildSpeed, m.CoreArmor = 0.3, 0.15, -0.15
	case RockIsle:
		m.Prod[Stone], m.TowerDefense, m.Prod[Food] = 0.3, 0.15, -0.15
	case HotSpring:
		m.Growth, m.Famine = 0.5, 0.5
	case HighIsle:
		m.Descent, m.ExpandCost = -0.2, 0.5
	case FertileIsle:
		m.Prod[Food], m.Defense = 0.35, -0.1
	}
	return m
}

// featureCounts are how many groves, rocks and crystals an island of a
// trait has.
func (t Trait) featureCounts() (groves, rocks, crystals int) {
	switch t {
	case CrystalIsle:
		return 2, 3, 5
	case ForestIsle:
		return 5, 2, 2
	case RockIsle:
		return 3, 5, 2
	}
	return 3, 3, 2
}

// Mods are all the modifiers of a civilisation: its discoveries and the
// trait of its island.
func (c *Civ) Mods() Mods {
	return c.Tech.Mods().add(c.Trait.Mods())
}

// DescentSpeed is how fast this civilisation's island sinks, in sky units
// per second.
func (c *Civ) DescentSpeed() float64 {
	return DescentSpeed(c.Power()) * (1 + c.Mods().Descent)
}

// ExpandCost is what pushing the edges of the island costs.
func (c *Civ) ExpandCost() Stock {
	k := 1 + c.Mods().ExpandCost
	var s Stock
	for r, v := range expandCost {
		s[r] = v * k
	}
	return s
}

// lower turns the first letter of a name to lower case: « une île de
// cristal » in the middle of a sentence.
func lower(s string) string {
	r := []rune(s)
	if len(r) > 0 && r[0] >= 'A' && r[0] <= 'Z' {
		r[0] += 'a' - 'A'
	}
	return string(r)
}

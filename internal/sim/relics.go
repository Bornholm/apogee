package sim

import (
	"fmt"
	"sort"
)

// Relic is the one treasure the survivors of a fallen civilisation saved
// (GDD 11.6). Drawn from the deeds of the fallen, chosen at rebirth, it
// helps the next generation in one field, and that generation only.
type Relic int8

const (
	NoRelic      Relic = iota
	Crown              // from a high apogee: the reserves of the start, doubled
	Codex              // from the tiers reached: knowledge
	Shield             // from the waves held off: the heart's armour
	Seeds              // from the fields: food
	WarBanner          // from the raids won: warriors
	BuilderStone       // from the buildings: quicker building
	NumRelics
)

// Name names a relic.
func (r Relic) Name() string {
	if r <= NoRelic || r >= NumRelics {
		return ""
	}
	return [...]string{"", "Couronne de l'apogée", "Codex des anciens", "Bouclier du cœur", "Semences anciennes",
		"Bannière des conquérants", "Pierre des bâtisseurs"}[r]
}

// Text tells what a relic does, for the player.
func (r Relic) Text() string {
	if r <= NoRelic || r >= NumRelics {
		return ""
	}
	return [...]string{"", "réserves de départ doublées", "savoir +20 %", "le cœur encaisse 20 % de moins",
		"vivres +20 %", "guerriers +20 %", "construction 20 % plus rapide"}[r]
}

// Mods are the effects of a relic. The crown acts once, at the choice.
func (r Relic) Mods() Mods {
	var m Mods
	switch r {
	case Codex:
		m.Knowledge = 0.2
	case Shield:
		m.CoreArmor = 0.2
	case Seeds:
		m.Prod[Food] = 0.2
	case WarBanner:
		m.AssailantStrength = 0.2
	case BuilderStone:
		m.BuildSpeed = 0.2
	}
	return m
}

// relicsOf draws the three relics a fallen civilisation may leave: those
// its deeds earned most.
func relicsOf(c *Civ, peak float64) []Relic {
	farms := 0
	for _, b := range c.Buildings {
		if b.Kind == Farm && b.Done() {
			farms++
		}
	}
	score := map[Relic]float64{
		Crown:        peak / 50,
		Codex:        float64(c.Tech.Total()),
		Shield:       float64(c.Repelled) / 3,
		Seeds:        float64(farms) / 2,
		WarBanner:    float64(c.Conquests),
		BuilderStone: float64(len(c.Buildings)) / 8,
	}
	all := []Relic{Crown, Codex, Shield, Seeds, WarBanner, BuilderStone}
	sort.SliceStable(all, func(i, j int) bool { return score[all[i]] > score[all[j]] })
	return all[:3]
}

// offerRelics asks the newborn people which relic the survivors saved.
func (c *Civ) offerRelics(relics []Relic, tick uint64) {
	if len(relics) == 0 {
		return
	}
	s := Supplique{Kind: AskRelic, Relics: relics,
		Question: "Les survivants de l'île engloutie n'ont pu sauver qu'un trésor. Lequel ?"}
	for _, r := range relics {
		s.Options = append(s.Options, fmt.Sprintf("%s : %s", r.Name(), r.Text()))
	}
	c.ask(s, tick)
}

// keepRelic gives the people the relic chosen.
func (c *Civ) keepRelic(r Relic, tick uint64) {
	c.Relic = r
	if r == Crown {
		for i := range c.Stock {
			c.Stock[i] *= 2
		}
	}
	c.chronicle(tick, "Le peuple dresse près du cœur la stèle de son trésor : %s.", lower(r.Name()))
}

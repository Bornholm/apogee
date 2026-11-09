package sim

// Technology (GDD 5): four axes of two tiers each in the MVP. The axis the
// scholars follow is the culture's dominant family; at each tier they offer
// two discoveries through a supplique.

// Tech tunables (GDD 🔧).
const (
	TiersPerAxis   = 2
	TechCap        = 3     // tiers a people can master in all (GDD 5.3)
	baseKnowledge  = 0.01  // per second, the people's own curiosity
	aetherPerPoint = 1.0   // aether burnt per point of knowledge from study
	nightKnowledge = 1.3   // the scholars stay up (GDD 3.6)
	cultureBonus   = 0.15  // cost reduction for the dominant family
	aetherReserve  = 100.0 // aether the scholars never burn
)

// tierCost is the knowledge needed to reach each tier of an axis.
var tierCost = [TiersPerAxis]float64{300, 900}

// TierCost returns the knowledge needed to reach a tier (0-based).
func TierCost(tier int) float64 { return tierCost[tier] }

// Mods are the effects of discoveries, added together.
type Mods struct {
	Prod              [NumResources]float64 // extra share of production
	Knowledge         float64
	BuildSpeed        float64
	Storage           float64
	TowerDefense      float64
	DefenderStrength  float64
	AssailantStrength float64
	CoreArmor         float64
	NightProduction   float64 // replaces the night factor when higher
}

func (m Mods) add(o Mods) Mods {
	for i := range m.Prod {
		m.Prod[i] += o.Prod[i]
	}
	m.Knowledge += o.Knowledge
	m.BuildSpeed += o.BuildSpeed
	m.Storage += o.Storage
	m.TowerDefense += o.TowerDefense
	m.DefenderStrength += o.DefenderStrength
	m.AssailantStrength += o.AssailantStrength
	m.CoreArmor += o.CoreArmor
	m.NightProduction = max(m.NightProduction, o.NightProduction)
	return m
}

// Discovery is one of the two choices offered at a tier.
type Discovery struct {
	Name string
	Text string // what it does, for the player
	Mods Mods
	// Lean is the intention whose people would pick it unprompted.
	Lean Intention
}

// Discoveries lists, for each axis and tier, the two choices (GDD 5.3).
var Discoveries = [NumFamilies][TiersPerAxis][2]Discovery{
	Weapons: {
		{
			{"Lames trempées", "Assaillants +30 %", Mods{AssailantStrength: 0.3}, Arm},
			{"Cris de guerre", "Défenseurs +20 %, assaillants +10 %", Mods{DefenderStrength: 0.2, AssailantStrength: 0.1}, Fortify},
		},
		{
			{"Béliers", "Assaillants +30 %", Mods{AssailantStrength: 0.3}, Arm},
			{"Forges", "Bois +20 %, pierre +20 %", Mods{Prod: [NumResources]float64{Wood: 0.2, Stone: 0.2}}, Prosper},
		},
	},
	Walls: {
		{
			{"Mortier", "Tours +40 %", Mods{TowerDefense: 0.4}, Fortify},
			{"Guet", "Défenseurs +30 %", Mods{DefenderStrength: 0.3}, Fortify},
		},
		{
			{"Remparts du cœur", "Cœur blindé +50 %", Mods{CoreArmor: 0.5}, Fortify},
			{"Maçons", "Construction +30 %, pierre +20 %", Mods{BuildSpeed: 0.3, Prod: [NumResources]float64{Stone: 0.2}}, Prosper},
		},
	},
	Industry: {
		{
			{"Charrue", "Vivres +30 %", Mods{Prod: [NumResources]float64{Food: 0.3}}, Prosper},
			{"Scie", "Bois +30 %, pierre +10 %", Mods{Prod: [NumResources]float64{Wood: 0.3, Stone: 0.1}}, Expand},
		},
		{
			{"Greniers", "Stockage +50 %", Mods{Storage: 0.5}, Prosper},
			{"Corvées", "Construction +40 %", Mods{BuildSpeed: 0.4}, Expand},
		},
	},
	Arcana: {
		{
			{"Lentilles", "Savoir +40 %", Mods{Knowledge: 0.4}, Study},
			{"Résonance", "Aether +40 %", Mods{Prod: [NumResources]float64{Aether: 0.4}}, Study},
		},
		{
			{"Vision nocturne", "Production de nuit à 80 %", Mods{NightProduction: 0.8}, Prosper},
			{"Sceaux", "Cœur blindé +30 %, défenseurs +10 %", Mods{CoreArmor: 0.3, DefenderStrength: 0.1}, Fortify},
		},
	},
}

// Tech is a people's knowledge.
type Tech struct {
	// Chosen holds, per axis and tier, the discovery picked (0 or 1), or -1
	// for a tier not mastered.
	Chosen [NumFamilies][TiersPerAxis]int8
	// Progress is the knowledge gathered towards the next tier of the axis
	// the scholars follow.
	Progress float64
}

func newTech() Tech {
	var t Tech
	for f := range t.Chosen {
		for k := range t.Chosen[f] {
			t.Chosen[f][k] = -1
		}
	}
	return t
}

// Tier returns how many tiers of the axis are mastered.
func (t *Tech) Tier(f Family) int {
	n := 0
	for _, c := range t.Chosen[f] {
		if c >= 0 {
			n++
		}
	}
	return n
}

// Total returns the tiers mastered across all axes.
func (t *Tech) Total() int {
	n := 0
	for f := range NumFamilies {
		n += t.Tier(f)
	}
	return n
}

// Mods adds up the effects of every discovery made.
func (t *Tech) Mods() Mods {
	var m Mods
	for f := range t.Chosen {
		for k, c := range t.Chosen[f] {
			if c >= 0 {
				m = m.add(Discoveries[f][k][c].Mods)
			}
		}
	}
	return m
}

// FamilyOf returns the family a building belongs to, for the culture bonus.
func FamilyOf(k Kind) Family {
	switch k {
	case Tower:
		return Walls
	case Barracks:
		return Weapons
	case Library, Extractor:
		return Arcana
	default:
		return Industry
	}
}

// Cost returns what building kind k costs this people: the dominant family
// builds its own buildings cheaper (GDD 6.6).
func (c *Civ) Cost(k Kind) Stock {
	cost := Specs[k].Cost
	for r := range cost {
		cost[r] *= CostScale
	}
	if FamilyOf(k) == c.Dominant() {
		for r := range cost {
			cost[r] *= 1 - cultureBonus
		}
	}
	return cost
}

// research turns aether and curiosity into knowledge, and knowledge into
// tiers of the axis the culture favours.
func (c *Civ) research(dt float64, night bool, libs float64, tick uint64) {
	m := c.Tech.Mods()
	gain := baseKnowledge
	// Study burns aether into knowledge (GDD 5.3).
	// Only the aether above a reserve is burnt: the scholars leave enough to
	// build libraries and extractors.
	burn := min(max(c.Stock[Aether]-aetherReserve, 0), 0.02*c.Conviction[Study]*dt)
	c.Stock[Aether] -= burn
	gain += burn / aetherPerPoint / dt
	gain *= (1 + c.Conviction[Study]) * (1 + m.Knowledge)
	if night {
		gain *= nightKnowledge
	}
	libs *= (1 + c.Conviction[Study]) * (1 + m.Knowledge)
	c.Knowledge += gain*dt + libs // lifetime total, for the Annals

	if c.Pending != nil && c.Pending.Kind == AskDiscovery {
		return // the scholars wait for the choice before going further
	}
	axis := c.Dominant()
	tier := c.Tech.Tier(axis)
	if tier >= TiersPerAxis {
		return // the axis is mastered: knowledge waits for a new direction
	}
	c.Tech.Progress += gain*dt + libs
	if c.Tech.Progress < tierCost[tier] {
		return
	}
	c.Tech.Progress = 0
	if c.Tech.Total() >= TechCap {
		c.forget(axis, tick)
	}
	c.offerDiscovery(axis, tier, tick)
}

// forget drops the highest tier of the axis the culture cares least about,
// to make room for the new one (GDD 5.3).
func (c *Civ) forget(keep Family, tick uint64) {
	victim, low := Family(-1), 2.0
	for f := range NumFamilies {
		if f != keep && c.Tech.Tier(f) > 0 && c.Culture[f] < low {
			victim, low = f, c.Culture[f]
		}
	}
	if victim < 0 {
		return
	}
	k := c.Tech.Tier(victim) - 1
	lost := Discoveries[victim][k][c.Tech.Chosen[victim][k]].Name
	c.Tech.Chosen[victim][k] = -1
	c.chronicle(tick, "Les derniers gardiens de « %s » se sont éteints : ce savoir est oublié.", lost)
}

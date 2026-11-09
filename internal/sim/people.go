package sim

import (
	"math"
	"math/rand/v2"

	"apogee/internal/geo"
)

// Placement tunables (GDD 3.5).
const (
	placementMargin = 1.5 // free space between two footprints
	edgeMargin      = 2.0 // free space between a footprint and the cliff
	placementTries  = 32
	minScore        = 0.05 // below this, the people rather wait
	survivalScore   = 0.3  // floor of a building the people need to survive
)

// decide is the people's interpretation of the god's intentions (GDD 6.4):
// a utility score picks what to build next, then where. It runs every few
// seconds and only starts one project at a time.
func (c *Civ) decide(tick uint64) {
	if c.building() {
		return
	}
	r := rand.New(rand.NewPCG(c.Seed^0x7fb5d329728ea185, tick))

	bestKind, bestScore := Kind(-1), minScore
	for k := House; k < NumKinds; k++ {
		if s := c.score(k); s > bestScore {
			bestKind, bestScore = k, s
		}
	}
	// An expansion the people cannot afford yet does not stop them from
	// building what they can.
	if s := c.expandScore(); s > bestScore && c.Stock.Covers(expandCost) {
		c.Stock = c.Stock.Sub(expandCost)
		c.Expanding = 60
		c.chronicle(tick, "%s, ton peuple repousse les bords de l'île.", Expand.motive())
		return
	}
	if bestKind < 0 || !c.Stock.Covers(c.Cost(bestKind)) {
		return // waiting for resources is a decision too
	}
	p, ok := c.place(bestKind, r)
	if !ok {
		if !c.Crowded {
			c.Crowded = true
			c.chronicle(tick, "Ton peuple manque de place pour bâtir %s.", Specs[bestKind].Name)
		}
		return
	}
	c.Crowded = false
	c.Stock = c.Stock.Sub(c.Cost(bestKind))
	c.Buildings = append(c.Buildings, Building{ID: c.NextID, Kind: bestKind, X: p.X, Y: p.Y})
	c.NextID++
	c.chronicle(tick, "%s, ton peuple entreprend de bâtir %s.", c.motiveFor(bestKind).motive(), Specs[bestKind].Name)
}

// score is the utility of building one more of kind k.
func (c *Civ) score(k Kind) float64 {
	s := Specs[k]
	base := 0.0
	for i, a := range s.Affinity {
		base += a * c.Conviction[i]
	}
	need := 1.0
	switch k {
	case House:
		if c.Population+2 >= float64(c.Housing()) {
			need = 2.5 // the people are cramped
		} else {
			need = 0.3
		}
	case Farm:
		if c.foodBalance() < 0.05 || c.FarmUrge > 0 {
			need = 2.5 // not enough food coming in
		}
	case Warehouse:
		cap, full := c.Cap(), 0
		for _, v := range c.Stock {
			if v > 0.85*cap {
				full++
			}
		}
		if full >= 2 {
			need = 1.5
		}
	case Tower:
		need = 1 / (1 + 0.5*float64(c.Count(Tower))) // towers need defenders: fewer, stronger
	case Barracks:
		if c.Count(Barracks) > 0 {
			need = 0.15
		}
	}
	if s.Jobs > 0 && c.freeWorkers() < float64(s.Jobs)/2 && need < 2 {
		need *= 0.3 // nobody would work there
	}
	// Diminishing returns: each extra building of a kind is worth less.
	score := base * need / (1 + 0.5*float64(c.Count(k)))
	// Survival does not wait for the god's wishes: a hungry or cramped people
	// builds farms and houses whatever the pie says.
	if need >= 2.5 && (k == Farm || k == House) {
		score = math.Max(score, survivalScore)
	}
	return score
}

func (c *Civ) expandScore() float64 {
	if c.Size >= maxSize {
		return 0
	}
	crowd := float64(len(c.Buildings)) / (c.Size * c.Size / 100)
	return c.Conviction[Expand] * (0.4 + 0.5*crowd)
}

// motiveFor returns the intention that weighs most in building kind k.
func (c *Civ) motiveFor(k Kind) Intention {
	best, bestV := Prosper, -1.0
	for i, a := range Specs[k].Affinity {
		if v := a * c.Conviction[i]; v > bestV {
			best, bestV = Intention(i), v
		}
	}
	return best
}

func (c *Civ) jobs() float64 {
	n := 0
	for _, b := range c.Buildings {
		n += Specs[b.Kind].Jobs
	}
	return float64(n)
}

func (c *Civ) freeWorkers() float64 { return c.Population - c.jobs() }

// foodBalance estimates the food gained per second at full employment,
// averaged over a day and a night.
func (c *Civ) foodBalance() float64 {
	in := coreOutput[Food]
	for _, b := range c.Buildings {
		if b.Kind == Farm {
			in += float64(Specs[Farm].Jobs) * Specs[Farm].Output
		}
	}
	return in*dayAverage - c.Population*foodPerPerson - (c.Defenders+c.Assailants)*foodPerSoldier
}

// place picks a free spot for a building of kind k: inside the island, off
// the cliffs, without overlapping anything, and where the kind prefers to be
// (GDD 3.5).
func (c *Civ) place(k Kind, r *rand.Rand) (geo.Vec2, bool) {
	outline := geo.Outline(c.Seed)
	s := Specs[k]
	best, bestScore, found := geo.Vec2{}, math.Inf(-1), false
	for range placementTries {
		a := r.Float64() * 2 * math.Pi
		d := c.Size * 0.9 * math.Sqrt(r.Float64())
		p := geo.Vec2{X: d * math.Cos(a), Y: d * math.Sin(a)}
		if !c.free(outline, p, s.Radius) {
			continue
		}
		if sc := c.preference(k, p); sc > bestScore {
			best, bestScore, found = p, sc, true
		}
	}
	return best, found
}

// free reports whether a footprint of radius rad fits at p.
func (c *Civ) free(outline []geo.Vec2, p geo.Vec2, rad float64) bool {
	if geo.EdgeDistance(outline, c.Size, p) < rad+edgeMargin || !geo.Inside(outline, c.Size, p) {
		return false
	}
	for _, b := range c.Buildings {
		if p.Dist(geo.Vec2{X: b.X, Y: b.Y}) < rad+Specs[b.Kind].Radius+placementMargin {
			return false
		}
	}
	for _, f := range c.Features {
		if p.Dist(geo.Vec2{X: f.X, Y: f.Y}) < rad+f.Radius+placementMargin {
			return false
		}
	}
	return true
}

// preference scores a spot for a kind: higher is better.
func (c *Civ) preference(k Kind, p geo.Vec2) float64 {
	s := Specs[k]
	if s.Near != NoFeature {
		d := math.Inf(1)
		for _, f := range c.Features {
			if f.Kind == s.Near {
				d = math.Min(d, p.Dist(geo.Vec2{X: f.X, Y: f.Y})-f.Radius)
			}
		}
		return -d
	}
	switch k {
	case House:
		return -p.Len() // close to the heart
	case Tower:
		return p.Len() // on the rim
	default:
		return -math.Abs(p.Len() - c.Size*0.45)
	}
}

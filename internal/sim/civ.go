package sim

import (
	"fmt"
	"math"
	"math/rand/v2"

	"apogee/internal/geo"
)

// Tunables of the civilisation (GDD 🔧).
const (
	startSize        = 50.0 // island radius at birth, in island units
	maxSize          = 85.0
	expandStep       = 6.0
	baseStorage      = 100.0
	convictionTau    = 90.0 // seconds for the conviction to cover ~63 % of a change
	cultureTau       = 3 * 3600.0
	resistance       = 0.7  // conviction speed outside the dominant family
	nightProduction  = 0.6  // production factor at night (GDD 3.6)
	foodPerPerson    = 0.02 // per second
	foodPerSoldier   = 0.03 // per second
	growthPerSecond  = 0.02 // people born per second while fed and housed
	decisionInterval = 5.0  // seconds between two decisions of the people
	trainInterval    = 10.0 // seconds to train one soldier
	farFromFeature   = 0.5  // output factor of a producer far from its site
	featureReach     = 16.0 // distance at which a producer works its site
	chronicleSize    = 40
	famineLoss       = 0.002 // share of the people lost per second of famine
	militaryShare    = 0.35  // at most this share of the people under arms
	// dayAverage is the mean production factor over a day/night cycle:
	// 21 minutes at full rate, 9 at night rate (GDD 3.6).
	dayAverage = (21 + 9*nightProduction) / 30
)

var (
	trainCost  = Stock{Food: 5, Wood: 3}
	expandCost = Stock{Wood: 40, Stone: 40}
	coreOutput = Stock{Wood: 0.05, Stone: 0.05, Food: 0.1, Aether: 0.01}
)

// Entry is one line of the chronicle (GDD 6.7).
type Entry struct {
	Tick uint64
	Text string
}

// Civ is the civilisation living on an island: its people, economy and
// buildings. It is the same for humans and rival AIs (GDD 9.2).
type Civ struct {
	Seed       uint64
	Size       float64
	Stock      Stock
	Population float64
	Defenders  float64
	Assailants float64
	Knowledge  float64
	Buildings  []Building
	Features   []Feature
	Pie        Pie
	Conviction [NumIntentions]float64
	Culture    [NumFamilies]float64
	Chronicle  []Entry

	// Timers, in seconds of simulated time.
	SinceDecision float64
	SinceTraining float64
	// Expanding is the remaining time of an expansion in progress, or 0.
	Expanding float64
	// Starving is set during a famine, Crowded when the people found no
	// room for what they wanted to build. Both keep the chronicle from
	// repeating itself.
	Starving bool
	Crowded  bool

	NextID uint32
}

// NewCiv founds a civilisation on a fresh island.
func NewCiv(seed uint64) Civ {
	c := Civ{
		Seed:       seed,
		Size:       startSize,
		Stock:      Stock{Wood: 60, Stone: 40, Food: 60, Aether: 10},
		Population: 6,
		Pie:        DefaultPie,
	}
	for i, v := range c.Pie {
		c.Conviction[i] = float64(v) / 100
	}
	c.updateCulture(0)
	c.Culture = c.familyConviction()
	c.Features = features(seed)
	c.NextID = 1
	c.Buildings = []Building{{ID: c.NextID, Kind: Core, Progress: 1}}
	c.NextID++
	return c
}

// features scatters the natural sites of an island from its seed.
func features(seed uint64) []Feature {
	r := rand.New(rand.NewPCG(seed, 0x2545f4914f6cdd1d))
	outline := geo.Outline(seed)
	var fs []Feature
	add := func(kind FeatureKind, n int, radius, lo, hi float64) {
		for tries := 0; n > 0 && tries < 200; tries++ {
			a := r.Float64() * 2 * math.Pi
			d := startSize * (lo + r.Float64()*(hi-lo))
			p := geo.Vec2{X: d * math.Cos(a), Y: d * math.Sin(a)}
			if geo.EdgeDistance(outline, startSize, p) < radius+1 {
				continue
			}
			ok := p.Len() > Specs[Core].Radius+radius+2
			for _, f := range fs {
				ok = ok && p.Dist(geo.Vec2{X: f.X, Y: f.Y}) > f.Radius+radius+2
			}
			if ok {
				fs = append(fs, Feature{Kind: kind, X: p.X, Y: p.Y, Radius: radius})
				n--
			}
		}
	}
	add(Grove, 3, 7, 0.55, 0.75)
	add(Rock, 3, 3, 0.35, 0.8)
	add(Crystal, 2, 2.5, 0.5, 0.8)
	return fs
}

// Housing is how many people the island can shelter.
func (c *Civ) Housing() int {
	n := 0
	for _, b := range c.Buildings {
		if b.Done() {
			n += Specs[b.Kind].Housing
		}
	}
	return n
}

// Cap is the storage cap of every resource (GDD 4.3).
func (c *Civ) Cap() float64 {
	cap := baseStorage
	for _, b := range c.Buildings {
		if b.Done() {
			cap += Specs[b.Kind].Storage
		}
	}
	return cap
}

// Count returns how many buildings of a kind stand or are being built.
func (c *Civ) Count(k Kind) int {
	n := 0
	for _, b := range c.Buildings {
		if b.Kind == k {
			n++
		}
	}
	return n
}

// Defense is the island's strength against attacks: towers, the core and
// the defenders.
func (c *Civ) Defense() float64 {
	d := c.Defenders
	for _, b := range c.Buildings {
		if b.Done() {
			d += Specs[b.Kind].Defense
		}
	}
	return d
}

// SetPie changes what the god whispers. The conviction of the people follows
// slowly (GDD 6.2).
func (c *Civ) SetPie(p Pie) error {
	if err := p.Validate(); err != nil {
		return err
	}
	c.Pie = p
	return nil
}

// Dominant returns the family that dominates the culture. Ties go to the
// family with the largest share of the current pie (GDD 5.3).
func (c *Civ) Dominant() Family {
	best := Family(0)
	pie := c.familyPie()
	for f := Family(1); f < NumFamilies; f++ {
		switch {
		case c.Culture[f] > c.Culture[best]+1e-9:
			best = f
		case math.Abs(c.Culture[f]-c.Culture[best]) <= 1e-9 && pie[f] > pie[best]:
			best = f
		}
	}
	return best
}

func (c *Civ) familyPie() [NumFamilies]int {
	var out [NumFamilies]int
	for i, v := range c.Pie {
		out[Intention(i).Family()] += v
	}
	return out
}

func (c *Civ) familyConviction() [NumFamilies]float64 {
	var out [NumFamilies]float64
	for i, v := range c.Conviction {
		out[Intention(i).Family()] += v
	}
	return out
}

func (c *Civ) chronicle(tick uint64, format string, args ...any) {
	c.Chronicle = append(c.Chronicle, Entry{Tick: tick, Text: fmt.Sprintf(format, args...)})
	if len(c.Chronicle) > chronicleSize {
		c.Chronicle = append(c.Chronicle[:0:0], c.Chronicle[len(c.Chronicle)-chronicleSize:]...)
	}
}

// Step advances the civilisation by dt seconds. Night slows production down
// (GDD 3.6); tick dates chronicle entries and seeds decisions.
func (c *Civ) Step(dt float64, night bool, tick uint64) {
	c.updateConviction(dt)
	c.updateCulture(dt)
	c.produce(dt, night)
	c.feed(dt, tick)
	c.build(dt, night, tick)
	c.train(dt)
	c.SinceDecision += dt
	if c.SinceDecision >= decisionInterval {
		c.SinceDecision -= decisionInterval
		c.decide(tick)
	}
}

func (c *Civ) updateConviction(dt float64) {
	dom := c.Dominant()
	for i := range c.Conviction {
		target := float64(c.Pie[i]) / 100
		k := 1 - math.Exp(-dt/convictionTau)
		if Intention(i).Family() != dom && target > c.Conviction[i] {
			k *= resistance // the culture resists foreign wishes (GDD 6.6)
		}
		c.Conviction[i] += (target - c.Conviction[i]) * k
	}
}

func (c *Civ) updateCulture(dt float64) {
	fc := c.familyConviction()
	k := 1 - math.Exp(-dt/cultureTau)
	for f := range c.Culture {
		c.Culture[f] += (fc[f] - c.Culture[f]) * k
	}
}

// needs weighs each resource: scarcity plus what the intentions call for.
func (c *Civ) needs() [NumResources]float64 {
	cap := c.Cap()
	var n [NumResources]float64
	for r := range n {
		n[r] = 1 - c.Stock[r]/cap
	}
	cv := c.Conviction
	n[Food] += cv[Prosper]*0.6 + cv[Arm]*0.4
	n[Wood] += cv[Prosper]*0.4 + cv[Expand]*0.5 + cv[Arm]*0.3
	n[Stone] += cv[Fortify]*0.7 + cv[Expand]*0.5
	n[Aether] += cv[Study] * 0.9
	return n
}

// produce assigns workers to producers by need and collects their output.
// Assignment is recomputed every step: there are no individual workers to
// manage (GDD 4.4).
func (c *Civ) produce(dt float64, night bool) {
	f := 1.0
	if night {
		f = nightProduction
	}
	for r := range c.Stock {
		c.Stock[r] += coreOutput[r] * f * dt
	}
	needs := c.needs()
	weight := func(b Building) float64 {
		if b.Kind == Library {
			return c.Conviction[Study] + 0.2
		}
		return needs[Specs[b.Kind].Produces]
	}
	// Producers in order of need; ties keep building order (stable).
	order := make([]int, 0, len(c.Buildings))
	for i, b := range c.Buildings {
		if b.Done() && Specs[b.Kind].Jobs > 0 {
			order = append(order, i)
		}
	}
	for i := 1; i < len(order); i++ {
		for j := i; j > 0 && weight(c.Buildings[order[j]]) > weight(c.Buildings[order[j-1]]); j-- {
			order[j], order[j-1] = order[j-1], order[j]
		}
	}
	workers := c.Population
	for _, i := range order {
		b := c.Buildings[i]
		s := Specs[b.Kind]
		w := math.Min(workers, float64(s.Jobs))
		if w <= 0 {
			break
		}
		workers -= w
		out := w * s.Output * f * dt
		if s.Near != NoFeature && !c.nearFeature(b, s.Near) {
			out *= farFromFeature
		}
		if b.Kind == Library {
			c.Knowledge += out * (1 + c.Conviction[Study])
		} else {
			c.Stock[s.Produces] += out
		}
	}
	cap := c.Cap()
	for r := range c.Stock {
		c.Stock[r] = math.Min(c.Stock[r], cap) // the surplus is wasted (GDD 4.3)
	}
}

func (c *Civ) nearFeature(b Building, kind FeatureKind) bool {
	for _, f := range c.Features {
		if f.Kind == kind && math.Hypot(f.X-b.X, f.Y-b.Y) <= featureReach+f.Radius {
			return true
		}
	}
	return false
}

// feed makes the people eat; they grow when fed and housed, and dwindle in
// famine.
func (c *Civ) feed(dt float64, tick uint64) {
	eat := (c.Population*foodPerPerson + (c.Defenders+c.Assailants)*foodPerSoldier) * dt
	if c.Stock[Food] >= eat {
		c.Stock[Food] -= eat
		c.Starving = false
		if c.Stock[Food] > 0.3*c.Cap() && c.Population+1 <= float64(c.Housing()) {
			before := int(c.Population)
			c.Population += growthPerSecond * dt
			if int(c.Population) > before && int(c.Population)%10 == 0 {
				c.chronicle(tick, "Ton peuple compte désormais %d âmes.", int(c.Population))
			}
		}
		return
	}
	c.Stock[Food] = 0
	c.Population = math.Max(1, c.Population-famineLoss*c.Population*dt)
	if !c.Starving {
		c.Starving = true
		c.chronicle(tick, "La famine frappe : ton peuple dépérit.")
	}
}

// build advances the construction in progress, or the expansion.
func (c *Civ) build(dt float64, night bool, tick uint64) {
	f := 1.0
	if night {
		f = nightProduction
	}
	for i := range c.Buildings {
		b := &c.Buildings[i]
		if b.Done() {
			continue
		}
		b.Progress += dt / Specs[b.Kind].BuildTime * f
		if b.Done() {
			b.Progress = 1
			c.chronicle(tick, "Ton peuple a achevé %s.", Specs[b.Kind].Name)
		}
	}
	if c.Expanding > 0 {
		c.Expanding -= dt * f
		if c.Expanding <= 0 {
			c.Expanding = 0
			c.Size = math.Min(c.Size+expandStep, maxSize)
			c.chronicle(tick, "L'île s'est agrandie : de nouvelles terres s'offrent à ton peuple.")
		}
	}
}

// train turns people into soldiers, as the intentions ask (GDD 6.3).
func (c *Civ) train(dt float64) {
	c.SinceTraining += dt
	if c.SinceTraining < trainInterval {
		return
	}
	c.SinceTraining = 0
	soldiers := c.Defenders + c.Assailants
	if c.Starving && soldiers >= 1 {
		// In a famine, soldiers go back to the fields.
		if c.Assailants >= 1 {
			c.Assailants--
		} else {
			c.Defenders--
		}
		c.Population++
		return
	}
	if c.Population < 4 || !c.Stock.Covers(trainCost) || soldiers+1 > militaryShare*(c.Population+soldiers) {
		return
	}
	towers := float64(c.Count(Tower))
	wantDef := towers + c.Population*0.25*c.Conviction[Fortify]
	wantAss := 0.0
	if c.hasDone(Barracks) {
		wantAss = c.Population * 0.4 * c.Conviction[Arm]
	}
	switch {
	case c.Defenders+1 <= wantDef && (c.Defenders/math.Max(wantDef, 1) <= c.Assailants/math.Max(wantAss, 1) || wantAss == 0):
		c.Defenders++
	case c.Assailants+1 <= wantAss:
		c.Assailants++
	default:
		return
	}
	c.Population--
	c.Stock = c.Stock.Sub(trainCost)
}

func (c *Civ) hasDone(k Kind) bool {
	for _, b := range c.Buildings {
		if b.Kind == k && b.Done() {
			return true
		}
	}
	return false
}

func (c *Civ) building() bool {
	for _, b := range c.Buildings {
		if !b.Done() {
			return true
		}
	}
	return c.Expanding > 0
}

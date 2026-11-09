package sim

import (
	"errors"
	"fmt"
	"math"
)

// Raids (GDD 7): the god whispers an outer intention towards another island,
// the people carry it out in their own time. Units cross the sky, visible to
// all, and the target is warned at their departure.

// Outer is an intention turned towards another island (GDD 6.3).
type Outer int

const (
	NoOuter    Outer = iota
	Covet            // gather and raid when ready
	Punish           // raid at once with what is at hand
	Annihilate       // a long assault on the heart of a declining island
	Rescue           // send defenders to a neighbour
	NumOuter
)

// Name returns the intention's verb, as shown to the player.
func (o Outer) Name() string {
	return [...]string{"", "Convoiter", "Punir", "Anéantir", "Secourir"}[o]
}

// OuterIntent is the outer intention of a people, with its target.
type OuterIntent struct {
	Kind   Outer
	Target IslandID
	// Since is the time in seconds since it was whispered.
	Since float64
}

// Raid tunables (GDD 🔧).
const (
	raidSpeed       = 2.0  // sky units per second
	pillageShare    = 0.25 // at most this share of the target's stock (GDD 7.4)
	carryPerUnit    = 10.0 // resources an assailant carries home
	shieldTime      = 15 * 60.0
	newcomerShield  = 12 * 3600.0 // GDD 11.3
	covetTimeout    = 20 * 60.0   // a coveting people goes with what it has
	annihilatePrep  = 10 * 60.0
	rescueStay      = 30 * 60.0
	minimalGuard    = 2.0 // defenders a rescuing people keeps at home
	weakTargetRatio = 0.6 // below this power ratio, pillage is restricted
)

// RaidKind tells what a group crossing the sky intends.
type RaidKind int

const (
	Pillage RaidKind = iota
	Assault          // Annihilate: aims at the heart
	Relief           // Rescue: defenders helping a neighbour
)

// Raid is a group of units crossing the sky.
type Raid struct {
	ID       uint32
	Kind     RaidKind
	From, To IslandID
	Units    float64
	// Progress goes from 0 to 1 on the way out, and back to 0 on the way
	// home.
	Progress  float64
	Returning bool
	Loot      Stock
	// Stay is the time a relief force remains on the island it helps.
	Stay float64
}

// Errors returned by Whisper for an impossible outer intention.
var (
	ErrNoTarget     = errors.New("no such island")
	ErrOwnIsland    = errors.New("a people cannot raid its own island")
	ErrProtected    = errors.New("this young civilisation is still protected")
	ErrNotDeclining = errors.New("only a declining island can be annihilated")
	ErrShielded     = errors.New("this island was pillaged by you a moment ago")
)

// Whisper sets the outer intention of an island's people (GDD 6.3). It
// checks the rules that protect targets (GDD 7.5, 11.3); the people then
// carry it out in their own time.
func (s *Sky) Whisper(from IslandID, kind Outer, target IslandID) error {
	is := s.island(from)
	if is == nil {
		return ErrNoTarget
	}
	if kind == NoOuter {
		s.recall(is)
		return nil
	}
	t := s.island(target)
	switch {
	case t == nil || t.Falling > 0:
		return ErrNoTarget
	case t.ID == from:
		return ErrOwnIsland
	case kind != Rescue && s.protected(t):
		return ErrProtected
	case kind == Annihilate && !t.Declining:
		return ErrNotDeclining
	case kind != Rescue && t.ShieldFrom == from && t.ShieldLeft > 0:
		return ErrShielded
	}
	if kind != Rescue {
		is.Aggressor = true // raiding ends a newcomer's own protection (GDD 11.3)
	}
	is.Civ.Outer = OuterIntent{Kind: kind, Target: target}
	switch kind {
	case Covet:
		t.Civ.chronicle(s.Tick, "%s nous regarde avec envie.", is.Name)
	case Annihilate:
		t.Civ.chronicle(s.Tick, "%s prépare notre anéantissement : sa bannière de guerre se dresse vers nous.", is.Name)
		for i := range s.Islands {
			if o := &s.Islands[i]; o.ID != t.ID && o.ID != is.ID {
				o.Civ.chronicle(s.Tick, "%s a juré la perte de %s.", is.Name, t.Name)
			}
		}
	}
	return nil
}

// Protected reports whether a young civilisation is still shielded from
// raids: during its first hours, unless it raided itself (GDD 11.3).
func (s *Sky) Protected(is *Island) bool { return s.protected(is) }

func (s *Sky) protected(is *Island) bool {
	return !is.Aggressor && s.age(is) < newcomerShield
}

// age returns how long an island has lived, in seconds.
func (s *Sky) age(is *Island) float64 {
	return float64(s.Tick-is.Born) * TickDuration.Seconds()
}

// recall brings every unit of an island home (GDD 6.3, Rappeler).
func (s *Sky) recall(is *Island) {
	is.Civ.Outer = OuterIntent{}
	for i := range s.Raids {
		if r := &s.Raids[i]; r.From == is.ID {
			r.Returning = true
			r.Stay = 0
		}
	}
}

// raids runs once per tick: departures, travel, fights and returns.
func (s *Sky) raids(dt float64) {
	for i := range s.Islands {
		is := &s.Islands[i]
		if is.ShieldLeft > 0 {
			is.ShieldLeft = math.Max(0, is.ShieldLeft-dt)
		}
		if is.Falling == 0 && is.Civ.Outer.Kind != NoOuter {
			s.carryOut(is, dt)
		}
	}
	kept := s.Raids[:0]
	for _, r := range s.Raids {
		if s.advance(&r, dt) {
			kept = append(kept, r)
		}
	}
	s.Raids = kept
}

// carryOut lets a people act on its outer intention when it judges the
// moment right.
func (s *Sky) carryOut(is *Island, dt float64) {
	c := &is.Civ
	o := &c.Outer
	o.Since += dt
	t := s.island(o.Target)
	if t == nil || t.Falling > 0 {
		*o = OuterIntent{}
		return
	}
	switch o.Kind {
	case Covet:
		// Ready when the visible force of the target can be beaten, or
		// after a while with whatever is at hand.
		if c.Assailants >= 1 && (s.strike(is) > (t.Civ.Defense()+s.relief(t.ID))*1.2 || o.Since > covetTimeout) {
			s.launch(is, t, Pillage, c.Assailants)
			*o = OuterIntent{}
		}
	case Punish:
		if c.Assailants >= 1 {
			s.launch(is, t, Pillage, c.Assailants)
		}
		*o = OuterIntent{}
	case Annihilate:
		if !t.Declining {
			*o = OuterIntent{}
			return
		}
		if o.Since >= annihilatePrep && c.Assailants >= 1 {
			s.launch(is, t, Assault, c.Assailants)
			*o = OuterIntent{}
		}
	case Rescue:
		if spare := c.Defenders - minimalGuard; spare >= 1 {
			s.launch(is, t, Relief, math.Floor(spare))
		}
		*o = OuterIntent{}
	}
}

// strike is the strength of an island's assailants.
func (s *Sky) strike(is *Island) float64 {
	return is.Civ.Assailants * (1 + is.Civ.Tech.Mods().AssailantStrength)
}

func (s *Sky) launch(from, to *Island, kind RaidKind, units float64) {
	switch kind {
	case Relief:
		from.Civ.Defenders -= units
	default:
		from.Civ.Assailants -= units
	}
	s.NextRaid++
	s.Raids = append(s.Raids, Raid{ID: s.NextRaid, Kind: kind, From: from.ID, To: to.ID, Units: units})
	verb := map[RaidKind]string{Pillage: "lance un raid", Assault: "lance l'assaut final", Relief: "envoie des renforts"}[kind]
	to.Civ.chronicle(s.Tick, "%s %s vers nous : %.0f %s traversent le ciel.", from.Name, verb, units, plural(units, "guerrier", "guerriers"))
	from.Civ.chronicle(s.Tick, "Ton peuple %s vers %s.", verb, to.Name)
}

func plural(n float64, one, many string) string {
	if math.Round(n) <= 1 {
		return one
	}
	return many
}

// distance between two islands, in sky units.
func distance(a, b *Island) float64 {
	ax, ay := a.Radius*math.Cos(a.Angle), a.Radius*math.Sin(a.Angle)
	bx, by := b.Radius*math.Cos(b.Angle), b.Radius*math.Sin(b.Angle)
	return math.Hypot(ax-bx, ay-by)
}

// advance moves a raid and resolves it on arrival; it reports whether the
// raid is still underway.
func (s *Sky) advance(r *Raid, dt float64) bool {
	from, to := s.island(r.From), s.island(r.To)
	if from == nil || from.Falling > 0 {
		return false // their home fell: the units are lost with it
	}
	if r.Returning {
		d := 1.0
		if to != nil {
			d = math.Max(distance(from, to), 1)
		}
		r.Progress -= raidSpeed * dt / d
		if r.Progress <= 0 {
			s.homecoming(from, r)
			return false
		}
		return true
	}
	if to == nil || to.Falling > 0 {
		r.Returning = true
		return true
	}
	if r.Stay > 0 {
		r.Stay -= dt
		if r.Stay <= 0 {
			r.Returning = true
		}
		return true
	}
	r.Progress += raidSpeed * dt / math.Max(distance(from, to), 1)
	if r.Progress < 1 {
		return true
	}
	r.Progress = 1
	s.clash(from, to, r)
	return true
}

// clash resolves the arrival of a group, as an autobattler (GDD 6.9).
func (s *Sky) clash(from, to *Island, r *Raid) {
	if r.Kind == Relief {
		r.Stay = rescueStay
		to.Civ.chronicle(s.Tick, "Les renforts de %s veillent sur notre île.", from.Name)
		return
	}
	atk := r.Units * (1 + from.Civ.Tech.Mods().AssailantStrength)
	def := to.Civ.Defense() + s.relief(to.ID)
	r.Returning = true
	if atk <= def {
		lost := math.Min(r.Units, math.Ceil(r.Units*0.6))
		r.Units -= lost
		to.Civ.chronicle(s.Tick, "Ton peuple a repoussé les guerriers de %s.", from.Name)
		from.Civ.chronicle(s.Tick, "Les guerriers ont été repoussés par %s : %.0f sont tombés.", to.Name, lost)
		return
	}
	// Victory: casualties on both sides follow the balance of forces.
	r.Units -= math.Floor(r.Units * def / atk * 0.5)
	to.Civ.Defenders = math.Max(0, to.Civ.Defenders-math.Ceil(to.Civ.Defenders*0.5))
	if r.Kind == Assault {
		harm := (atk - def) / atk * 0.5 / (1 + to.Civ.Tech.Mods().CoreArmor)
		to.Civ.CoreHealth -= harm
		if to.Civ.CoreHealth <= 0 {
			to.Civ.CoreHealth = 0
			to.Falling = fallDuration
			to.Cause = "aspirée par le tourbillon sous l'assaut de " + from.Name
			to.Civ.chronicle(s.Tick, "Les guerriers de %s ont brisé le cœur. L'île sombre dans le tourbillon.", from.Name)
			from.Civ.chronicle(s.Tick, "Tes guerriers ont achevé %s.", to.Name)
			return
		}
		to.Civ.chronicle(s.Tick, "L'assaut de %s a frappé le cœur (%d %% de vigueur restante).", from.Name, int(math.Round(to.Civ.CoreHealth*100)))
		return
	}
	// Pillage is limited (GDD 7.4): a share of the stock, less against a much
	// weaker island, and no more than the raiders can carry.
	share := pillageShare
	if ratio := to.Civ.Power() / math.Max(from.Civ.Power(), 1); ratio < weakTargetRatio {
		share *= ratio / weakTargetRatio
	}
	capacity := r.Units * carryPerUnit
	var total float64
	for res := range to.Civ.Stock {
		total += to.Civ.Stock[res] * share
	}
	k := 1.0
	if total > capacity {
		k = capacity / total
	}
	for res := range to.Civ.Stock {
		take := to.Civ.Stock[res] * share * k
		to.Civ.Stock[res] -= take
		r.Loot[res] += take
	}
	to.ShieldFrom, to.ShieldLeft = from.ID, shieldTime
	to.Civ.chronicle(s.Tick, "Les pillards de %s nous ont dépouillés de %.0f ressources.", from.Name, total*k)
}

// relief adds up the defenders other islands sent to help this one.
func (s *Sky) relief(id IslandID) float64 {
	sum := 0.0
	for _, r := range s.Raids {
		if r.Kind == Relief && r.To == id && r.Stay > 0 && !r.Returning {
			sum += r.Units
		}
	}
	return sum
}

func (s *Sky) homecoming(home *Island, r *Raid) {
	c := &home.Civ
	switch r.Kind {
	case Relief:
		c.Defenders += r.Units
	default:
		c.Assailants += r.Units
	}
	cap := c.Cap()
	got := 0.0
	for res := range c.Stock {
		c.Stock[res] = math.Min(cap, c.Stock[res]+r.Loot[res])
		got += r.Loot[res]
	}
	if got > 0 {
		c.chronicle(s.Tick, "Les guerriers sont rentrés chargés de %.0f ressources.", got)
	} else if r.Units >= 1 {
		c.chronicle(s.Tick, "%.0f %s sont rentrés.", r.Units, plural(r.Units, "guerrier", "guerriers"))
	}
}

// String describes an outer intention, for the chronicle and the HUD.
func (o OuterIntent) String() string {
	if o.Kind == NoOuter {
		return ""
	}
	return fmt.Sprintf("%s → île %d", o.Kind.Name(), o.Target)
}

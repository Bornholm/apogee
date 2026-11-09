package sim

import (
	"math/rand/v2"
	"sort"
	"strings"
)

// Rival AIs (GDD 9): an AI god has exactly the same people as a human and
// only acts by whispering the same intentions, with the same inertia. Its
// profile decides what it whispers.

// Profile is the personality of a rival AI god (GDD 9.4).
type Profile int

const (
	Warlike     Profile = iota // Belliqueux
	Cautious                   // Prudent
	Builder                    // Bâtisseur
	Opportunist                // Opportuniste
	NumProfiles
)

// Name returns the profile's name.
func (p Profile) Name() string {
	return [...]string{"Belliqueux", "Prudent", "Bâtisseur", "Opportuniste"}[p]
}

// basePie is what each profile whispers when all is well.
var basePie = [NumProfiles]Pie{
	Warlike:     {Arm: 50, Prosper: 30, Fortify: 20},
	Cautious:    {Fortify: 50, Prosper: 40, Study: 10},
	Builder:     {Prosper: 60, Expand: 30, Study: 10},
	Opportunist: {Prosper: 40, Arm: 30, Study: 30},
}

// AIPrefix marks the owner of an island held by a rival AI: AIs always say
// what they are (GDD 9.5).
const AIPrefix = "ai/"

// rivalInterval is how often an AI god reconsiders its pie, in seconds.
const rivalInterval = 90.0

// IsAI reports whether an island belongs to a rival AI.
func (is *Island) IsAI() bool { return strings.HasPrefix(is.Owner, AIPrefix) }

// FoundRival creates an island held by a rival AI god of the given profile.
func (s *Sky) FoundRival(name string, p Profile, seed uint64, angle float64) IslandID {
	id := s.Found(AIPrefix+name, name, seed, angle)
	s.island(id).Profile = p
	return id
}

// rivals lets every AI god think, each at its own moment.
func (s *Sky) rivals() {
	n := uint64(rivalInterval / TickDuration.Seconds())
	for i := range s.Islands {
		is := &s.Islands[i]
		if is.IsAI() && is.Falling == 0 && (s.Tick+uint64(is.ID))%n == 0 {
			s.rivalThink(is)
		}
	}
}

// rivalThink lets the AI god of an island adjust its pie to the situation.
func (s *Sky) rivalThink(is *Island) {
	c := &is.Civ
	r := rand.New(rand.NewPCG(is.Seed^0x9a1d, s.Tick))
	// An array, not a map: the noise below must be drawn in a fixed order
	// for the simulation to stay deterministic (GDD 14.3).
	var want [NumIntentions]float64
	for i, v := range basePie[is.Profile] {
		want[i] = float64(v)
	}
	if c.CoreHealth < 0.7 || is.Declining {
		want[Fortify] += 30 // a wounded or declining people bunkers down
	}
	if c.Starving {
		want[Prosper] += 30
	}
	// A little noise, so that rivals are not clockwork (GDD 9.3).
	for i := range want {
		want[i] *= 0.85 + 0.3*r.Float64()
	}
	p := toPie(want)
	if p != c.Pie {
		c.SetPie(p)
	}
	s.rivalAim(is, r)
	// An AI god answers the suppliques of its people like a human would,
	// following its own leanings.
	if c.Pending != nil {
		c.Answer(c.Pending.ID, c.leaning(*c.Pending), s.Tick)
	}
}

// rivalAim lets warlike and opportunist gods turn their people against a
// neighbour, by the same outer intentions a human whispers (GDD 9.4).
func (s *Sky) rivalAim(is *Island, r *rand.Rand) {
	c := &is.Civ
	if c.Outer.Kind != NoOuter || c.Assailants < 4 {
		return
	}
	// The weakest island that may be raided.
	var prey *Island
	for i := range s.Islands {
		t := &s.Islands[i]
		if t.ID == is.ID || t.Falling > 0 || s.protected(t) || (t.ShieldFrom == is.ID && t.ShieldLeft > 0) {
			continue
		}
		if prey == nil || t.Civ.Power() < prey.Civ.Power() {
			prey = t
		}
	}
	if prey == nil {
		return
	}
	switch is.Profile {
	case Warlike:
		if prey.Declining && r.Float64() < 0.3 {
			s.Whisper(is.ID, Annihilate, prey.ID)
		} else if r.Float64() < 0.25 {
			s.Whisper(is.ID, Covet, prey.ID)
		}
	case Opportunist:
		// Strike while the sky is busy with the colossus (GDD 9.4).
		if s.Boss != nil && s.Boss.In < 5*60 && r.Float64() < 0.5 {
			s.Whisper(is.ID, Punish, prey.ID)
		}
	}
}

// toPie turns weights into a valid pie: the three heaviest intentions, in
// steps of 10 %, adding up to 100 %.
func toPie(want [NumIntentions]float64) Pie {
	type w struct {
		i Intention
		v float64
	}
	ws := make([]w, 0, len(want))
	for i, v := range want {
		if v > 0 {
			ws = append(ws, w{Intention(i), v})
		}
	}
	sort.SliceStable(ws, func(a, b int) bool {
		if ws[a].v != ws[b].v {
			return ws[a].v > ws[b].v
		}
		return ws[a].i < ws[b].i
	})
	if len(ws) > MaxSlices {
		ws = ws[:MaxSlices]
	}
	total := 0.0
	for _, x := range ws {
		total += x.v
	}
	var p Pie
	left := 100
	for k, x := range ws {
		n := int(x.v/total*10+0.5) * 10
		if k == len(ws)-1 {
			n = left
		}
		n = min(n, left)
		p[x.i] = n
		left -= n
	}
	if left > 0 { // rounding: give the rest to the heaviest
		p[ws[0].i] += left
	}
	return p
}

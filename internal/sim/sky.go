// Package sim is Apogée's authoritative simulation. Only the server runs it
// (GDD 10.3); it has no rendering and no I/O, which keeps it deterministic and
// testable.
package sim

import (
	"math"
	"time"
)

// TickRate is the fixed simulation rate. An idle game needs little: 10 Hz is
// plenty for orbits, the day cycle and, later, autobattler combat.
const TickRate = 10

// TickDuration is the simulated time covered by one tick.
const TickDuration = time.Second / TickRate

// DayLength is one full day/night cycle of the sky (GDD 3.6).
const DayLength = 30 * time.Minute

// Phase is a moment of the day/night cycle.
type Phase int

const (
	Dawn Phase = iota
	Day
	Dusk
	Night
)

func (p Phase) String() string {
	return [...]string{"dawn", "day", "dusk", "night"}[p]
}

// phaseEnds lists, in order, the end of each phase within a cycle:
// dawn 3 min, day 15 min, dusk 3 min, night 9 min (GDD 3.6).
var phaseEnds = [...]time.Duration{3 * time.Minute, 18 * time.Minute, 21 * time.Minute, DayLength}

// IslandID identifies an island for the lifetime of the sky.
type IslandID uint32

// Island is one civilisation's island, as far as the sky is concerned.
type Island struct {
	ID   IslandID
	Name string
	Seed uint64
	// Orbit around the portal (GDD 3.1): distance in sky units and angle.
	Radius float64
	Angle  float64
	// Civ is the civilisation living on the island.
	Civ Civ

	// Owner is the player key of a human, or "ai/…" for a rival AI.
	Owner     string
	OwnerName string
	// Generation counts the civilisations of this owner (GDD 11.5).
	Generation int
	Born       uint64 // tick of the founding
	PeakPower  float64
	Declining  bool
	// Falling is the time left before the whirlpool swallows the island,
	// once its heart broke; 0 otherwise.
	Falling float64
	Cause   string
	// Profile is the personality of a rival AI god (GDD 9.4).
	Profile Profile
	// Aggressor is set once the island raided another: it loses the
	// protection of newcomers (GDD 11.3).
	Aggressor bool
	// ShieldFrom cannot pillage this island again for ShieldLeft seconds
	// (GDD 7.4).
	ShieldFrom IslandID
	ShieldLeft float64
}

// Sky is the whole simulated world.
type Sky struct {
	Seed    uint64
	Tick    uint64
	Islands []Island // ordered by ID: never iterate a map in the simulation
	NextID  IslandID

	// Wave and Boss are the next announced attacks (GDD 8).
	Wave *Threat
	Boss *Threat
	// Annals list the fallen civilisations (GDD 11.5).
	Annals []Annal
	// Waiting lists the peoples to be reborn at the next dawn.
	Waiting []Rebirth
	// Raids are the groups crossing the sky (GDD 7).
	Raids    []Raid
	NextRaid uint32
	// LastPhase is the time of day at the previous tick. A new sky starts
	// at dawn, so the first dawn does not count as a new one.
	LastPhase Phase
}

// Sky geometry, in sky units. New civilisations are born at the rim and
// descend towards the portal over their life (GDD 3.1).
const (
	RimRadius    = 1000.0
	PortalRadius = 120.0
	// rimPeriod is the time an island at the rim takes to orbit once.
	rimPeriod = 6 * time.Hour
)

// AddIsland creates an island without owner at the rim of the sky and
// returns its ID.
func (s *Sky) AddIsland(name string, seed uint64, angle float64) IslandID {
	return s.Found("", name, seed, angle)
}

// Found creates an island at the rim for an owner and returns its ID. New
// civilisations are always born at the rim (GDD 3.1).
func (s *Sky) Found(owner, ownerName string, seed uint64, angle float64) IslandID {
	s.NextID++
	s.Islands = append(s.Islands, Island{
		ID: s.NextID, Name: islandName(ownerName, seed), Seed: seed,
		Radius: RimRadius, Angle: math.Mod(angle, 2*math.Pi),
		Civ:   NewCiv(seed),
		Owner: owner, OwnerName: ownerName, Generation: 1, Born: s.Tick,
	})
	c := &s.Islands[len(s.Islands)-1].Civ
	if t := c.Trait; t != NoTrait {
		c.chronicle(s.Tick, "Ton peuple découvre son île : %s. %s", lower(t.Name()), t.Text())
	}
	return s.NextID
}

// islandName names a civilisation after its owner.
func islandName(owner string, seed uint64) string {
	if owner == "" {
		return "Île sans nom"
	}
	return owner
}

// IslandOf returns the living island of an owner.
func (s *Sky) IslandOf(owner string) (IslandID, bool) {
	for _, is := range s.Islands {
		if is.Owner == owner {
			return is.ID, true
		}
	}
	return 0, false
}

func (s *Sky) island(id IslandID) *Island {
	for i := range s.Islands {
		if s.Islands[i].ID == id {
			return &s.Islands[i]
		}
	}
	return nil
}

// Island returns the island with the given ID, or nil.
func (s *Sky) Island(id IslandID) *Island { return s.island(id) }

// AddIslandAt creates an island at a given distance from the portal, clamped
// between the portal and the rim. It serves the demo rivals until the real
// rival AIs exist (GDD section 9).
func (s *Sky) AddIslandAt(name string, seed uint64, radius, angle float64) IslandID {
	id := s.AddIsland(name, seed, angle)
	s.Islands[len(s.Islands)-1].Radius = math.Min(math.Max(radius, PortalRadius), RimRadius)
	return id
}

// Step advances the sky by one tick.
func (s *Sky) Step() {
	s.Tick++
	dt := TickDuration.Seconds()
	phase, _ := s.TimeOfDay()
	for i := range s.Islands {
		is := &s.Islands[i]
		is.Angle = math.Mod(is.Angle+AngularSpeed(is.Radius)*dt, 2*math.Pi)
		if is.Falling == 0 {
			is.Civ.Step(dt, phase, s.Tick)
		}
	}
	s.lifecycle(dt, phase)
	s.raids(dt)
	s.rivals()
}

// AngularSpeed is the orbital speed in radians per second at a distance from
// the portal. Inner orbits turn faster, like planets (r^-1.5), so islands near
// the portal change neighbours more often.
func AngularSpeed(radius float64) float64 {
	base := 2 * math.Pi / rimPeriod.Seconds()
	return base * math.Pow(RimRadius/math.Max(radius, PortalRadius), 1.5)
}

// Elapsed is the simulated time since the sky was created.
func (s *Sky) Elapsed() time.Duration { return time.Duration(s.Tick) * TickDuration }

// TimeOfDay returns the current phase and the time left before the next one.
func (s *Sky) TimeOfDay() (Phase, time.Duration) {
	t := s.Elapsed() % DayLength
	for i, end := range phaseEnds {
		if t < end {
			return Phase(i), end - t
		}
	}
	return Night, DayLength - t
}

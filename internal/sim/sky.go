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
}

// Sky is the whole simulated world.
type Sky struct {
	Tick    uint64
	Islands []Island // ordered by ID: never iterate a map in the simulation
	nextID  IslandID
}

// Sky geometry, in sky units. New civilisations are born at the rim and
// descend towards the portal over their life (GDD 3.1).
const (
	RimRadius    = 1000.0
	PortalRadius = 120.0
	// rimPeriod is the time an island at the rim takes to orbit once.
	rimPeriod = 6 * time.Hour
)

// AddIsland creates an island at the rim of the sky and returns its ID.
func (s *Sky) AddIsland(name string, seed uint64, angle float64) IslandID {
	s.nextID++
	s.Islands = append(s.Islands, Island{
		ID: s.nextID, Name: name, Seed: seed,
		Radius: RimRadius, Angle: math.Mod(angle, 2*math.Pi),
		Civ: NewCiv(seed),
	})
	return s.nextID
}

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
		is.Civ.Step(dt, phase, s.Tick)
	}
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

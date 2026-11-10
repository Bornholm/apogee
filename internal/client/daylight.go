package client

import (
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

// The day/night cycle tints the whole scene (GDD 12.5): the faceted sprites
// keep their baked light, the palette shifts warm at dawn and dusk, blue and
// darker at night. Night stays bright enough to read at a glance.

type rgb struct{ r, g, b float32 }

var (
	dayTint   = rgb{1, 1, 1}
	duskTint  = rgb{1.0, 0.82, 0.70}
	nightTint = rgb{0.42, 0.48, 0.76}
)

// phaseLength is the duration of each phase of the cycle (GDD 3.6).
var phaseLength = map[string]time.Duration{
	"dawn": 3 * time.Minute, "day": 15 * time.Minute, "dusk": 3 * time.Minute, "night": 9 * time.Minute,
}

// tintAt returns the tint of a moment: a phase and the time left in it.
// Dawn goes from night through the warm tint to day; dusk the other way.
func tintAt(phase string, left time.Duration) rgb {
	total := phaseLength[phase]
	if total == 0 {
		return dayTint
	}
	t := 1 - float32(left)/float32(total) // 0 at the start of the phase, 1 at its end
	t = min(max(t, 0), 1)
	switch phase {
	case "dawn":
		if t < 0.5 {
			return mix(nightTint, duskTint, t*2)
		}
		return mix(duskTint, dayTint, t*2-1)
	case "dusk":
		if t < 0.5 {
			return mix(dayTint, duskTint, t*2)
		}
		return mix(duskTint, nightTint, t*2-1)
	case "night":
		return nightTint
	}
	return dayTint
}

func mix(a, b rgb, t float32) rgb {
	if t >= 1 {
		return b // exactly, for the turn of the phases
	}
	return rgb{a.r + (b.r-a.r)*t, a.g + (b.g-a.g)*t, a.b + (b.b-a.b)*t}
}

// daylight returns the colour scale of the current moment.
func (g *Game) daylight() ebiten.ColorScale {
	var cs ebiten.ColorScale
	phase, left, ok := g.moment()
	if !ok {
		return cs
	}
	t := tintAt(phase, left)
	cs.Scale(t.r, t.g, t.b, 1)
	return cs
}

// moment returns the time of day and the time left in it, or the forced
// phase, half gone.
func (g *Game) moment() (string, time.Duration, bool) {
	if g.ForcePhase != "" {
		return g.ForcePhase, phaseLength[g.ForcePhase] / 2, true
	}
	return g.src.Phase(time.Now())
}

// dark tells how dark it is now, from 0 to 1 (see darkness).
func (g *Game) dark() float64 {
	phase, left, ok := g.moment()
	if !ok {
		return 0
	}
	return darkness(phase, left)
}

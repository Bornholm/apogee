package client

import (
	"image/color"
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"apogee/internal/art"
	"apogee/internal/proto"
	"apogee/internal/sim"
)

// At dusk the island lights up (GDD 12.5): the windows of the houses, the
// braziers of the towers, the heart's aether, the lanterns the villagers
// carry. Lights escape the night's tint and add to what lies behind them,
// in soft rings of pixels.

type lampKind int

const (
	windowLamp  lampKind = iota // a warm window
	brazierLamp                 // a tower's fire, flickering
	aetherLamp                  // the heart's or a crystal's glow, pulsing
	lanternLamp                 // carried by a villager or a guard
	relicLamp                   // the golden crystal of a relic's stele
)

// lamp is a light of the island, in island units.
type lamp struct {
	x, y, z float64
	kind    lampKind
	// by is the depth of what holds the light, to draw it behind what
	// stands in front.
	by   float64
	seed uint32
	big  bool // the heart
	// festive lamps, the lanterns and the bonfire of a feast, shine by
	// day as well.
	festive bool
}

// Halo radii, in virtual pixels: small, medium, large.
var haloRadii = [...]int{4, 7, 14}

var (
	windowColour  = color.RGBA{0xff, 0xd2, 0x7a, 0xff}
	fireColour    = color.RGBA{0xff, 0x9a, 0x3c, 0xff}
	aetherColour  = color.RGBA{0x8a, 0xe0, 0xff, 0xff}
	lanternColour = color.RGBA{0xff, 0xc0, 0x60, 0xff}
)

// darkness tells how dark it is, from 0 in daylight to 1 at night: lights
// come on in the second half of dusk and go out in the first half of dawn.
func darkness(phase string, left time.Duration) float64 {
	total := phaseLength[phase]
	if total == 0 {
		return 0
	}
	t := 1 - float64(left)/float64(total)
	switch phase {
	case "night":
		return 1
	case "dusk":
		return ease((t - 0.3) / 0.6)
	case "dawn":
		return 1 - ease((t-0.1)/0.5)
	}
	return 0
}

// islandLamps lists the lights of an island's finished buildings.
func islandLamps(sum proto.IslandSummary, own *proto.Own) []lamp {
	var out []lamp
	if c, ok := relicSpot(sum, own); ok {
		out = append(out, lamp{x: c.X, y: c.Y, z: 8.5, kind: relicLamp, by: c.Y + 0.01, seed: 5})
	}
	for _, b := range own.Buildings {
		if !validKind(b.Kind) || b.Progress < 1 {
			continue
		}
		seed := sum.Seed*977 + uint64(b.ID)
		add := func(k lampKind, at float64, mats ...art.Material) {
			for i, w := range art.Windows(meshFor(b.Kind, seed), at, mats...) {
				out = append(out, lamp{x: b.X + w.X, y: b.Y + w.Y, z: w.Z, kind: k, by: b.Y,
					seed: uint32(seed)*31 + uint32(i)})
			}
		}
		switch b.Kind {
		case sim.House, sim.Barracks:
			add(windowLamp, 0.45, art.Plaster)
		case sim.Woodcutter:
			add(windowLamp, 0.4, art.Wood)
		case sim.Library:
			add(windowLamp, 0.5, art.Stone)
			out = append(out, lamp{x: b.X, y: b.Y, z: 6.5, kind: aetherLamp, by: b.Y + 0.01, seed: uint32(seed)})
		case sim.Tower:
			add(brazierLamp, 0.85, art.Stone)
		case sim.Extractor:
			out = append(out, lamp{x: b.X, y: b.Y, z: 4.5, kind: aetherLamp, by: b.Y + 0.01, seed: uint32(seed)})
		case sim.Core:
			out = append(out, lamp{x: b.X, y: b.Y, z: 13, kind: aetherLamp, by: b.Y + 0.01, seed: uint32(seed), big: true})
		}
	}
	return out
}

// lit reports whether a window shows light at a moment: deep in the night
// some go out, and others come on, every so often.
func (l lamp) lit(clock, dark float64) bool {
	if l.kind != windowLamp {
		return true
	}
	epoch := uint32(clock / 40)
	h := (l.seed ^ epoch*0x9e3779b9) * 0x85ebca6b
	h ^= h >> 13
	share := 0.9 - 0.3*dark // the later, the fewer
	return float64(h%1000)/1000 < share
}

// glow returns the colour of a lamp, its halo size and its strength, at a
// moment.
func (l lamp) glow(clock float64) (color.RGBA, int, float64) {
	ph := float64(l.seed%628) / 100
	switch l.kind {
	case brazierLamp:
		f := 0.75 + 0.15*math.Sin(clock*11+ph) + 0.1*math.Sin(clock*17+ph*2)
		return fireColour, 1, f
	case aetherLamp:
		f := 0.7 + 0.3*math.Sin(clock*1.6+ph)
		if l.big {
			return aetherColour, 2, f
		}
		return aetherColour, 1, f * 0.8
	case relicLamp:
		return goldColor, 1, 0.7 + 0.3*math.Sin(clock*1.2+ph)
	case lanternLamp:
		if l.festive && l.kind == lanternLamp {
			return festiveColours[l.seed%uint32(len(festiveColours))], 0, 0.9 + 0.1*math.Sin(clock*3+ph)
		}
		return lanternColour, 0, 0.8 + 0.2*math.Sin(clock*9+ph)
	}
	return windowColour, 1, 0.55
}

// haloImage draws a round halo in rings of decreasing strength, dithered
// where two rings meet, so that it reads as pixel art.
func haloImage(radius int) *ebiten.Image {
	size := 2*radius + 1
	img := ebiten.NewImage(size, size)
	px := make([]byte, size*size*4)
	bayer := [4][4]float64{{0, 8, 2, 10}, {12, 4, 14, 6}, {3, 11, 1, 9}, {15, 7, 13, 5}}
	for y := range size {
		for x := range size {
			dx, dy := float64(x-radius), float64(y-radius)/0.8
			d := math.Sqrt(dx*dx+dy*dy) / float64(radius)
			if d >= 1 {
				continue
			}
			// Three rings, the threshold matrix deciding the ring of a
			// pixel near a boundary.
			v := (1-d)*3 + (bayer[y%4][x%4]/16-0.5)*0.6
			var a float64
			switch {
			case v > 2:
				a = 0.42
			case v > 1:
				a = 0.22
			case v > 0.15:
				a = 0.09
			}
			b := byte(a * 255)
			i := (y*size + x) * 4
			px[i], px[i+1], px[i+2], px[i+3] = b, b, b, b
		}
	}
	img.WritePixels(px)
	return img
}

// drawLamp draws a lamp at a screen position in virtual pixels: its halo,
// added to the scene, and its bright core.
func (g *Game) drawLamp(l lamp, x, y, dark, clock float64) {
	if l.festive {
		dark = math.Max(dark, 0.6) // a feast shines by day too
	}
	if dark <= 0 || !l.lit(clock, dark) || g.glowAt == nil {
		return
	}
	col, size, f := l.glow(clock)
	f *= dark
	halo := g.halos[size]
	r := float64(haloRadii[size])
	g.glowAt(halo, x-r, y-r, col, f, true)
	core := 1.0
	if l.kind == windowLamp {
		g.glowAt(g.pixel, x, y-1, col, dark, false)
	} else if l.kind == lanternLamp {
		core = 0.9
	}
	g.glowAt(g.pixel, x, y, col, dark*core, false)
}

// lanterns are carried, at night, by the villagers walking about and by the
// guards on their rounds.
func lanterns(people []walker) []lamp {
	var out []lamp
	for i, w := range people {
		if w.state == resting && !w.guard {
			continue
		}
		if w.gathered() && w.act == hide {
			continue
		}
		dx := 0.8
		if i%2 == 1 {
			dx = -0.8
		}
		out = append(out, lamp{x: w.x + dx, y: w.y, z: 2.2, kind: lanternLamp, by: w.y, seed: uint32(i) * 7919})
	}
	return out
}

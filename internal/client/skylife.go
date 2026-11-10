package client

import (
	"image"
	"image/color"
	"math"
	"time"

	"apogee/internal/art"
	"apogee/internal/proto"
	"apogee/internal/sim"
)

// The sky between the islands lives too (GDD 12.6): at night every island
// shows its lights, more for a bigger people, and by day small airships of
// traders ply between neighbours. Both follow the sky's tick, so that every
// god sees the same traffic.

// tradeSpell is how often the traders set out; flight how long they take.
const (
	tradeSpell  = 3 * time.Minute
	tradeFlight = 70 * time.Second
)

// trade is an airship between two islands, at a point of its flight.
type trade struct {
	from, to proto.IslandID
	k        float64 // 0 at departure, 1 on arrival
}

// trades returns the airships in the sky at a tick: from some islands, to
// their nearest neighbour; never to or from an island that falls, nor
// between islands at war.
func trades(st *proto.State) []trade {
	if st == nil || len(st.Islands) < 2 {
		return nil
	}
	ticks := uint64(tradeSpell / sim.TickDuration)
	n, at := st.Tick/ticks, st.Tick%ticks
	k := float64(at) / float64(tradeFlight/sim.TickDuration)
	if k >= 1 {
		return nil
	}
	var out []trade
	for _, a := range st.Islands {
		if a.FallingMs > 0 || mixHash(n*7919+uint64(a.ID))%100 >= 35 {
			continue
		}
		best, bd := proto.IslandID(0), math.Inf(1)
		for _, b := range st.Islands {
			if b.ID == a.ID || b.FallingMs > 0 || (a.Banner != 0 && a.BannerTarget == b.ID) || (b.Banner != 0 && b.BannerTarget == a.ID) {
				continue
			}
			ax, ay := a.Radius*math.Cos(a.Angle), a.Radius*math.Sin(a.Angle)
			bx, by := b.Radius*math.Cos(b.Angle), b.Radius*math.Sin(b.Angle)
			if d := math.Hypot(ax-bx, ay-by); d < bd {
				best, bd = b.ID, d
			}
		}
		if best != 0 {
			out = append(out, trade{from: a.ID, to: best, k: k})
		}
	}
	return out
}

var (
	envelopeColour = color.RGBA{0xea, 0xda, 0xb4, 0xff}
	gondolaColour  = color.RGBA{0x7d, 0x56, 0x38, 0xff}
)

// drawSkyLife draws the lights of the islands at night and the traders'
// airships, in the sky view.
func (g *Game) drawSkyLife(islands []proto.IslandSummary, at func(p image.Point) (float64, float64)) {
	clock := float64(time.Now().UnixMilli()) / 1000 // the sky view keeps no animation clock
	if dark := g.dark(); dark > 0 {
		for _, is := range islands {
			if is.FallingMs > 0 {
				continue
			}
			ox, oy := at(g.orbitPixel(is))
			r := art.Rng(is.Seed + 0x11)
			n := min(3+int(is.Power/8), 8)
			size := math.Max(is.Size, 1) / miniScale * 0.5
			for i := range n {
				a := r.Float64() * 2 * math.Pi
				d := math.Sqrt(r.Float64()) * size
				x, y := g.cam.Project(art.V(d*math.Cos(a), d*math.Sin(a), 1.5))
				l := lamp{kind: windowLamp, seed: uint32(is.ID)*131 + uint32(i)}
				if i == 0 {
					l.kind = aetherLamp // the heart
				}
				col, _, f := l.glow(clock)
				f *= dark
				g.glowAt(g.halos[0], ox+math.Round(x)-float64(haloRadii[0]), oy+math.Round(y)-float64(haloRadii[0]), col, math.Min(f*1.5, 1), true)
				g.glowAt(g.pixel, ox+math.Round(x), oy+math.Round(y), col, dark, false)
			}
		}
	}
	where := map[proto.IslandID]image.Point{}
	for _, is := range islands {
		where[is.ID] = g.orbitPixel(is)
	}
	for _, t := range trades(g.src.State()) {
		a, okA := where[t.from]
		b, okB := where[t.to]
		if !okA || !okB {
			continue
		}
		e := ease(t.k)
		x := float64(a.X) + float64(b.X-a.X)*e
		// They rise from the island, cruise above, and come down.
		y := float64(a.Y) + float64(b.Y-a.Y)*e - 8 - 10*math.Sin(t.k*math.Pi) + math.Round(math.Sin(clock*2+float64(t.from)))
		px, py := at(image.Pt(int(math.Round(x)), int(math.Round(y))))
		for dx := -1.0; dx <= 1; dx++ {
			g.atTinted(g.pixel, px+dx, py, envelopeColour)
		}
		g.atTinted(g.pixel, px, py-1, envelopeColour)
		g.atTinted(g.pixel, px, py+2, gondolaColour)
	}
}

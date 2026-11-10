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

// The weather of the sky (GDD 12.5): spells of rain, wind that bends the
// pines and drives the smoke, morning fog over the island. It follows the
// sky's tick, so that every god of a sky sees the same weather.

// spell is how long the weather keeps, before it may change.
const spell = 12 * time.Minute

// weather is how strong each kind of weather is, from 0 to 1.
type weather struct {
	rain, wind, fog float64
}

// mixHash scrambles a number into a well spread one.
func mixHash(x uint64) uint64 {
	x ^= x >> 33
	x *= 0xff51afd7ed558ccd
	x ^= x >> 33
	x *= 0xc4ceb9fe1a85ec53
	x ^= x >> 33
	return x
}

// spellWeather is the weather a spell brings: rain in about one in four,
// and a wind that is stronger in the rain.
func spellWeather(n uint64) (rain, wind float64) {
	h := mixHash(n + 0x5ea)
	if h%100 < 24 {
		rain = 0.5 + float64(h>>8%50)/100
	}
	wind = 0.15 + float64(h>>16%60)/100 + rain*0.3
	return rain, math.Min(wind, 1)
}

// weatherAt returns the weather at a tick and time of day: each spell fades
// into the next over a minute, and fog lies over the island at dawn, more
// on some days than others.
func weatherAt(tick uint64, phase string, left time.Duration) weather {
	ticks := uint64(spell / sim.TickDuration)
	n := tick / ticks
	k := float64(tick%ticks) / float64(ticks)
	rain, wind := spellWeather(n)
	fade := float64(time.Minute) / float64(spell)
	if k > 1-fade {
		r2, w2 := spellWeather(n + 1)
		t := ease((k - (1 - fade)) / fade)
		rain, wind = rain+(r2-rain)*t, wind+(w2-wind)*t
	}
	var fog float64
	day := tick / uint64(30*time.Minute/sim.TickDuration)
	if mixHash(day+0xf06)%100 < 55 {
		switch phase {
		case "dawn":
			fog = 0.9
		case "day":
			// It lifts over the first minutes of the day.
			gone := phaseLength["day"] - left
			fog = 0.9 * (1 - ease(gone.Minutes()/4))
		}
	}
	return weather{rain: rain, wind: wind, fog: fog}
}

// forced returns a named weather, for screenshots.
func forcedWeather(name string) (weather, bool) {
	switch name {
	case "rain":
		return weather{rain: 1, wind: 0.8}, true
	case "wind":
		return weather{wind: 1}, true
	case "fog":
		return weather{fog: 1, wind: 0.1}, true
	case "clear":
		return weather{wind: 0.2}, true
	}
	return weather{}, false
}

var rainColours = []color.RGBA{{0xb8, 0xd4, 0xf0, 0xff}, {0x94, 0xb8, 0xe0, 0xff}}

// rain lets drops fall over the island in view, slanting with the wind.
func (p *particles) rainOver(size, rain, wind, dt float64) {
	if rain <= 0 || p.r == nil {
		return
	}
	r := p.r
	n := rain * 90 * dt
	count := int(n)
	if r.Float64() < n-float64(count) {
		count++
	}
	for range count {
		if len(p.ps) >= maxParticles {
			return
		}
		a := r.Float64() * 2 * math.Pi
		d := math.Sqrt(r.Float64()) * size * 1.1
		p.ps = append(p.ps, particle{x: d*math.Cos(a) - wind*8, y: d * math.Sin(a), z: 26 + r.Float64()*10,
			vx: wind * 9, vz: -28 - r.Float64()*6, life: 1.4, col: rainColours[r.IntN(len(rainColours))], streak: true})
	}
}

// sway returns how far a pine leans in the wind at a moment, as a shear:
// gusts run across the island from the west.
func (w weather) sway(clock, x float64) float64 {
	if w.wind <= 0 {
		return 0
	}
	gust := 0.5 + 0.5*math.Sin(clock*0.9-x*0.08)
	return w.wind * (0.04 + 0.1*gust)
}

// weatherNow returns the weather of the moment, or the forced one.
func (g *Game) weatherNow(st *proto.State) weather {
	if w, ok := forcedWeather(g.ForceWeather); ok {
		return w
	}
	phase, left, ok := g.moment()
	if !ok || st == nil {
		return weather{}
	}
	return weatherAt(st.Tick, phase, left)
}

// drawFog lays banks of mist low over the island in view, drifting with
// the wind.
func (g *Game) drawFog(atScaled func(*ebiten.Image, float64, float64, ebiten.ColorScale), size float64) {
	if g.wx.fog <= 0.02 || len(g.clouds) == 0 {
		return
	}
	drift := float64(g.animTick) / 60 * (0.6 + g.wx.wind*2)
	n := 12
	for i := range n {
		a := float64(i) * 2.39996
		d := size * (0.1 + 0.85*float64(i)/float64(n))
		x := d*math.Cos(a) + math.Mod(drift+float64(i)*13, size*2.4) - size*1.2
		y := d * math.Sin(a) * 0.8
		px, py := g.cam.Project(art.V(x, y, 3+float64(i%3)*2))
		c := g.clouds[i%len(g.clouds)].img
		b := c.Bounds()
		var cs ebiten.ColorScale
		cs.Scale(1.08, 1.08, 1.1, 1)
		cs.ScaleAlpha(float32(g.wx.fog * 0.4))
		atScaled(c, math.Round(px)-float64(b.Dx()/2), math.Round(py)-float64(b.Dy()/2), cs)
	}
}

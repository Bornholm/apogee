package sound

import (
	"math"
	"math/rand/v2"
)

// Ambience is the background of what the god looks at, each from 0 to 1:
// birds by day, crickets at night, rain, wind, running water and the
// hammers of the building sites. The engine draws chirps, trills and knocks
// from it at random, so that it never loops.
type Ambience struct {
	Birds, Crickets, Rain, Wind, Water, Work float64
}

// ambienceLevel keeps the ambience under the effects and the music.
const ambienceLevel = 0.4

// slice is the stretch of time each draw of the ambience covers.
const slice = 0.25

// ambient schedules the ambience slice by slice.
type ambient struct {
	next int64 // frame of the next slice
	r    *rand.Rand
}

// SetAmbience tells the engine what the god's view sounds like.
func (e *Engine) SetAmbience(a Ambience) {
	e.mu.Lock()
	e.ambience = a
	e.mu.Unlock()
}

// slice returns the notes of the next slice of ambience.
func (a *ambient) slice(m Ambience) []Note {
	r := a.r
	var out []Note
	pan := func() float64 { return r.Float64()*1.6 - 0.8 }
	at := func() float64 { return r.Float64() * slice }
	// Birds: two or three soft chirps, now and then.
	if r.Float64() < m.Birds*0.15 {
		t, p := at(), pan()
		base := 79 + float64(r.IntN(8))
		for i := range 2 + r.IntN(2) {
			out = append(out, Note{At: t + float64(i)*0.1, Dur: 0.07, Freq: midi(base + float64(r.IntN(3))),
				Wave: Triangle, Vol: 0.14, Slide: 2 - 4*r.Float64(), Attack: 0.01, Decay: 0.03, Pan: p})
		}
	}
	// Crickets: a short, quiet trill.
	if r.Float64() < m.Crickets*0.15 {
		t, p := at(), pan()
		f := 2200 + r.Float64()*400
		for i := range 3 + r.IntN(3) {
			out = append(out, Note{At: t + float64(i)*0.045, Dur: 0.022, Freq: f, Wave: Triangle, Vol: 0.07, Attack: 0.004, Pan: p})
		}
	}
	// Rain: a muffled patter, and drops now and then.
	if m.Rain > 0.02 {
		out = append(out, Note{Dur: slice + 0.06, Freq: 5000, Wave: Noise, Vol: 0.05 * m.Rain, Attack: 0.05})
		for range int(m.Rain * 2) {
			out = append(out, Note{At: at(), Dur: 0.03, Freq: 2500 + r.Float64()*1500, Wave: Noise, Vol: 0.06, Attack: 0.004, Decay: 0.01, Pan: pan()})
		}
	}
	// Running water: a low murmur.
	if m.Water > 0.02 {
		out = append(out, Note{Dur: slice + 0.06, Freq: 1400, Wave: Noise, Vol: 0.04 * m.Water, Attack: 0.05})
	}
	// Wind: a gust rises and falls, now and then.
	if r.Float64() < m.Wind*0.04 {
		out = append(out, Note{At: at(), Dur: 2.5, Freq: 3000, Wave: Noise, Vol: 0.05 * m.Wind,
			Attack: 1, Decay: 0.6, Sustain: 0, Pan: pan()})
	}
	// Hammers on the building sites: a knock, sometimes two.
	if r.Float64() < m.Work*0.2 {
		t, p := at(), pan()
		for i := range 1 + r.IntN(2) {
			out = append(out, Note{At: t + float64(i)*0.18, Dur: 0.04, Freq: 700 + r.Float64()*300, Wave: Noise, Vol: 0.08, Attack: 0.004, Decay: 0.012, Pan: p})
		}
	}
	return out
}

// quiet reports whether an ambience makes no sound at all.
func (m Ambience) quiet() bool {
	return math.Max(math.Max(m.Birds, m.Crickets), math.Max(math.Max(m.Rain, m.Wind), math.Max(m.Water, m.Work))) <= 0.02
}

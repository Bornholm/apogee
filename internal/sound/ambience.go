package sound

import (
	"math"
	"math/rand/v2"
)

// Ambience is the background of what the god looks at, each from 0 to 1:
// birds by day, crickets at night, rain, wind, a stream now and then, and the
// hammers of the building sites. The engine draws chirps, trills and knocks
// from it at random, so that it never loops.
type Ambience struct {
	Birds, Crickets, Rain, Wind, Water, Work float64
}

// ambienceLevel keeps the ambience under the effects and the music.
const ambienceLevel = 0.4

// slice is the stretch of time each draw of the ambience covers.
const slice = 0.25

// ambient schedules the ambience slice by slice: the chirps, trills, drops
// and knocks as notes, and the steady beds of rain and wind as
// filtered noise that runs on without ever starting over.
type ambient struct {
	next int64 // frame of the next slice
	r    *rand.Rand
	beds beds
}

// beds are the continuous noises of the ambience. Each is white noise
// through a low-pass filter, at a level that glides towards its target, so
// that the weather comes and goes without a step.
type beds struct {
	seed  uint64     // a xorshift generator: never the same stretch twice
	rain  [2]float64 // filter state, per channel
	wind  [2]float64
	level [2]float64 // rain and wind, as heard
	t     float64    // seconds, for the gusts
}

// Bed levels at full strength, and how fast the levels glide.
const (
	rainBed = 0.07
	windBed = 0.09
	glide   = 1.0 / (SampleRate * 2) // two seconds to reach a new level
)

// white returns the next sample of white noise, in [-1, 1].
func (b *beds) white() float64 {
	if b.seed == 0 {
		b.seed = 0x9e3779b97f4a7c15
	}
	b.seed ^= b.seed << 13
	b.seed ^= b.seed >> 7
	b.seed ^= b.seed << 17
	return float64(b.seed>>11)/float64(1<<53)*2 - 1
}

// sample returns the beds' next stereo frame for an ambience.
func (b *beds) sample(m Ambience) [2]float64 {
	target := [2]float64{m.Rain * rainBed, m.Wind * windBed}
	for i := range b.level {
		b.level[i] += (target[i] - b.level[i]) * glide * 8
	}
	// The gusts: two slow swells out of step, so that the wind rises and
	// falls without a pattern one can follow.
	b.t += 1.0 / SampleRate
	gust := 0.25 + 0.75*math.Max(0, math.Sin(b.t*0.41)*0.6+math.Sin(b.t*0.173+1)*0.4)
	var out [2]float64
	for c := range 2 {
		n := b.white()
		b.rain[c] += (n - b.rain[c]) * 0.35 // a soft patter, about 3 kHz
		b.wind[c] += (n - b.wind[c]) * 0.02 // a deep rush
		out[c] = b.rain[c]*b.level[0] + b.wind[c]*b.level[1]*3*gust
	}
	return out
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
	// Rain: drops now and then over its patter (a bed).
	if m.Rain > 0.02 {
		for range int(m.Rain * 2) {
			out = append(out, Note{At: at(), Dur: 0.03, Freq: 2500 + r.Float64()*1500, Wave: Noise, Vol: 0.06, Attack: 0.004, Decay: 0.01, Pan: pan()})
		}
	}
	// A stream: now and then a soft gurgle, a few low notes that bend,
	// never a steady murmur.
	if r.Float64() < m.Water*0.025 {
		t, p := at(), pan()
		base := 50 + float64(r.IntN(6))
		for i := range 2 + r.IntN(3) {
			out = append(out, Note{At: t + float64(i)*0.07, Dur: 0.06, Freq: midi(base + float64(r.IntN(5))),
				Wave: Triangle, Vol: 0.1, Slide: 3 + 3*r.Float64(), Attack: 0.01, Decay: 0.03, Pan: p})
		}
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

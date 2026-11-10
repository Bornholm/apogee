package sound

import "math/rand/v2"

// Mood is what the sky is like, which the music follows: the time of day,
// and the decline of the island.
type Mood struct {
	Phase   string // "dawn", "day", "dusk", "night"; anything else is calm
	Decline bool
}

// style is how a mood sounds.
type style struct {
	bpm   float64
	root  float64 // MIDI note of the key
	scale []int   // a pentatonic, in semitones from the root
	// prog is the chord of each bar of a phrase, as a root in semitones
	// from the key and whether the chord is minor.
	prog    [4]chord
	density float64 // chance that a melody slot starts a note
	arp     float64 // chance that an arpeggio slot plays
	perc    bool
	lead    Wave
	leadVol float64
}

type chord struct {
	root  int
	minor bool
}

var (
	majorPenta = []int{0, 2, 4, 7, 9}
	minorPenta = []int{0, 3, 5, 7, 10}
)

func styleOf(m Mood) style {
	var s style
	switch m.Phase {
	case "day":
		s = style{bpm: 84, root: 60, scale: majorPenta,
			prog:    [4]chord{{0, false}, {9, true}, {5, false}, {7, false}},
			density: 0.55, arp: 0.8, perc: true, lead: Pulse50, leadVol: 0.32}
	case "dusk":
		s = style{bpm: 70, root: 62, scale: minorPenta,
			prog:    [4]chord{{0, true}, {10, false}, {8, false}, {10, false}},
			density: 0.4, arp: 0.6, lead: Pulse25, leadVol: 0.3}
	case "night":
		s = style{bpm: 58, root: 57, scale: minorPenta,
			prog:    [4]chord{{0, true}, {8, false}, {3, false}, {10, false}},
			density: 0.28, arp: 0.35, lead: Pulse12, leadVol: 0.3}
	default: // dawn, and the home screen
		s = style{bpm: 70, root: 65, scale: majorPenta,
			prog:    [4]chord{{0, false}, {7, false}, {9, true}, {5, false}},
			density: 0.4, arp: 0.6, lead: Triangle, leadVol: 0.5}
	}
	if m.Decline {
		// A declining people sings lower and slower, in minor.
		s.root -= 3
		s.scale = minorPenta
		s.bpm *= 0.9
		s.perc = false
		for i := range s.prog {
			s.prog[i].minor = i%2 == 0
		}
	}
	return s
}

// The music comes in pieces of a few minutes, separated by silences: a game
// that runs all day on a second screen must not tire its player.
const (
	phrasesPerPiece = 8
	barsPerPhrase   = 4
	slotsPerBar     = 8 // eighth notes
	minSilence      = 60.0
	maxSilence      = 150.0
)

// scheduled is a note to start at a base frame.
type scheduled struct {
	Note
	base int64
}

// composer writes the music one bar at a time.
type composer struct {
	rng  *rand.Rand
	next int64 // frame where the next bar starts
	bar0 int   // bars written in the current piece
	sty  style
	// motif is the melody of two bars, as scale steps (or -1 for a rest)
	// and lengths in slots; each phrase plays it, then a variation.
	motif []step
	vary  []step
}

type step struct {
	deg  int // index in the scale over two octaves, or -1 for a rest
	dur  int // slots
	slot int
}

func newComposer(seed uint64) composer {
	return composer{rng: rand.New(rand.NewPCG(seed, 0xa90)), next: SampleRate * 2}
}

// idle delays the next piece while the music is off.
func (c *composer) idle(now int64) {
	if c.next < now {
		c.next, c.bar0 = now+SampleRate, 0
	}
}

// bar returns the notes of the next bar and advances.
func (c *composer) bar(m Mood) []scheduled {
	piece := phrasesPerPiece * barsPerPhrase
	if c.bar0 == piece {
		// The piece is over: a silence, then a new one.
		silence := minSilence + c.rng.Float64()*(maxSilence-minSilence)
		c.next += int64(silence * SampleRate)
		c.bar0 = 0
		return nil
	}
	inPhrase := c.bar0 % barsPerPhrase
	if inPhrase == 0 {
		// Moods change between phrases, and the melody every two phrases.
		c.sty = styleOf(m)
		if c.bar0%(2*barsPerPhrase) == 0 {
			c.motif = c.melody()
			c.vary = c.variation(c.motif)
		}
	}
	s := c.sty
	beat := 60 / s.bpm
	slot := beat / 2
	ch := s.prog[inPhrase]
	var out []scheduled
	add := func(n Note) { out = append(out, scheduled{n, c.next}) }

	// Bass, on the triangle.
	bass := s.root - 24 + float64(ch.root)
	if s.bpm >= 80 {
		add(pluck(0, 1.8*beat, bass, Triangle, 0.55))
		add(pluck(2*beat, 1.8*beat, bass+7, Triangle, 0.5))
	} else {
		add(held(0, 3.8*beat, bass, Triangle, 0.5))
	}
	// Arpeggio of the chord, thin and quiet.
	third := 4
	if ch.minor {
		third = 3
	}
	tones := []int{0, third, 7, 12, 7, third, 0, 7}
	for i := range slotsPerBar {
		if c.rng.Float64() < s.arp {
			m := s.root - 12 + float64(ch.root+tones[i])
			add(Note{At: float64(i) * slot, Dur: slot * 0.9, Freq: midi(m), Wave: Pulse12, Vol: 0.16,
				Decay: slot / 3, Pan: -0.3})
		}
	}
	// Melody: the first bar of the motif on even bars, the second on odd
	// ones; the second half of a phrase plays the variation.
	mel := c.motif
	if inPhrase >= 2 {
		mel = c.vary
	}
	half := inPhrase % 2
	for _, st := range mel {
		if st.deg < 0 || st.slot/slotsPerBar != half {
			continue
		}
		at := float64(st.slot%slotsPerBar) * slot
		add(Note{At: at, Dur: float64(st.dur)*slot - 0.01, Freq: midi(s.root + float64(c.degree(st.deg))),
			Wave: s.lead, Vol: s.leadVol, Decay: float64(st.dur) * slot / 2, Sustain: 0.5, Pan: 0.25})
	}
	// Percussion, by day only: a soft hat on the off-beats.
	if s.perc {
		for i := 1; i < slotsPerBar; i += 2 {
			add(Note{At: float64(i) * slot, Dur: 0.03, Freq: 11000, Wave: Noise, Vol: 0.12, Decay: 0.01})
		}
	}
	c.next += int64(4 * beat * SampleRate)
	c.bar0++
	return out
}

// degree turns a step over two octaves of the scale into semitones.
func (c *composer) degree(d int) int {
	n := len(c.sty.scale)
	return 12*(d/n) + c.sty.scale[d%n]
}

// melody writes a motif of two bars: a walk over the scale, with rests,
// that ends on the root.
func (c *composer) melody() []step {
	var out []step
	deg := len(c.sty.scale) // the root, an octave up
	for slot := 0; slot < 2*slotsPerBar; {
		dur := 1 + c.rng.IntN(3)
		if slot%2 == 0 && c.rng.Float64() < 0.5 {
			dur++ // notes on the beat last longer
		}
		dur = min(dur, 2*slotsPerBar-slot)
		if c.rng.Float64() >= c.sty.density && slot > 0 {
			out = append(out, step{deg: -1, dur: dur, slot: slot})
		} else {
			deg = clampDeg(deg+c.rng.IntN(5)-2, 2*len(c.sty.scale)-1)
			out = append(out, step{deg: deg, dur: dur, slot: slot})
		}
		slot += dur
	}
	return out
}

// variation keeps the first bar of a motif and rewrites the end, to land
// on the root.
func (c *composer) variation(m []step) []step {
	out := append([]step(nil), m...)
	for i := range out {
		if out[i].slot >= slotsPerBar+slotsPerBar/2 && out[i].deg >= 0 {
			out[i].deg = clampDeg(out[i].deg+c.rng.IntN(3)-1, 2*len(c.sty.scale)-1)
		}
	}
	for i := len(out) - 1; i >= 0; i-- {
		if out[i].deg >= 0 {
			out[i].deg = len(c.sty.scale) // resolve on the root
			break
		}
	}
	return out
}

func clampDeg(d, hi int) int { return min(max(d, 0), hi) }

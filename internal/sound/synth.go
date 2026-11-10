// Package sound is Apogée's 8-bit sound (GDD 12.7): a small synthesiser in
// the manner of the old consoles, with two pulse channels, a triangle and a
// noise generator, that plays the effects of the game and a generative
// music following the time of day. Nothing is recorded: every sound is
// computed, so the package holds no audio file.
package sound

import "math"

// SampleRate is the rate of the stream, in frames per second.
const SampleRate = 44100

// Wave is the shape of a voice.
type Wave uint8

const (
	Pulse12  Wave = iota // thin pulse, 12.5 % duty
	Pulse25              // hollow pulse, 25 % duty
	Pulse50              // square
	Triangle             // soft, stepped like the old consoles
	Noise                // a 15-bit shift register; Freq is its clock
)

// Note is one sound of one voice.
type Note struct {
	At   float64 // seconds after the moment it is scheduled
	Dur  float64 // seconds
	Freq float64 // Hz
	Wave Wave
	Vol  float64 // 0 to 1
	// Slide bends the pitch by this many semitones over the note.
	Slide float64
	// Attack is the rise time in seconds; a few milliseconds if zero.
	Attack float64
	// Decay is the time constant in seconds of the fall towards Sustain
	// (a fraction of Vol); zero holds the note at full volume.
	Decay, Sustain float64
	Pan            float64 // -1 left to 1 right
}

// Bus sorts voices between music and effects, which have their own volume.
type bus uint8

const (
	musicBus bus = iota
	effectsBus
	ambienceBus
)

// voice is a note being played.
type voice struct {
	n     Note
	bus   bus
	start int64 // first sample
	end   int64 // sample after the last
	phase float64
	lfsr  uint16
	noise float64
}

const minAttack = 0.004
const release = 0.03

// envelope returns the volume of the voice at t seconds into the note,
// quantised to sixteen levels like the volume registers of the old consoles.
func (v *voice) envelope(t float64) float64 {
	n := &v.n
	a := math.Max(n.Attack, minAttack)
	e := 1.0
	if t < a {
		e = t / a
	}
	if n.Decay > 0 {
		e *= n.Sustain + (1-n.Sustain)*math.Exp(-math.Max(t-a, 0)/n.Decay)
	}
	if left := n.Dur - t; left < release {
		e *= math.Max(left, 0) / release
	}
	return math.Round(e*n.Vol*15) / 15
}

// sample returns the next sample of the voice, t seconds into the note.
func (v *voice) sample(t float64) float64 {
	n := &v.n
	f := n.Freq
	if n.Slide != 0 && n.Dur > 0 {
		f *= math.Exp2(n.Slide * t / n.Dur / 12)
	}
	v.phase += f / SampleRate
	wrapped := v.phase >= 1
	v.phase -= math.Floor(v.phase)
	var s float64
	switch n.Wave {
	case Pulse12, Pulse25, Pulse50:
		duty := [...]float64{0.125, 0.25, 0.5}[n.Wave]
		if v.phase < duty {
			s = 1
		} else {
			s = -1
		}
		s *= 0.5 // squares are loud: bring them level with the triangle
	case Triangle:
		s = 1 - 4*math.Abs(v.phase-0.5)
		s = (min(math.Floor((s+1)*8), 15) - 7.5) / 7.5 // sixteen steps

	case Noise:
		if wrapped || v.lfsr == 0 {
			if v.lfsr == 0 {
				v.lfsr = 1
			}
			bit := (v.lfsr ^ v.lfsr>>1) & 1
			v.lfsr = v.lfsr>>1 | bit<<14
			v.noise = float64(v.lfsr&1)*2 - 1
		}
		s = v.noise * 0.6
	}
	return s * v.envelope(t)
}

// midi returns the frequency of a MIDI note number.
func midi(n float64) float64 { return 440 * math.Exp2((n-69)/12) }

package sound

// Effect is a short sound tied to an event of the game.
type Effect int

const (
	Click      Effect = iota // a button of the interface
	Whisper                  // the god whispers a pie
	Answer                   // the god answers a supplique
	Built                    // a building is finished, or the island grew
	Discovery                // the scholars mastered a discovery
	Supplique                // the people ask their god
	Raid                     // warriors of another island threaten us
	Struck                   // a wave or a raid struck the heart
	Victory                  // an attack was repelled
	Decline                  // the decline began
	Fall                     // the island falls into the whirlpool
	Birth                    // a new island rises from the clouds
	Feast                    // the people feast a milestone
	NumEffects               // the number of effects
)

// pluck is a note that fades like a plucked string.
func pluck(at, dur, m float64, w Wave, vol float64) Note {
	return Note{At: at, Dur: dur, Freq: midi(m), Wave: w, Vol: vol, Decay: dur / 3}
}

// held is a note that keeps most of its volume.
func held(at, dur, m float64, w Wave, vol float64) Note {
	return Note{At: at, Dur: dur, Freq: midi(m), Wave: w, Vol: vol, Decay: dur / 2, Sustain: 0.6}
}

// arpeggio plays the notes one after the other, step seconds apart.
func arpeggio(at, step, dur float64, w Wave, vol float64, notes ...float64) []Note {
	out := make([]Note, len(notes))
	for i, m := range notes {
		out[i] = pluck(at+float64(i)*step, dur, m, w, vol)
	}
	return out
}

func join(parts ...[]Note) []Note {
	var out []Note
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// Note numbers: 60 is middle C. The effects are soft (GDD 12.7): the
// triangle carries them, thin pulses only colour them, nothing goes
// shrill, and they stay short. The game runs all day beside other work.
var effects = [NumEffects][]Note{
	Click: {{Dur: 0.03, Freq: 660, Wave: Triangle, Vol: 0.12, Decay: 0.012}},
	Whisper: join(
		// A breath, then a rising chord.
		[]Note{{Dur: 0.5, Freq: 3000, Wave: Noise, Vol: 0.06, Attack: 0.2, Decay: 0.15}},
		arpeggio(0.12, 0.08, 0.35, Triangle, 0.4, 60, 64, 67, 72),
	),
	Answer: {
		pluck(0, 0.3, 67, Triangle, 0.4),
		pluck(0.12, 0.45, 74, Triangle, 0.35),
		pluck(0.12, 0.45, 74, Pulse25, 0.08),
	},
	Built: join(
		// Two soft knocks, then a satisfied pair of notes.
		[]Note{
			{Dur: 0.04, Freq: 900, Wave: Noise, Vol: 0.12, Decay: 0.012},
			{At: 0.16, Dur: 0.04, Freq: 900, Wave: Noise, Vol: 0.12, Decay: 0.012},
		},
		arpeggio(0.34, 0.12, 0.35, Triangle, 0.4, 60, 67),
		[]Note{pluck(0.34, 0.5, 43, Triangle, 0.35)},
	),
	Discovery: join(
		arpeggio(0, 0.08, 0.3, Triangle, 0.35, 72, 74, 76, 79, 81, 84),
		arpeggio(0.2, 0.08, 0.3, Pulse25, 0.06, 72, 74, 76, 79, 81, 84), // echo
		[]Note{held(0, 0.8, 48, Triangle, 0.3)},
	),
	Supplique: {
		// A call that waits for an answer: the last note rises.
		pluck(0, 0.3, 64, Triangle, 0.4),
		pluck(0.2, 0.3, 67, Triangle, 0.4),
		{At: 0.4, Dur: 0.6, Freq: midi(69), Wave: Triangle, Vol: 0.4, Slide: 2, Decay: 0.4, Sustain: 0.5},
	},
	Raid: {
		// A distant horn, twice, over a drum.
		{Dur: 0.45, Freq: midi(50), Wave: Pulse25, Vol: 0.14, Attack: 0.06, Decay: 0.3, Sustain: 0.7},
		{At: 0.5, Dur: 0.75, Freq: midi(45), Wave: Pulse25, Vol: 0.14, Attack: 0.06, Slide: -1, Decay: 0.5, Sustain: 0.6},
		{Dur: 0.45, Freq: midi(50), Wave: Triangle, Vol: 0.35, Attack: 0.06, Decay: 0.3, Sustain: 0.7},
		{At: 0.5, Dur: 0.75, Freq: midi(45), Wave: Triangle, Vol: 0.35, Attack: 0.06, Decay: 0.5, Sustain: 0.6},
		{Dur: 0.2, Freq: 90, Wave: Triangle, Vol: 0.45, Slide: -12, Decay: 0.08},
		{At: 0.5, Dur: 0.2, Freq: 90, Wave: Triangle, Vol: 0.45, Slide: -12, Decay: 0.08},
	},
	Struck: {
		// A low rumble and a falling blow.
		{Dur: 1.0, Freq: 300, Wave: Noise, Vol: 0.16, Attack: 0.03, Decay: 0.35},
		{Dur: 0.9, Freq: midi(38), Wave: Triangle, Vol: 0.55, Slide: -7, Decay: 0.4},
	},
	Victory: join(
		arpeggio(0, 0.12, 0.2, Triangle, 0.4, 60, 64, 67),
		[]Note{
			held(0.36, 0.7, 72, Triangle, 0.35),
			held(0.36, 0.7, 67, Pulse25, 0.06),
			pluck(0, 0.3, 36, Triangle, 0.4),
			pluck(0.36, 0.6, 36, Triangle, 0.4),
		},
	),
	Decline: arpeggio(0, 0.4, 0.7, Triangle, 0.4, 57, 53, 50, 45),
	Fall: {
		// The island slides down into the whirlpool.
		{Dur: 2.6, Freq: midi(60), Wave: Triangle, Vol: 0.4, Slide: -24, Decay: 1.5, Sustain: 0.3},
		{Dur: 2.8, Freq: 1500, Wave: Noise, Vol: 0.1, Attack: 0.8, Slide: -24, Decay: 1.2},
		{At: 2.2, Dur: 1.6, Freq: midi(33), Wave: Triangle, Vol: 0.55, Decay: 0.8},
	},
	Birth: join(
		// Dawn over a new island: a slow chord that opens.
		[]Note{
			held(0, 1.6, 55, Triangle, 0.35),
			held(0.3, 1.3, 59, Triangle, 0.25),
			held(0.6, 1.0, 62, Triangle, 0.25),
		},
		arpeggio(0.9, 0.14, 0.6, Triangle, 0.25, 67, 71, 74),
	),
	Feast: join(
		// A dancing tune: a skipping melody over a bouncing bass.
		arpeggio(0, 0.13, 0.15, Triangle, 0.35, 60, 64, 67, 64, 69, 67, 64, 72),
		arpeggio(0, 0.26, 0.2, Triangle, 0.35, 36, 43, 36, 43),
	),
}

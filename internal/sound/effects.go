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

// Note numbers: 60 is middle C.
var effects = [NumEffects][]Note{
	Click: {{Dur: 0.025, Freq: 1320, Wave: Pulse12, Vol: 0.25, Decay: 0.01}},
	Whisper: join(
		// A breath, then a rising chord.
		[]Note{{Dur: 0.5, Freq: 9000, Wave: Noise, Vol: 0.12, Attack: 0.2, Decay: 0.15}},
		arpeggio(0.12, 0.07, 0.35, Triangle, 0.5, 72, 76, 79, 84),
		arpeggio(0.15, 0.07, 0.3, Pulse12, 0.15, 72, 76, 79, 84),
	),
	Answer: {
		pluck(0, 0.25, 79, Pulse25, 0.35),
		pluck(0.1, 0.45, 86, Pulse25, 0.35),
		pluck(0.1, 0.45, 74, Triangle, 0.4),
	},
	Built: join(
		// Two hammer blows, then a satisfied pair of notes.
		[]Note{
			{Dur: 0.05, Freq: 2400, Wave: Noise, Vol: 0.35, Decay: 0.015},
			{At: 0.14, Dur: 0.05, Freq: 2400, Wave: Noise, Vol: 0.35, Decay: 0.015},
		},
		arpeggio(0.32, 0.1, 0.3, Pulse50, 0.3, 67, 72),
		[]Note{pluck(0.32, 0.5, 48, Triangle, 0.5)},
	),
	Discovery: join(
		arpeggio(0, 0.06, 0.25, Pulse12, 0.3, 84, 86, 88, 91, 93, 96),
		arpeggio(0.18, 0.06, 0.25, Pulse12, 0.12, 84, 86, 88, 91, 93, 96), // echo
		[]Note{held(0, 0.8, 60, Triangle, 0.35)},
	),
	Supplique: {
		// A call that waits for an answer: the last note rises.
		pluck(0, 0.3, 76, Pulse25, 0.35),
		pluck(0.18, 0.3, 79, Pulse25, 0.35),
		{At: 0.36, Dur: 0.6, Freq: midi(81), Wave: Triangle, Vol: 0.5, Slide: 2, Decay: 0.4, Sustain: 0.5},
	},
	Raid: {
		// A war horn, twice, over a drum.
		{Dur: 0.4, Freq: midi(62), Wave: Pulse25, Vol: 0.35, Attack: 0.04, Decay: 0.3, Sustain: 0.7},
		{At: 0.45, Dur: 0.7, Freq: midi(57), Wave: Pulse25, Vol: 0.35, Attack: 0.04, Slide: -1, Decay: 0.5, Sustain: 0.6},
		{Dur: 0.2, Freq: 110, Wave: Triangle, Vol: 0.6, Slide: -12, Decay: 0.08},
		{At: 0.45, Dur: 0.2, Freq: 110, Wave: Triangle, Vol: 0.6, Slide: -12, Decay: 0.08},
	},
	Struck: {
		// A low rumble and a falling blow.
		{Dur: 1.2, Freq: 700, Wave: Noise, Vol: 0.4, Decay: 0.4},
		{Dur: 0.9, Freq: midi(43), Wave: Triangle, Vol: 0.7, Slide: -7, Decay: 0.4},
	},
	Victory: join(
		arpeggio(0, 0.1, 0.18, Pulse50, 0.3, 72, 76, 79),
		[]Note{
			held(0.3, 0.6, 84, Pulse50, 0.3),
			held(0.3, 0.6, 79, Pulse25, 0.18),
			pluck(0, 0.3, 48, Triangle, 0.5),
			pluck(0.3, 0.6, 48, Triangle, 0.5),
		},
	),
	Decline: join(
		arpeggio(0, 0.35, 0.6, Triangle, 0.45, 69, 65, 62, 57),
		arpeggio(0, 0.35, 0.5, Pulse12, 0.12, 69, 65, 62, 57),
	),
	Fall: {
		// The island slides down into the whirlpool.
		{Dur: 2.6, Freq: midi(72), Wave: Pulse25, Vol: 0.3, Slide: -30, Decay: 1.5, Sustain: 0.3},
		{Dur: 2.8, Freq: 5000, Wave: Noise, Vol: 0.25, Attack: 0.6, Slide: -24, Decay: 1.2},
		{At: 2.2, Dur: 1.6, Freq: midi(33), Wave: Triangle, Vol: 0.7, Decay: 0.8},
	},
	Birth: join(
		// Dawn over a new island: a slow chord that opens.
		[]Note{
			held(0, 1.6, 67, Triangle, 0.4),
			held(0.25, 1.35, 71, Pulse12, 0.18),
			held(0.5, 1.1, 74, Pulse12, 0.18),
		},
		arpeggio(0.75, 0.12, 0.6, Pulse25, 0.22, 79, 83, 86),
	),
}

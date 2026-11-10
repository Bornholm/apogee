package sound

import (
	"encoding/binary"
	"math"
	"math/rand/v2"
	"sync"
)

// maxEffectNotes bounds the notes of effects waiting or playing: a burst
// of effects never costs more than this, and the oldest notes give way.
// Effects have up to fifteen notes, so three or four can overlap. The music
// needs no bound: it is written half a second ahead, no more.
const maxEffectNotes = 48

// Engine mixes the effects and the music into a stream of 32-bit float
// stereo frames, the format Ebitengine's NewPlayerF32 reads. It is safe for
// concurrent use: the game triggers effects while the audio thread reads.
type Engine struct {
	mu     sync.Mutex
	clock  int64 // frames rendered so far
	voices []voice

	volume  float64 // master, 0 to 1
	music   bool
	muted   bool
	mood    Mood
	compose composer
	// ambience is the background of the god's view (ambience.go).
	ambience  Ambience
	ambient   ambient
	noiseSeed uint32

	// One-pole filters on the mix: a high-pass removes the offset of thin
	// pulses, a low-pass takes the edge off the squares.
	hp, hpIn, lp [2]float64
}

// New returns an engine whose music is drawn from seed.
func New(seed uint64) *Engine {
	return &Engine{volume: 0.35, music: true, compose: newComposer(seed),
		ambient: ambient{r: rand.New(rand.NewPCG(seed, 0xa3b1))}}
}

// SetVolume sets the master volume, from 0 to 1.
func (e *Engine) SetVolume(v float64) {
	e.mu.Lock()
	e.volume = math.Min(math.Max(v, 0), 1)
	e.mu.Unlock()
}

// SetMuted silences everything, or brings the sound back.
func (e *Engine) SetMuted(m bool) {
	e.mu.Lock()
	e.muted = m
	e.mu.Unlock()
}

// Muted reports whether the sound is silenced.
func (e *Engine) Muted() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.muted
}

// SetMusic turns the music on or off; the effects keep playing.
func (e *Engine) SetMusic(on bool) {
	e.mu.Lock()
	e.music = on
	if !on {
		e.dropBus(musicBus)
	}
	e.mu.Unlock()
}

// SetMood tells the music what the sky is like. It changes at the next
// phrase, never in the middle of one.
func (e *Engine) SetMood(m Mood) {
	e.mu.Lock()
	e.mood = m
	e.mu.Unlock()
}

// Play starts an effect now.
func (e *Engine) Play(fx Effect) {
	notes := effects[fx]
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.muted {
		return
	}
	for _, n := range notes {
		// A few milliseconds of attack: no effect starts with a click.
		n.Attack = math.Max(n.Attack, 0.008)
		e.add(n, effectsBus, e.clock)
	}
}

// add schedules a note from a base frame.
func (e *Engine) add(n Note, b bus, base int64) {
	if n.Dur <= 0 || n.Vol <= 0 {
		return
	}
	if b == effectsBus && e.count(effectsBus) >= maxEffectNotes {
		for i, v := range e.voices {
			if v.bus == effectsBus {
				e.voices = append(e.voices[:i], e.voices[i+1:]...)
				break
			}
		}
	}
	start := base + int64(n.At*SampleRate)
	v := voice{n: n, bus: b, start: start, end: start + int64(n.Dur*SampleRate)}
	if n.Wave == Noise {
		// Each noise starts somewhere else in the sequence: two knocks
		// never sound exactly alike.
		e.noiseSeed = e.noiseSeed*1103515245 + 12345
		v.lfsr = uint16(e.noiseSeed>>16)&0x7fff | 1
	}
	e.voices = append(e.voices, v)
}

func (e *Engine) count(b bus) int {
	n := 0
	for _, v := range e.voices {
		if v.bus == b {
			n++
		}
	}
	return n
}

func (e *Engine) dropBus(b bus) {
	kept := e.voices[:0]
	for _, v := range e.voices {
		if v.bus != b {
			kept = append(kept, v)
		}
	}
	e.voices = kept
}

// Bus levels, under the master volume: the music stays in the background.
const (
	musicLevel   = 0.45
	effectsLevel = 0.55
)

// Read fills p with float32 stereo frames. It never fails and never ends.
func (e *Engine) Read(p []byte) (int, error) {
	frames := len(p) / 8
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.music && !e.muted {
		// Compose half a second ahead of what is played.
		for e.compose.next < e.clock+int64(frames)+SampleRate/2 {
			for _, sn := range e.compose.bar(e.mood) {
				e.add(sn.Note, musicBus, sn.base)
			}
		}
	} else {
		// Without music, the composer waits: it starts a new piece when
		// the music comes back.
		e.compose.idle(e.clock + int64(frames))
	}
	// The ambience, a quarter of a second ahead.
	for e.ambient.next < e.clock+int64(frames)+SampleRate/4 {
		if !e.muted && !e.ambience.quiet() {
			for _, n := range e.ambient.slice(e.ambience) {
				e.add(n, ambienceBus, e.ambient.next)
			}
		}
		e.ambient.next += int64(slice * SampleRate)
	}
	gain := e.volume
	if e.muted {
		gain = 0
	}
	const hpK = 0.997 // about 20 Hz
	const lpK = 0.4   // about 3.5 kHz: nothing shrill
	for i := range frames {
		now := e.clock + int64(i)
		var mix [2]float64
		for j := range e.voices {
			v := &e.voices[j]
			if now < v.start || now >= v.end {
				continue
			}
			s := v.sample(float64(now-v.start) / SampleRate)
			switch v.bus {
			case musicBus:
				s *= musicLevel
			case ambienceBus:
				s *= ambienceLevel
			default:
				s *= effectsLevel
			}
			mix[0] += s * math.Min(1, 1-v.n.Pan)
			mix[1] += s * math.Min(1, 1+v.n.Pan)
		}
		if !e.muted {
			bed := e.ambient.beds.sample(e.ambience)
			mix[0] += bed[0] * ambienceLevel
			mix[1] += bed[1] * ambienceLevel
		}
		for c := range 2 {
			x := mix[c] * gain
			e.hp[c] = hpK * (e.hp[c] + x - e.hpIn[c])
			e.hpIn[c] = x
			e.lp[c] += lpK * (e.hp[c] - e.lp[c])
			if math.Abs(e.lp[c]) < 1e-9 && math.Abs(e.hp[c]) < 1e-9 {
				e.hp[c], e.lp[c] = 0, 0 // no endless tail of denormals
			}
			out := math.Max(-1, math.Min(1, e.lp[c]))
			binary.LittleEndian.PutUint32(p[i*8+c*4:], math.Float32bits(float32(out)))
		}
	}
	e.clock += int64(frames)
	kept := e.voices[:0]
	for _, v := range e.voices {
		if v.end > e.clock {
			kept = append(kept, v)
		}
	}
	e.voices = kept
	return frames * 8, nil
}

package sound

import (
	"encoding/binary"
	"math"
	"testing"
)

// render reads seconds of stream and returns the peak of each half second.
func render(e *Engine, seconds float64) []float64 {
	buf := make([]byte, SampleRate/2*8)
	var peaks []float64
	for range int(seconds * 2) {
		n, err := e.Read(buf)
		if err != nil || n != len(buf) {
			panic("short read")
		}
		peak := 0.0
		for i := 0; i < n; i += 4 {
			v := float64(math.Float32frombits(binary.LittleEndian.Uint32(buf[i:])))
			if math.IsNaN(v) || v < -1 || v > 1 {
				panic("sample out of range")
			}
			peak = math.Max(peak, math.Abs(v))
		}
		peaks = append(peaks, peak)
	}
	return peaks
}

func TestEveryEffectSounds(t *testing.T) {
	for fx := range NumEffects {
		e := New(1)
		e.SetMusic(false)
		e.Play(fx)
		if p := render(e, 0.5)[0]; p < 0.01 {
			t.Errorf("effect %d is silent (peak %.4f)", fx, p)
		}
		// Every effect ends, and leaves no voice behind.
		render(e, 5)
		if len(e.voices) != 0 {
			t.Errorf("effect %d still holds %d voices after 5 s", fx, len(e.voices))
		}
	}
}

func TestMutedIsSilent(t *testing.T) {
	e := New(1)
	e.SetMuted(true)
	e.Play(Victory)
	for i, p := range render(e, 4) {
		if p != 0 {
			t.Fatalf("muted engine sounds at %.1f s (peak %.4f)", float64(i)/2, p)
		}
	}
}

func TestMusicPlaysThenRests(t *testing.T) {
	for _, phase := range []string{"dawn", "day", "dusk", "night", ""} {
		for _, decline := range []bool{false, true} {
			e := New(3)
			e.SetMood(Mood{Phase: phase, Decline: decline})
			peaks := render(e, 400)
			sounding, silent := 0, 0
			for _, p := range peaks[6:] { // the first piece starts after 2 s
				if p > 0.005 {
					sounding++
				} else {
					silent++
				}
			}
			// A piece lasts some minutes, then the music rests a while.
			if sounding < 2*90 || silent < 2*50 {
				t.Errorf("%q decline=%v: %d s of music and %d s of silence in 400 s",
					phase, decline, sounding/2, silent/2)
			}
		}
	}
}

func TestMusicIsDeterministic(t *testing.T) {
	a, b := New(9), New(9)
	pa, pb := render(a, 30), render(b, 30)
	for i := range pa {
		if pa[i] != pb[i] {
			t.Fatalf("same seed, different music at %.1f s", float64(i)/2)
		}
	}
}

func TestMusicOffStopsAtOnce(t *testing.T) {
	e := New(1)
	render(e, 10)
	e.SetMusic(false)
	for i, p := range render(e, 2) {
		// The filters let a few samples through, no more.
		if i > 0 && p > 1e-4 {
			t.Fatalf("music still sounds %.1f s after being turned off", float64(i)/2)
		}
	}
}

func TestVoicesAreBounded(t *testing.T) {
	e := New(1)
	for range 100 {
		e.Play(Discovery)
	}
	if n := e.count(effectsBus); n > maxEffectNotes {
		t.Fatalf("%d effect voices, more than %d", n, maxEffectNotes)
	}
}

// A long read, or a burst of effects, never drops notes of the music.
func TestMusicKeepsItsNotes(t *testing.T) {
	short, long := New(5), New(5)
	render(short, 20)
	buf := make([]byte, 20*SampleRate*8)
	long.Read(buf)
	if short.compose.next != long.compose.next || len(short.voices) != len(long.voices) {
		t.Fatalf("a long read wrote different music: %d voices against %d", len(long.voices), len(short.voices))
	}
	e := New(5)
	render(e, 20)
	before := e.count(musicBus)
	for range 100 {
		e.Play(Discovery)
	}
	if e.count(musicBus) != before {
		t.Fatalf("effects pushed out music: %d music voices, then %d", before, e.count(musicBus))
	}
}

// The triangle has sixteen levels, within ±1.
func TestTriangleHasSixteenSteps(t *testing.T) {
	v := voice{n: Note{Dur: 1, Freq: 441, Wave: Triangle, Vol: 1}}
	levels := map[float64]bool{}
	for i := range SampleRate / 10 {
		s := v.sample(float64(i)/SampleRate) / v.envelope(float64(i)/SampleRate)
		if v.envelope(float64(i)/SampleRate) == 0 {
			continue
		}
		if math.Abs(s) > 1+1e-9 {
			t.Fatalf("triangle sample %.4f beyond ±1", s)
		}
		levels[math.Round(s*1e6)] = true
	}
	if len(levels) != 16 {
		t.Fatalf("%d levels, want 16", len(levels))
	}
}

func TestAmbienceSoundsOnlyWhenAsked(t *testing.T) {
	energy := func(a Ambience, muted bool) float64 {
		e := New(3)
		e.SetMusic(false)
		e.SetVolume(1)
		e.SetMuted(muted)
		e.SetAmbience(a)
		buf := make([]byte, SampleRate/10*8)
		sum := 0.0
		for range 40 {
			e.Read(buf)
			for i := 0; i < len(buf); i += 4 {
				v := float64(math.Float32frombits(binary.LittleEndian.Uint32(buf[i:])))
				sum += v * v
			}
		}
		return sum
	}
	if q := energy(Ambience{}, false); q != 0 {
		t.Fatalf("a quiet view makes noise: %v", q)
	}
	if m := energy(Ambience{Rain: 1, Birds: 1}, true); m != 0 {
		t.Fatalf("muted, the ambience still sounds: %v", m)
	}
	for name, a := range map[string]Ambience{"birds": {Birds: 1}, "crickets": {Crickets: 1}, "rain": {Rain: 1}, "work": {Work: 1}, "water": {Water: 1}} {
		if energy(a, false) == 0 {
			t.Errorf("no sound from %s", name)
		}
	}
}

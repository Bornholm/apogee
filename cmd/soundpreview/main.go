// Command soundpreview renders Apogée's effects and music to a WAV file, to
// hear them without launching the game.
package main

import (
	"encoding/binary"
	"flag"
	"log"
	"math"
	"os"

	"apogee/internal/sound"
)

func main() {
	out := flag.String("out", "apogee-sound.wav", "WAV file to write")
	seconds := flag.Float64("music", 40, "seconds of music per mood")
	seed := flag.Uint64("seed", 1, "seed of the music")
	effects := flag.Bool("effects", true, "start with every effect, one after the other")
	ambience := flag.Float64("ambience", 0, "seconds of each ambience (a morning, a building site, a rainy day, a night), alone, instead of the effects and the music")
	flag.Parse()

	var pcm []int16
	read := func(e *sound.Engine, s float64) {
		// Read by tenths of a second, as the audio thread does.
		buf := make([]byte, sound.SampleRate/10*8)
		for range int(s * 10) {
			n, _ := e.Read(buf)
			for i := 0; i < n; i += 4 {
				v := math.Float32frombits(binary.LittleEndian.Uint32(buf[i:]))
				pcm = append(pcm, int16(v*32767))
			}
		}
	}
	if *ambience > 0 {
		for _, a := range []sound.Ambience{
			{Birds: 1, Water: 0.6, Wind: 0.1},
			{Birds: 0.45, Water: 0.6, Work: 1, Wind: 0.2},
			{Rain: 1, Wind: 0.6, Water: 0.6},
			{Crickets: 0.8, Water: 0.6, Wind: 0.1},
		} {
			e := sound.New(*seed)
			e.SetVolume(1)
			e.SetMusic(false)
			e.SetAmbience(a)
			read(e, *ambience)
		}
		if err := writeWAV(*out, pcm); err != nil {
			log.Fatal(err)
		}
		return
	}
	if *effects {
		e := sound.New(*seed)
		e.SetVolume(1)
		e.SetMusic(false)
		for fx := range sound.NumEffects {
			e.Play(fx)
			read(e, 3)
		}
	}
	for _, m := range []sound.Mood{{Phase: "dawn"}, {Phase: "day"}, {Phase: "dusk"}, {Phase: "night"}, {Phase: "day", Decline: true}} {
		e := sound.New(*seed)
		e.SetVolume(1)
		e.SetMood(m)
		read(e, *seconds)
	}
	if err := writeWAV(*out, pcm); err != nil {
		log.Fatal(err)
	}
}

func writeWAV(path string, pcm []int16) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	data := uint32(len(pcm) * 2)
	hdr := []any{
		[4]byte{'R', 'I', 'F', 'F'}, 36 + data, [4]byte{'W', 'A', 'V', 'E'},
		[4]byte{'f', 'm', 't', ' '}, uint32(16), uint16(1), uint16(2), uint32(sound.SampleRate),
		uint32(sound.SampleRate * 4), uint16(4), uint16(16),
		[4]byte{'d', 'a', 't', 'a'}, data,
	}
	for _, h := range hdr {
		if err := binary.Write(f, binary.LittleEndian, h); err != nil {
			return err
		}
	}
	if err := binary.Write(f, binary.LittleEndian, pcm); err != nil {
		return err
	}
	return f.Close()
}

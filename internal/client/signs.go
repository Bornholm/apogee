package client

import (
	"image/color"
	"math"
	"math/rand/v2"

	"apogee/internal/geo"
	"apogee/internal/proto"
	"apogee/internal/sim"
)

// Signs make every significant event visible on the player's island (GDD
// 13.1 bis): Apogée is mostly watched, not played, so what happens must show
// without reading the chronicle. Some signs are brief, raised by a change of
// the state (a building finished, the island grown, a discovery, a
// whisper); others last as long as their cause (a supplique waiting, a wave
// coming, a wounded heart, the decline).

// signKind is a brief sign.
type signKind int

const (
	finishedSign  signKind = iota // a building is finished
	grownSign                     // the island grew
	discoverySign                 // the scholars mastered a discovery
	whisperSign                   // the god whispered
	birthSign                     // the island rose from the clouds
)

// signLength is how long each brief sign lasts, in seconds.
var signLength = [...]float64{finishedSign: 1.2, grownSign: 2.5, discoverySign: 3.5, whisperSign: 1.6, birthSign: 5}

type sign struct {
	kind signKind
	x, y float64
	t    float64
}

// lasting is what the lasting signs depend on, read from the state.
type lasting struct {
	supplique bool
	waveIn    float64 // seconds before a wave strikes the island, or -1
	heart     float64 // 0 to 1
	declining bool
	falling   float64 // 0 to 1, how far the island has fallen
}

type signs struct {
	primed bool
	heard  uint64          // newest chronicle entry considered
	built  map[uint32]bool // buildings already finished
	size   float64
	shore  shore
	heart  geo.Vec2 // where the heart stands
	now    []sign
	state  lasting
	r      *rand.Rand
	// rehearse names a sign to show at every state, for screenshots.
	rehearse string
	// calls and lifts are the gatherings of the people the state asks for,
	// for the game to pass to the island's life.
	calls []gathering
	lifts []string
	// starving is set during a famine: the people drag their feet.
	starving bool
}

// observe compares a new state of the player's island with the last one and
// raises the brief signs of what changed.
func (s *signs) observe(sum proto.IslandSummary, own *proto.Own) {
	if s.r == nil {
		s.r = rand.New(rand.NewPCG(sum.Seed, 0x5195))
	}
	if s.built == nil || s.shore.outline == nil || s.size == 0 {
		s.built = map[uint32]bool{}
		s.shore = shore{geo.Outline(sum.Seed), sum.Size}
	}
	library := geo.Vec2{X: math.NaN()}
	for _, b := range own.Buildings {
		switch {
		case b.Kind == sim.Core:
			s.heart = geo.Vec2{X: b.X, Y: b.Y}
		case b.Kind == sim.Library && b.Progress >= 1:
			library = geo.Vec2{X: b.X, Y: b.Y}
		}
		if b.Progress >= 1 && !s.built[b.ID] {
			s.built[b.ID] = true
			if s.primed && b.Kind != sim.Core {
				s.add(finishedSign, b.X, b.Y)
				s.calls = append(s.calls, gathering{key: "built", at: geo.Vec2{X: b.X, Y: b.Y}, radius: 6, share: 0.35, act: cheer, left: 6})
			}
		}
	}
	if s.primed && sum.Size > s.size+0.01 {
		s.add(grownSign, 0, 0)
		s.calls = append(s.calls, gathering{key: "grown", inPlace: true, share: 1, act: cheer, left: 3})
	}
	if sum.Size != s.size {
		s.size = sum.Size
		s.shore = shore{geo.Outline(sum.Seed), sum.Size}
	}
	for _, e := range own.Chronicle {
		if e.Tick <= s.heard {
			continue
		}
		if s.primed {
			switch e.Kind {
			case sim.DiscoveryNews:
				at := s.heart
				if !math.IsNaN(library.X) {
					at = library
				}
				s.add(discoverySign, at.X, at.Y)
				s.calls = append(s.calls, gathering{key: "discovery", at: at, radius: 6, share: 0.4, act: cheer, left: 7})
			case sim.BirthNews:
				s.add(birthSign, 0, 0)
				s.calls = append(s.calls, gathering{key: "birth", inPlace: true, share: 1, act: cheer, delay: 1, left: 4})
			case sim.DeclineNews:
				// The elders gather the people around the heart.
				s.calls = append(s.calls, gathering{key: "decline", at: s.heart, radius: 7, share: 0.7, act: pray, left: 12})
			}
		}
		s.heard = max(s.heard, e.Tick)
	}
	s.primed = true
	s.starving = own.Starving
	s.state = lasting{
		supplique: own.Pending != nil,
		waveIn:    -1,
		heart:     sum.Heart,
		declining: sum.Declining,
		falling:   math.Min(math.Max(1-float64(sum.FallingMs)/fallDuration, 0), 1),
	}
	if sum.FallingMs == 0 {
		s.state.falling = 0
	}
	if own.WaveInMs >= 0 {
		s.state.waveIn = float64(own.WaveInMs) / 1000
	}
	s.rehearsal()
	// Lasting gatherings follow their cause.
	if s.state.supplique {
		s.calls = append(s.calls, gathering{key: "supplique", at: s.heart, radius: 6, share: 0.5, act: pray})
	} else {
		s.lifts = append(s.lifts, "supplique")
	}
	if s.state.waveIn >= 0 && s.state.waveIn < 45 {
		s.calls = append(s.calls, gathering{key: "shelter", atHome: true, share: 1, act: hide})
	} else {
		s.lifts = append(s.lifts, "shelter")
	}
}

// rehearsal forces a sign, for screenshots and for trying the scene.
func (s *signs) rehearsal() {
	switch s.rehearse {
	case "finished":
		s.add(finishedSign, s.heart.X+8, s.heart.Y+4)
		s.calls = append(s.calls, gathering{key: "built", at: geo.Vec2{X: s.heart.X + 8, Y: s.heart.Y + 4}, radius: 6, share: 0.35, act: cheer, left: 6})
	case "grown":
		s.add(grownSign, 0, 0)
	case "discovery":
		s.add(discoverySign, s.heart.X, s.heart.Y)
		s.calls = append(s.calls, gathering{key: "discovery", at: s.heart, radius: 6, share: 0.4, act: cheer, left: 7})
	case "whisper":
		s.whispered()
	case "birth":
		s.add(birthSign, 0, 0)
	case "wave":
		s.state.waveIn = 5
	case "wound":
		s.state.heart = 0.3
	case "decline":
		s.state.declining = true
	case "supplique":
		s.state.supplique = true
	case "fall":
		s.state.falling = 0.4
	case "famine":
		s.starving = true
	}
}

// ShowSign shows a sign of the island at every state, by name: finished,
// grown, discovery, whisper, birth, wave, wound, decline, supplique, fall.
func (g *Game) ShowSign(name string) { g.signs.rehearse = name }

// add raises a brief sign.
func (s *signs) add(k signKind, x, y float64) {
	if len(s.now) < 16 {
		s.now = append(s.now, sign{kind: k, x: x, y: y})
	}
}

// whispered shows the god's breath going over the island.
func (s *signs) whispered() {
	s.add(whisperSign, s.heart.X, s.heart.Y)
	// The people hear their god: they stop and leap.
	s.calls = append(s.calls, gathering{key: "whisper", inPlace: true, share: 1, act: cheer, delay: 0.4, left: 2.5})
}

// Colours of the signs.
var (
	goldColours  = []color.RGBA{{0xff, 0xe0, 0x7a, 0xff}, {0xf2, 0xb8, 0x3c, 0xff}, {0xff, 0xf6, 0xd0, 0xff}}
	mistColours  = []color.RGBA{{0xff, 0xff, 0xff, 0xff}, {0xe4, 0xec, 0xf4, 0xff}}
	smokeColours = []color.RGBA{{0x4a, 0x14, 0x68, 0xff}, {0x3a, 0x30, 0x48, 0xff}, {0x5c, 0x4a, 0x6e, 0xff}}
	rockColours  = []color.RGBA{{0x8a, 0x80, 0x9c, 0xff}, {0x5e, 0x56, 0x70, 0xff}, {0xa0, 0x71, 0x4a, 0xff}}
)

// step advances the signs by dt seconds and raises their particles. emit
// is set when the player's island is in view.
func (s *signs) step(dt float64, dust *particles, emit bool) {
	if s.r == nil {
		return
	}
	kept := s.now[:0]
	for _, g := range s.now {
		g.t += dt
		if g.t >= signLength[g.kind] {
			continue
		}
		if emit {
			s.emit(g, dt, dust)
		}
		kept = append(kept, g)
	}
	s.now = kept
	if !emit {
		return
	}
	r, st := s.r, s.state
	chance := func(perSecond float64) bool { return r.Float64() < perSecond*dt }
	// A wave on its way: dark motes gather in the sky above the island,
	// more and more as it comes.
	if st.waveIn >= 0 && st.waveIn < 90 {
		if chance(4 + 30*(1-st.waveIn/90)) {
			p := s.somewhere(1.1)
			dust.add(particle{x: p.X, y: p.Y, z: 28 + r.Float64()*14, vz: -(1 + r.Float64()*2),
				sway: 1, phase: r.Float64() * 6, life: 3 + r.Float64()*2, col: pick(r, monsterColours)})
		}
	}
	// A wounded heart smokes, the more the deeper the wound.
	if st.heart < 0.9 && chance(1+8*(1-st.heart)) {
		dust.add(particle{x: s.heart.X + (r.Float64()*2-1)*1.5, y: s.heart.Y, z: 7,
			vx: 0.6, vz: 2 + r.Float64()*1.5, sway: 0.5, phase: r.Float64() * 6,
			life: 2.5 + r.Float64()*1.5, col: pick(r, smokeColours)})
	}
	// A declining island crumbles: stones fall from its cliffs.
	if (st.declining || st.falling > 0) && chance(3+40*st.falling) {
		p := s.edge()
		dust.add(particle{x: p.X, y: p.Y, z: -1, vz: -1, life: 2 + r.Float64(),
			col: pick(r, rockColours), heavy: true, deep: true})
	}
	// A supplique waits: a golden glow rises from the heart.
	if st.supplique && chance(5) {
		dust.add(particle{x: s.heart.X + (r.Float64()*2-1)*2, y: s.heart.Y + (r.Float64()*2-1)*1.2, z: 10,
			vz: 2.5 + r.Float64(), life: 1.5 + r.Float64(), col: pick(r, goldColours)})
	}
}

// emit raises the particles of a brief sign for one step.
func (s *signs) emit(g sign, dt float64, dust *particles) {
	r := s.r
	k := g.t / signLength[g.kind]
	switch g.kind {
	case finishedSign:
		// A shower of golden sparks over the new building.
		if g.t < 0.3 {
			for range 4 {
				a := r.Float64() * 2 * math.Pi
				v := 2 + r.Float64()*4
				dust.add(particle{x: g.x, y: g.y, z: 6, vx: v * math.Cos(a), vy: v * math.Sin(a) * 0.6,
					vz: 4 + r.Float64()*4, life: 0.8 + r.Float64()*0.6, col: pick(r, goldColours), heavy: true})
			}
		}
	case grownSign:
		// Dust rises all along the new shore.
		for range 3 {
			p := s.edge()
			dust.add(particle{x: p.X, y: p.Y, z: 0.5, vx: p.X * 0.05, vy: p.Y * 0.05, vz: 1.5 + r.Float64()*2,
				life: 1 + r.Float64(), col: pick(r, dustColours)})
		}
	case discoverySign:
		// A column of aether light rises from the library, or the heart.
		for range 3 {
			dust.add(particle{x: g.x + (r.Float64()*2-1)*1.2, y: g.y + (r.Float64()*2-1)*0.8, z: 4,
				vz: 12 + r.Float64()*8, life: 1.2 + r.Float64()*0.6, col: pick(r, sparkColours)})
		}
	case whisperSign:
		// A golden ring spreads from the heart over the whole island.
		radius := k * s.size * 1.1
		for range 6 {
			a := r.Float64() * 2 * math.Pi
			p := geo.Vec2{X: g.x + radius*math.Cos(a), Y: g.y + radius*math.Sin(a)}
			if geo.Inside(s.shore.outline, s.shore.size, p) {
				dust.add(particle{x: p.X, y: p.Y, z: 1, vz: 1.5, life: 0.4, col: pick(r, goldColours)})
			}
		}
	case birthSign:
		// The clouds the island rose from part and drift away.
		for range 4 {
			p := s.somewhere(1)
			dust.add(particle{x: p.X, y: p.Y, z: r.Float64() * 4, vx: p.X * 0.15, vy: p.Y * 0.15, vz: 0.5,
				life: 1.5 + r.Float64(), col: pick(r, mistColours)})
		}
	}
}

// somewhere returns a random point over the island, up to k times its size
// from the centre.
func (s *signs) somewhere(k float64) geo.Vec2 {
	a := s.r.Float64() * 2 * math.Pi
	d := math.Sqrt(s.r.Float64()) * s.size * k
	return geo.Vec2{X: d * math.Cos(a), Y: d * math.Sin(a)}
}

// edge returns a random point on the island's shore.
func (s *signs) edge() geo.Vec2 {
	if len(s.shore.outline) == 0 {
		return geo.Vec2{}
	}
	return s.shore.outline[s.r.IntN(len(s.shore.outline))].Scale(s.shore.size)
}

func pick(r *rand.Rand, cs []color.RGBA) color.RGBA { return cs[r.IntN(len(cs))] }

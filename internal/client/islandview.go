package client

import (
	"hash/fnv"
	"math"

	"apogee/internal/art"
	"apogee/internal/geo"
	"apogee/internal/proto"
	"apogee/internal/sim"
)

// validKind reports whether a building kind received from the wire is one
// the client knows, so that it can index the specs with it.
func validKind(k sim.Kind) bool { return k >= 0 && k < sim.NumKinds }

// meshFor returns the mesh of a finished building of a kind, in the
// style and the colour of its lineage (GDD 11.7).
func meshFor(k sim.Kind, seed uint64, l sim.Lineage) art.Mesh {
	style := art.Style(l.Style)
	var m art.Mesh
	switch k {
	case sim.Core:
		m = art.Core(seed)
	case sim.House:
		m = art.HouseIn(seed, style)
	case sim.Farm:
		m = art.Field(seed)
	case sim.Woodcutter:
		m = art.HutIn(seed, style)
	case sim.Quarry:
		m = art.QuarryPit(seed)
	case sim.Extractor:
		m = art.ExtractorTower(seed)
	case sim.Warehouse:
		m = art.BarnIn(seed, style)
	case sim.Tower:
		m = art.WatchTower(seed)
	case sim.Barracks:
		m = art.BarracksIn(seed, style)
	case sim.Library:
		m = art.Library(seed)
	}
	return art.Dress(m, int(l.Colour))
}

// signature summarises what the island view shows, to re-render it only
// when something visible changed.
func signature(sum proto.IslandSummary, own *proto.Own) uint64 {
	h := fnv.New64a()
	w := func(v uint64) {
		var b [8]byte
		for i := range b {
			b[i] = byte(v >> (8 * i))
		}
		h.Write(b[:])
	}
	w(sum.Seed)
	w(uint64(sum.Size * 10))
	w(uint64(sum.Trait))
	w(uint64(sum.Relic))                                             // chosen after the island rose
	w(uint64(sum.Lineage.Colour)<<16 | uint64(sum.Lineage.Style)<<8) // the god may change them
	for _, b := range own.Buildings {
		w(uint64(b.ID))
		w(uint64(b.Kind))
		w(uint64(b.Progress * 10)) // scaffolds rise in tenths
	}
	return h.Sum64()
}

// relicSpot is where the stele of an island's relic stands: just off the
// heart's plinth, where nothing may be built.
func relicSpot(sum proto.IslandSummary, own *proto.Own) (geo.Vec2, bool) {
	if sum.Relic == sim.NoRelic {
		return geo.Vec2{}, false
	}
	for _, b := range own.Buildings {
		if b.Kind == sim.Core {
			return geo.Vec2{X: b.X - 9.2, Y: b.Y + 2.4}, true
		}
	}
	return geo.Vec2{}, false
}

// islandScene builds the island of the player as the simulation sees it:
// the outline at its real size, the natural sites, the buildings and
// building sites where the people put them, and the people themselves.
func islandScene(cam art.Camera, sum proto.IslandSummary, own *proto.Own) art.IslandScene {
	mesh, _ := art.IslandStyled(sum.Seed, sum.Size, art.IslandStyle{High: sum.Trait == sim.HighIsle, Lush: sum.Trait == sim.FertileIsle})
	sc := art.IslandScene{Island: cam.Render(mesh)}
	art.PaintRivers(cam, sc.Island, geo.Rivers(sum.Seed, sum.Size))
	var springs []geo.Vec2
	for _, f := range own.Features {
		if f.Kind == sim.Spring {
			springs = append(springs, geo.Vec2{X: f.X, Y: f.Y})
		}
	}
	art.PaintPools(cam, sc.Island, springs, 4)
	add := func(m art.Mesh, x, y float64) {
		sc.Objects = append(sc.Objects, art.Placed{Sprite: cam.Render(m), Pos: art.V(x, y, 0)})
	}
	pine := func(m art.Mesh, x, y float64) {
		add(m, x, y)
		sc.Objects[len(sc.Objects)-1].Sways = true
	}
	for i, f := range own.Features {
		seed := sum.Seed*131 + uint64(i)*7
		switch f.Kind {
		case sim.Grove:
			// A grove is a cluster of pines filling its footprint.
			// The radius comes from the wire: a few dozen pines at most.
			n := min(int(f.Radius*1.6), 64)
			for k := range n {
				a := float64(k) * math.Pi * (3 - math.Sqrt(5))
				d := f.Radius * 0.85 * math.Sqrt((float64(k)+0.5)/float64(n))
				pine(art.Pine(seed+uint64(k)), f.X+d*math.Cos(a), f.Y+d*math.Sin(a))
			}
		case sim.Rock:
			add(art.Rock(seed), f.X, f.Y)
			add(art.Rock(seed+1), f.X+f.Radius*0.6, f.Y+f.Radius*0.4)
		case sim.Crystal:
			add(art.Crystal(seed), f.X, f.Y)
		}
	}
	for _, b := range own.Buildings {
		if !validKind(b.Kind) {
			continue // from the wire: never index with an unknown kind
		}
		seed := sum.Seed*977 + uint64(b.ID)
		if b.Progress < 1 {
			add(art.Scaffold(sim.Specs[b.Kind].Radius, b.Progress), b.X, b.Y)
			continue
		}
		add(meshFor(b.Kind, seed, sum.Lineage), b.X, b.Y)
	}

	// The stele of the relic saved from the civilisation before, beside
	// the heart (GDD 11.6).
	if c, ok := relicSpot(sum, own); ok {
		add(art.RelicStele(), c.X, c.Y)
	}

	// The people are not part of this still picture: they walk on top of it
	// (see life.go).
	return sc
}

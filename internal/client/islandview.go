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

// meshFor returns the mesh of a finished building of a kind.
func meshFor(k sim.Kind, seed uint64) art.Mesh {
	switch k {
	case sim.Core:
		return art.Core(seed)
	case sim.House:
		return art.House(seed)
	case sim.Farm:
		return art.Field(seed)
	case sim.Woodcutter:
		return art.Hut(seed)
	case sim.Quarry:
		return art.QuarryPit(seed)
	case sim.Extractor:
		return art.ExtractorTower(seed)
	case sim.Warehouse:
		return art.Barn(seed)
	case sim.Tower:
		return art.WatchTower(seed)
	case sim.Barracks:
		return art.Barracks(seed)
	case sim.Library:
		return art.Library(seed)
	}
	return art.Mesh{}
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
	for _, b := range own.Buildings {
		w(uint64(b.ID))
		w(uint64(b.Kind))
		w(uint64(b.Progress * 10)) // scaffolds rise in tenths
	}
	return h.Sum64()
}

// islandScene builds the island of the player as the simulation sees it:
// the outline at its real size, the natural sites, the buildings and
// building sites where the people put them, and the people themselves.
func islandScene(cam art.Camera, sum proto.IslandSummary, own *proto.Own) art.IslandScene {
	mesh, _ := art.Island(sum.Seed, sum.Size)
	sc := art.IslandScene{Island: cam.Render(mesh)}
	art.PaintRivers(cam, sc.Island, geo.Rivers(sum.Seed, sum.Size))
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
		add(meshFor(b.Kind, seed), b.X, b.Y)
	}

	// The people are not part of this still picture: they walk on top of it
	// (see life.go).
	return sc
}

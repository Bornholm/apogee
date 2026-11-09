package client

import (
	"hash/fnv"
	"math"

	"apogee/internal/art"
	"apogee/internal/geo"
	"apogee/internal/proto"
	"apogee/internal/sim"
)

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
	// Only what the picture shows: up to 24 villagers and 16 defenders.
	w(uint64(min(int(own.Population), 24)))
	w(uint64(min(int(own.Defenders), 16)))
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
	add := func(m art.Mesh, x, y float64) {
		sc.Objects = append(sc.Objects, art.Placed{Sprite: cam.Render(m), Pos: art.V(x, y, 0)})
	}
	r := art.Rng(sum.Seed ^ 0xfea7)
	for i, f := range own.Features {
		seed := sum.Seed*131 + uint64(i)*7
		switch f.Kind {
		case sim.Grove:
			// A grove is a cluster of pines filling its footprint.
			n := int(f.Radius * 1.6)
			for k := range n {
				a := float64(k) * math.Pi * (3 - math.Sqrt(5))
				d := f.Radius * 0.85 * math.Sqrt((float64(k)+0.5)/float64(n))
				add(art.Pine(seed+uint64(k)), f.X+d*math.Cos(a), f.Y+d*math.Sin(a))
			}
		case sim.Rock:
			add(art.Rock(seed), f.X, f.Y)
			add(art.Rock(seed+1), f.X+f.Radius*0.6, f.Y+f.Radius*0.4)
		case sim.Crystal:
			add(art.Crystal(seed), f.X, f.Y)
		}
	}
	for _, b := range own.Buildings {
		seed := sum.Seed*977 + uint64(b.ID)
		if b.Progress < 1 {
			add(art.Scaffold(sim.Specs[b.Kind].Radius, b.Progress), b.X, b.Y)
			continue
		}
		add(meshFor(b.Kind, seed), b.X, b.Y)
	}

	// People walk the free ground around the heart; defenders stand guard
	// near the towers and the core. Their places are drawn from the seed, so
	// the island does not flicker from one state to the next.
	outline := geo.Outline(sum.Seed)
	free := func(p geo.Vec2) bool {
		if geo.EdgeDistance(outline, sum.Size, p) < 3 {
			return false
		}
		for _, b := range own.Buildings {
			if p.Dist(geo.Vec2{X: b.X, Y: b.Y}) < sim.Specs[b.Kind].Radius+0.8 {
				return false
			}
		}
		for _, f := range own.Features {
			if p.Dist(geo.Vec2{X: f.X, Y: f.Y}) < f.Radius {
				return false
			}
		}
		return true
	}
	scatter := func(n int, team bool, near func(k int) geo.Vec2, spread float64) {
		for k, tries := 0, 0; k < n && tries < n*30; tries++ {
			c := near(k)
			p := geo.Vec2{X: c.X + (r.Float64()*2-1)*spread, Y: c.Y + (r.Float64()*2-1)*spread}
			if !free(p) {
				continue
			}
			v := art.Villager(sum.Seed + uint64(tries))
			if !team {
				v = recolor(v, art.Team, art.Plaster)
			}
			add(v, p.X, p.Y)
			k++
		}
	}
	scatter(min(int(own.Population), 24), false, func(int) geo.Vec2 { return geo.Vec2{} }, sum.Size*0.6)
	var posts []geo.Vec2
	for _, b := range own.Buildings {
		if b.Progress >= 1 && (b.Kind == sim.Tower || b.Kind == sim.Core) {
			posts = append(posts, geo.Vec2{X: b.X, Y: b.Y})
		}
	}
	if len(posts) == 0 {
		posts = []geo.Vec2{{}} // the heart's place, should no post stand
	}
	scatter(min(int(own.Defenders), 16), true, func(k int) geo.Vec2 { return posts[k%len(posts)] }, 9)
	return sc
}

// recolor swaps a material of a mesh, to dress villagers apart from soldiers.
func recolor(m art.Mesh, from, to art.Material) art.Mesh {
	for i := range m.Faces {
		if m.Faces[i].Mat == from {
			m.Faces[i].Mat = to
		}
	}
	return m
}

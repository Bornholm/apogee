package art

import (
	"math"
	"math/rand/v2"
)

// DemoIsland builds a populated island for the rendering spike. Placement is a
// stand-in for the simulation's (GDD 3.5): free positions inside the outline,
// with a minimum margin between footprints.
func DemoIsland(cam Camera, seed uint64, radius float64) IslandScene {
	r := Rng(seed)
	mesh, outline := Island(seed, radius)
	sc := IslandScene{Island: cam.Render(mesh)}

	type spot struct {
		p    Vec3
		size float64
	}
	var taken []spot
	free := func(p Vec3, size float64) bool {
		if !insideScaled(outline, p, 0.9) {
			return false
		}
		for _, t := range taken {
			if math.Hypot(p.X-t.p.X, p.Y-t.p.Y) < size+t.size {
				return false
			}
		}
		return true
	}
	place := func(m Mesh, p Vec3, size float64) {
		taken = append(taken, spot{p, size})
		sc.Objects = append(sc.Objects, Placed{Sprite: cam.Render(m), Pos: p})
	}
	try := func(n int, size float64, sample func() Vec3, build func(i int) Mesh) {
		for i, tries := 0, 0; i < n && tries < n*40; tries++ {
			p := sample()
			if free(p, size) {
				place(build(i), p, size)
				i++
			}
		}
	}
	polar := func(r *rand.Rand, lo, hi float64) func() Vec3 {
		return func() Vec3 {
			a := r.Float64() * 2 * math.Pi
			d := radius * math.Sqrt(between(r, lo*lo, hi*hi))
			return V(d*math.Cos(a), d*math.Sin(a), 0)
		}
	}

	place(Core(seed), V(0, 0, 0), 7)
	try(5, 6, polar(r, 0.22, 0.5), func(i int) Mesh { return House(seed*31 + uint64(i)) })

	// Groves: pines cluster around a few centres near the rim.
	for g := range 3 {
		a := r.Float64() * 2 * math.Pi
		c := V(radius*0.68*math.Cos(a), radius*0.68*math.Sin(a), 0)
		try(9, 2.8, func() Vec3 {
			return c.Add(V(between(r, -9, 9), between(r, -9, 9), 0))
		}, func(i int) Mesh { return Pine(seed*97 + uint64(g*16+i)) })
	}
	try(6, 3.5, polar(r, 0.3, 0.85), func(i int) Mesh { return Rock(seed*53 + uint64(i)) })
	try(3, 2.5, polar(r, 0.6, 0.85), func(i int) Mesh {
		m := Crystal(seed*71 + uint64(i))
		return m
	})
	try(8, 1.2, polar(r, 0.15, 0.55), func(i int) Mesh { return Villager(seed*13 + uint64(i)) })
	return sc
}

// insideScaled reports whether p lies inside the outline shrunk by k towards
// the origin, using the even-odd rule.
func insideScaled(outline []Vec3, p Vec3, k float64) bool {
	in := false
	for i, a := range outline {
		b := outline[(i+1)%len(outline)]
		ax, ay, bx, by := a.X*k, a.Y*k, b.X*k, b.Y*k
		if (ay > p.Y) != (by > p.Y) && p.X < ax+(p.Y-ay)/(by-ay)*(bx-ax) {
			in = !in
		}
	}
	return in
}

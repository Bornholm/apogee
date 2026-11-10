package art

import "math"

// Building meshes. Each fits the footprint the simulation reserves for its
// kind (sim.Specs[...].Radius) and takes its variations from the seed.

// Field is a farm: rows of ripe crops on tilled soil, with a small shed.
func Field(seed uint64) Mesh {
	r := Rng(seed)
	var m Mesh
	w, d := between(r, 3.6, 4.2), between(r, 2.6, 3.0)
	// Soil, flush with the ground: shadows may fall on it.
	m.Add(Earth, V(-w, -d, 0.05), V(w, -d, 0.05), V(w, d, 0.05), V(-w, d, 0.05))
	for y := -d + 0.5; y < d-0.4; y += 1.2 {
		box(&m, Crop, -w+0.4, y, 0, w-0.4, y+0.6, between(r, 0.6, 0.9))
	}
	box(&m, Wood, w-1.2, -d-1.6, 0, w+0.4, -d-0.2, 1.8)
	m.RotZ(float64(r.IntN(2)) * math.Pi / 2)
	return m
}

// Hut is a woodcutter's cabin with its log pile.
func Hut(seed uint64) Mesh { return HutIn(seed, Gabled) }

// HutIn is a woodcutter's cabin in a style.
func HutIn(seed uint64, style Style) Mesh {
	r := Rng(seed)
	var m Mesh
	box(&m, Wood, -1.8, -1.5, 0, 1.8, 1.5, 3)
	roofOf(&m, style, Roof, Wood, 1.8, 1.5, 3, 1.8, 0.4)
	for i := range 3 {
		z := float64(i) * 0.7
		box(&m, Wood, 2.2, -1+float64(i)*0.2, z, 3.2, 1.2-float64(i)*0.2, z+0.6)
	}
	m.RotZ(float64(r.IntN(4)) * math.Pi / 2)
	return m
}

// QuarryPit is a quarry: stacked cut stones and a hoist.
func QuarryPit(seed uint64) Mesh {
	r := Rng(seed)
	var m Mesh
	m.Add(Underside, V(-2.6, -2, 0.05), V(2.6, -2, 0.05), V(2.6, 2, 0.05), V(-2.6, 2, 0.05))
	for range 4 {
		x, y := between(r, -2, 1.2), between(r, -1.6, 0.8)
		s := between(r, 0.6, 0.9)
		box(&m, Stone, x, y, 0, x+s*1.4, y+s, s)
	}
	box(&m, Wood, -2.4, 1.4, 0, -2.0, 1.8, 4)
	box(&m, Wood, -2.4, -0.2, 3.6, -2.0, 1.8, 4)
	return m
}

// ExtractorTower is an aether extractor: a stone base holding a crystal in a
// wooden frame.
func ExtractorTower(seed uint64) Mesh {
	r := Rng(seed)
	var m Mesh
	box(&m, Stone, -1.6, -1.6, 0, 1.6, 1.6, 1.6)
	for _, p := range [][2]float64{{-1.4, -1.4}, {1.0, -1.4}, {-1.4, 1.0}, {1.0, 1.0}} {
		box(&m, Wood, p[0], p[1], 1.6, p[0]+0.4, p[1]+0.4, 6)
	}
	var gem Mesh
	rim := ring(r, 5, 1, 0.1, 4, 0)
	fan(&gem, Aether, rim, V(0, 0, 7.5))
	fan(&gem, Aether, rim, V(0, 0, 2))
	gem.OrientOutward(V(0, 0, 4))
	m.Merge(gem)
	return m
}

// Barn is a warehouse: a wide wooden barn under a large roof.
func Barn(seed uint64) Mesh { return BarnIn(seed, Gabled) }

// BarnIn is a warehouse in a style.
func BarnIn(seed uint64, style Style) Mesh {
	r := Rng(seed)
	var m Mesh
	w, d, h := between(r, 3.2, 3.8), 2.6, between(r, 3.5, 4.2)
	box(&m, Wood, -w, -d, 0, w, d, h)
	roofOf(&m, style, Roof, Wood, w, d, h, 2.6, 0.5)
	m.RotZ(float64(r.IntN(2)) * math.Pi / 2)
	return m
}

// WatchTower is a defensive tower: an octagonal stone shaft under a roof.
func WatchTower(seed uint64) Mesh {
	r := Rng(seed)
	var m Mesh
	h := between(r, 11, 14)
	var shaft Mesh
	bottom := ring(r, 8, 2.6, 0, 0, math.Pi/8)
	top := ring(r, 8, 2.2, 0, h, math.Pi/8)
	loft(&shaft, Stone, bottom, top)
	shaft.Add(Stone, top...)
	shaft.OrientOutward(V(0, 0, h/2))
	m.Merge(shaft)
	var cap Mesh
	eaves := ring(r, 8, 3, 0, h, math.Pi/8)
	fan(&cap, Roof, eaves, V(0, 0, h+3.5))
	cap.Add(Roof, eaves...)
	cap.OrientOutward(V(0, 0, h+1))
	m.Merge(cap)
	return m
}

// Barracks is a long plastered hall flying the team's banner.
func Barracks(seed uint64) Mesh { return BarracksIn(seed, Gabled) }

// BarracksIn is a barracks in a style.
func BarracksIn(seed uint64, style Style) Mesh {
	r := Rng(seed)
	var m Mesh
	box(&m, Plaster, -4, -2.4, 0, 4, 2.4, 4)
	roofOf(&m, style, Roof, Plaster, 4, 2.4, 4, 2.4, 0.5)
	box(&m, Wood, 4.4, -0.2, 0, 4.8, 0.2, 9)
	box(&m, Team, 4.8, -0.1, 6.2, 7, 0.1, 8.8)
	m.RotZ(float64(r.IntN(2)) * math.Pi)
	return m
}

// Library is a house of scholars: stone walls under an aether dome.
func Library(seed uint64) Mesh {
	r := Rng(seed)
	var m Mesh
	var walls Mesh
	bottom := ring(r, 8, 3.8, 0, 0, math.Pi/8)
	top := ring(r, 8, 3.8, 0, 4, math.Pi/8)
	loft(&walls, Stone, bottom, top)
	walls.OrientOutward(V(0, 0, 2))
	m.Merge(walls)
	var dome Mesh
	mid := ring(r, 8, 3, 0, 6, math.Pi/8)
	loft(&dome, Aether, top, mid)
	fan(&dome, Aether, mid, V(0, 0, 7.6))
	dome.OrientOutward(V(0, 0, 4))
	m.Merge(dome)
	return m
}

// Scaffold is a building site: a wooden frame rising with progress (0 to 1)
// over a footprint of the given radius.
func Scaffold(radius, progress float64) Mesh {
	var m Mesh
	s := radius * 0.7
	m.Add(Wood, V(-s, -s, 0.2), V(s, -s, 0.2), V(s, s, 0.2), V(-s, s, 0.2))
	h := 1 + 5*math.Min(math.Max(progress, 0), 1)
	for _, p := range [][2]float64{{-s, -s}, {s - 0.4, -s}, {-s, s - 0.4}, {s - 0.4, s - 0.4}} {
		box(&m, Wood, p[0], p[1], 0, p[0]+0.4, p[1]+0.4, h)
	}
	box(&m, Wood, -s, -s, h-0.3, s, -s+0.3, h)
	box(&m, Wood, -s, s-0.3, h-0.3, s, s, h)
	return m
}

// gable adds a gabled roof over a box of half sizes w × d and wall height h,
// with its ridge along X.
func gable(m *Mesh, roof, wall Material, w, d, h, rise, overhang float64) {
	var g Mesh
	o := overhang
	a, b := V(-w-o, -d-o, h), V(w+o, -d-o, h)
	c, e := V(w+o, d+o, h), V(-w-o, d+o, h)
	ra, rb := V(-w-o, 0, h+rise), V(w+o, 0, h+rise)
	g.Add(roof, a, b, rb, ra)
	g.Add(roof, e, c, rb, ra)
	g.Add(wall, V(-w, -d, h), V(-w, d, h), V(-w, 0, h+rise-0.4))
	g.Add(wall, V(w, -d, h), V(w, d, h), V(w, 0, h+rise-0.4))
	g.Add(roof, a, b, c, e)
	g.OrientOutward(V(0, 0, h+0.3))
	m.Merge(g)
}

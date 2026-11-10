package art

import (
	"math"
	"math/rand/v2"

	"apogee/internal/geo"
)

// Rng returns the deterministic generator used by every template: the same
// seed always yields the same mesh, on every machine (GDD 12.2).
func Rng(seed uint64) *rand.Rand {
	return rand.New(rand.NewPCG(seed, 0x9e3779b97f4a7c15))
}

func between(r *rand.Rand, lo, hi float64) float64 { return lo + r.Float64()*(hi-lo) }

// ring returns n points on an irregular horizontal ring at height z.
func ring(r *rand.Rand, n int, radius, jitter, z, phase float64) []Vec3 {
	pts := make([]Vec3, n)
	for i := range pts {
		a := phase + 2*math.Pi*float64(i)/float64(n)
		rr := radius * (1 + between(r, -jitter, jitter))
		pts[i] = V(rr*math.Cos(a), rr*math.Sin(a), z)
	}
	return pts
}

// loft joins two rings of the same size with quads.
func loft(m *Mesh, mat Material, lower, upper []Vec3) {
	n := len(lower)
	for i := range n {
		j := (i + 1) % n
		m.Add(mat, lower[i], lower[j], upper[j], upper[i])
	}
}

// fan closes a ring with triangles meeting at apex.
func fan(m *Mesh, mat Material, rim []Vec3, apex Vec3) {
	n := len(rim)
	for i := range n {
		m.Add(mat, rim[i], rim[(i+1)%n], apex)
	}
}

// box adds an axis-aligned box spanning [x0,x1]×[y0,y1]×[z0,z1].
func box(m *Mesh, mat Material, x0, y0, z0, x1, y1, z1 float64) {
	var b Mesh
	p := func(x, y, z float64) Vec3 { return V(x, y, z) }
	b.Add(mat, p(x0, y0, z1), p(x1, y0, z1), p(x1, y1, z1), p(x0, y1, z1)) // top
	b.Add(mat, p(x0, y0, z0), p(x1, y0, z0), p(x1, y1, z0), p(x0, y1, z0)) // bottom
	b.Add(mat, p(x0, y1, z0), p(x1, y1, z0), p(x1, y1, z1), p(x0, y1, z1)) // south
	b.Add(mat, p(x0, y0, z0), p(x1, y0, z0), p(x1, y0, z1), p(x0, y0, z1)) // north
	b.Add(mat, p(x1, y0, z0), p(x1, y1, z0), p(x1, y1, z1), p(x1, y0, z1)) // east
	b.Add(mat, p(x0, y0, z0), p(x0, y1, z0), p(x0, y1, z1), p(x0, y0, z1)) // west
	b.OrientOutward(V((x0+x1)/2, (y0+y1)/2, (z0+z1)/2))
	m.Merge(b)
}

// Pine is a conifer: a short trunk under one or two faceted cones.
func Pine(seed uint64) Mesh {
	r := Rng(seed)
	var m Mesh
	trunk := between(r, 1.2, 2.2)
	box(&m, Wood, -0.6, -0.6, 0, 0.6, 0.6, trunk)

	tiers := 1 + r.IntN(2)
	radius := between(r, 2.8, 4.2)
	height := between(r, 8, 12)
	z := trunk
	for t := range tiers {
		var c Mesh
		n := 5 + r.IntN(3)
		rim := ring(r, n, radius, 0.15, z, r.Float64()*math.Pi)
		h := height / float64(tiers) * (1.3 - 0.3*float64(t))
		apex := V(between(r, -0.4, 0.4), between(r, -0.4, 0.4), z+h)
		fan(&c, Foliage, rim, apex)
		c.Add(Foliage, rim...) // underside, culled but keeps the shadow solid
		c.OrientOutward(V(0, 0, z+h/3))
		m.Merge(c)
		z += h * 0.55
		radius *= 0.72
	}
	return m
}

// Rock is a low faceted boulder.
func Rock(seed uint64) Mesh {
	r := Rng(seed)
	var m Mesh
	n := 5 + r.IntN(3)
	radius := between(r, 1.8, 3.5)
	base := ring(r, n, radius, 0.25, 0, r.Float64()*math.Pi)
	top := make([]Vec3, n)
	for i, b := range base {
		k := between(r, 0.45, 0.7)
		top[i] = V(b.X*k+between(r, -0.3, 0.3), b.Y*k, radius*between(r, 0.5, 0.85))
	}
	loft(&m, Boulder, base, top)
	m.Add(Boulder, top...)
	m.OrientOutward(V(0, 0, radius*0.3))
	return m
}

// House is a small dwelling: plaster walls under a gabled roof. Its yaw comes
// from the seed; tones are computed after rotation, so the light stays right.
func House(seed uint64) Mesh {
	r := Rng(seed)
	var m Mesh
	w := between(r, 2.8, 3.8) // half length along the ridge
	d := between(r, 2.2, 2.8) // half depth
	h := between(r, 5, 6.5)   // wall height
	rh := between(r, 3, 4)
	box(&m, Plaster, -w, -d, 0, w, d, h)

	o := 0.6 // eaves overhang
	var roof Mesh
	a, b := V(-w-o, -d-o, h), V(w+o, -d-o, h)
	c, e := V(w+o, d+o, h), V(-w-o, d+o, h)
	ra, rb := V(-w-o, 0, h+rh), V(w+o, 0, h+rh)
	roof.Add(Roof, a, b, rb, ra)                                     // north slope
	roof.Add(Roof, e, c, rb, ra)                                     // south slope
	roof.Add(Plaster, V(-w, -d, h), V(-w, d, h), V(-w, 0, h+rh-0.4)) // west gable
	roof.Add(Plaster, V(w, -d, h), V(w, d, h), V(w, 0, h+rh-0.4))    // east gable
	roof.Add(Roof, a, b, c, e)                                       // underside
	roof.OrientOutward(V(0, 0, h+0.3))
	m.Merge(roof)

	m.RotZ(float64(r.IntN(4)) * math.Pi / 2)
	return m
}

// Crystal is a floating shard of aether, an elongated octahedron.
func Crystal(seed uint64) Mesh {
	r := Rng(seed)
	var m Mesh
	n := 4 + r.IntN(2)
	radius := between(r, 1.4, 2.2)
	mid := between(r, 4, 6)
	rim := ring(r, n, radius, 0.1, mid, r.Float64()*math.Pi)
	fan(&m, Aether, rim, V(0, 0, mid+between(r, 4, 6)))
	fan(&m, Aether, rim, V(0, 0, mid-between(r, 2.5, 3.5)))
	m.OrientOutward(V(0, 0, mid))
	return m
}

// Core is the heart of an island (GDD 3.3): a stone plinth carrying a large
// aether crystal that keeps the island aloft.
func Core(seed uint64) Mesh {
	r := Rng(seed)
	var m Mesh
	var plinth Mesh
	bottom := ring(r, 8, 7, 0, 0, math.Pi/8)
	top := ring(r, 8, 5.5, 0, 3.5, math.Pi/8)
	loft(&plinth, Stone, bottom, top)
	plinth.Add(Stone, top...)
	plinth.OrientOutward(V(0, 0, 1))
	m.Merge(plinth)

	var gem Mesh
	rim := ring(r, 6, 3.4, 0.05, 13, 0)
	fan(&gem, Aether, rim, V(0, 0, 25))
	fan(&gem, Aether, rim, V(0, 0, 6))
	gem.OrientOutward(V(0, 0, 13))
	m.Merge(gem)
	return m
}

// CloudPuff is a flattened faceted cloud drifting below the islands.
func CloudPuff(seed uint64) Mesh {
	r := Rng(seed)
	var m Mesh
	n := 6 + r.IntN(3)
	radius := between(r, 8, 14)
	base := ring(r, n, radius, 0.2, 0, r.Float64()*math.Pi)
	for i := range base {
		base[i].Y *= 0.6
	}
	top := make([]Vec3, n)
	for i, b := range base {
		top[i] = V(b.X*0.6, b.Y*0.6, radius*between(r, 0.35, 0.5))
	}
	loft(&m, Cloud, base, top)
	m.Add(Cloud, top...)
	m.OrientOutward(V(0, 0, radius*0.15))
	return m
}

// Villager is a tiny unit of the given team material.
func Villager(seed uint64) Mesh { return Person(seed, Team) }

// Person is a tiny figure: brown legs, a tunic of the given material and a
// light head, readable even a few pixels tall.
func Person(seed uint64, tunic Material) Mesh {
	r := Rng(seed)
	var m Mesh
	h := between(r, 3.2, 3.8)
	box(&m, Wood, -0.6, -0.6, 0, 0.6, 0.6, h*0.35)
	box(&m, tunic, -0.8, -0.7, h*0.35, 0.8, 0.7, h*0.78)
	box(&m, Plaster, -0.55, -0.55, h*0.78, 0.55, 0.55, h)
	return m
}

// Child is a small villager, a head shorter, in a bright tunic.
func Child(seed uint64, tunic Material) Mesh {
	r := Rng(seed)
	var m Mesh
	h := between(r, 2.2, 2.5)
	box(&m, Wood, -0.45, -0.45, 0, 0.45, 0.45, h*0.3)
	box(&m, tunic, -0.6, -0.55, h*0.3, 0.6, 0.55, h*0.72)
	box(&m, Plaster, -0.5, -0.5, h*0.72, 0.5, 0.5, h)
	return m
}

// Dog is a small four-legged companion: a body, a head held up, a tail.
func Dog(seed uint64) Mesh {
	r := Rng(seed)
	var m Mesh
	coat := Wood
	if r.IntN(2) == 0 {
		coat = Earth
	}
	box(&m, coat, -1.1, -0.45, 0.6, 0.9, 0.45, 1.4)  // body
	box(&m, coat, 0.7, -0.4, 1.1, 1.6, 0.4, 2.0)     // head
	box(&m, Wood, -1.0, -0.4, 0, -0.6, 0.4, 0.6)     // hind legs
	box(&m, Wood, 0.4, -0.4, 0, 0.8, 0.4, 0.6)       // fore legs
	box(&m, coat, -1.6, -0.15, 1.2, -1.1, 0.15, 1.9) // tail
	return m
}

// Island is a floating island (GDD 3.2 and 12.6): a flat grassy top with an
// irregular outline, a band of faceted earth cliffs and a rocky cone beneath.
// It returns the mesh and the outline of its top, in world coordinates.
func Island(seed uint64, radius float64) (Mesh, []Vec3) {
	return IslandStyled(seed, radius, IslandStyle{})
}

// IslandStyle varies an island with the trait of its land (GDD 11.6).
type IslandStyle struct {
	High bool // taller cliffs
	Lush bool // greener grass
}

// IslandStyled is an island in a style.
func IslandStyled(seed uint64, radius float64, style IslandStyle) (Mesh, []Vec3) {
	// The outline is shared with the simulation, which places buildings
	// inside it (internal/geo); the rest of the shape is decorative.
	outline := geo.Outline(seed)
	n := len(outline)
	top := make([]Vec3, n)
	for i, p := range outline {
		top[i] = V(p.X*radius, p.Y*radius, 0)
	}
	r := Rng(seed)

	var m Mesh
	grass := Grass
	if style.Lush {
		grass = Lush
	}
	m.Add(grass, top...)

	cliff := radius * 0.28
	if style.High {
		cliff *= 1.6
	}
	lip := make([]Vec3, n)
	for i, p := range top {
		k := between(r, 0.94, 0.99)
		lip[i] = V(p.X*k, p.Y*k, -cliff*between(r, 0.8, 1.2))
	}
	loft(&m, Earth, lip, top)

	// The underside must be steep enough for its faces to turn towards the
	// camera: a flat cone would only show faces looking down.
	// Its rings shrink and twist slightly as they go down, like an inverted
	// mountain torn from the ground.
	depth := radius * 2.4
	upper := lip
	for _, lv := range [...]struct{ k, z float64 }{{0.72, 0.22}, {0.45, 0.5}, {0.22, 0.75}} {
		next := make([]Vec3, n)
		for i, q := range lip {
			k := lv.k * between(r, 0.88, 1.12)
			next[i] = V(q.X*k+between(r, -1, 1), q.Y*k+between(r, -1, 1),
				-cliff-depth*lv.z*between(r, 0.92, 1.08))
		}
		loft(&m, Underside, next, upper)
		upper = next
	}
	fan(&m, Underside, upper, V(between(r, -2, 2), between(r, -2, 2), -cliff-depth))

	m.OrientOutward(V(0, 0, -cliff))
	// The top must face up whatever the outline's winding.
	if m.Faces[0].Normal().Z < 0 {
		v := m.Faces[0].V
		for a, b := 0, len(v)-1; a < b; a, b = a+1, b-1 {
			v[a], v[b] = v[b], v[a]
		}
	}
	return m, top
}

// Beacon is the floating golden gem that marks the player's own island in the
// sky view. Gold stands out against the blue sky and the blue teams.
func Beacon() Mesh {
	var m Mesh
	rim := []Vec3{V(3.5, 0, 6), V(0, 3.5, 6), V(-3.5, 0, 6), V(0, -3.5, 6)}
	fan(&m, Self, rim, V(0, 0, 11))
	fan(&m, Self, rim, V(0, 0, 0))
	m.OrientOutward(V(0, 0, 6))
	return m
}

// Swarm is a wave of monsters out of the portal: a cluster of dark shards.
func Swarm(seed uint64) Mesh {
	r := Rng(seed)
	var m Mesh
	for i := range 7 {
		var shard Mesh
		a := float64(i) * 2.4
		d := between(r, 0, 14)
		c := V(d*math.Cos(a), d*math.Sin(a), between(r, 5, 16))
		rim := ring(r, 3, between(r, 3, 4.5), 0.2, c.Z, r.Float64()*math.Pi)
		for j := range rim {
			rim[j] = rim[j].Add(V(c.X, c.Y, 0))
		}
		fan(&shard, Monster, rim, c.Add(V(0, 0, between(r, 4.5, 7))))
		fan(&shard, Monster, rim, c.Add(V(0, 0, -between(r, 4.5, 7))))
		shard.OrientOutward(c)
		m.Merge(shard)
	}
	return m
}

// Creature is one monster of a wave, fought on the island: a dark shard
// floating above the ground. The colossus is the same, larger.
func Creature(seed uint64, scale float64) Mesh {
	return CreatureOf(seed, scale, Monster)
}

// CreatureOf is a creature of the whirlpool in a material: each species
// of the bestiary has its colour (GDD 11.6).
func CreatureOf(seed uint64, scale float64, mat Material) Mesh {
	r := Rng(seed)
	var m Mesh
	c := V(0, 0, 4*scale)
	rim := ring(r, 4, between(r, 1.6, 2.2)*scale, 0.25, c.Z, r.Float64()*math.Pi)
	fan(&m, mat, rim, c.Add(V(0, 0, between(r, 2.6, 3.4)*scale)))
	fan(&m, mat, rim, c.Add(V(0, 0, -between(r, 2, 2.8)*scale)))
	m.OrientOutward(c)
	return m
}

// Banner is a war banner on a pole, raised above an island that covets or
// means to annihilate a neighbour (GDD 7.2).
func Banner(cloth Material) Mesh {
	var m Mesh
	box(&m, Wood, -0.4, -0.4, 0, 0.4, 0.4, 14)
	box(&m, cloth, 0.4, -0.2, 8, 7, 0.2, 13.5)
	return m
}

// Warband is a group of units crossing the sky: assailants in the team's
// colour, relief forces in plaster white.
func Warband(seed uint64, mat Material) Mesh {
	r := Rng(seed)
	var m Mesh
	for i := range 5 {
		a := float64(i) * 2.4
		d := between(r, 0, 3)
		x, y := d*math.Cos(a), d*math.Sin(a)
		box(&m, mat, x-0.7, y-0.7, 0, x+0.7, y+0.7, between(r, 2.4, 3.2))
	}
	return m
}

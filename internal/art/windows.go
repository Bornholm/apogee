package art

import "math"

// Windows returns where the lit windows of a building go: on the walls of
// the given materials that face the viewer, at a fraction of their height,
// one every three units along them. The night lights them (GDD 12.5).
func Windows(m Mesh, at float64, mats ...Material) []Vec3 {
	var out []Vec3
	for _, f := range m.Faces {
		if !hasMat(mats, f.Mat) {
			continue
		}
		n := f.Normal()
		if n.Y < 0.7 || math.Abs(n.Z) > 0.2 {
			continue // not a wall facing the viewer
		}
		x0, x1 := math.Inf(1), math.Inf(-1)
		z0, z1 := math.Inf(1), math.Inf(-1)
		y := math.Inf(-1)
		for _, v := range f.V {
			x0, x1 = math.Min(x0, v.X), math.Max(x1, v.X)
			z0, z1 = math.Min(z0, v.Z), math.Max(z1, v.Z)
			y = math.Max(y, v.Y)
		}
		if z0 > 0.5 || z1-z0 < 1.5 || x1-x0 < 1 {
			continue // a gable end, a roof or a sliver
		}
		k := max(1, int((x1-x0)/3))
		for i := range k {
			x := x0 + (x1-x0)*(float64(i)+0.5)/float64(k)
			out = append(out, V(x, y+0.05, z0+(z1-z0)*at))
		}
	}
	return out
}

func hasMat(mats []Material, m Material) bool {
	for _, x := range mats {
		if x == m {
			return true
		}
	}
	return false
}

// Chimney returns where smoke leaves a roof: on the ridge, towards one end.
func Chimney(m Mesh) Vec3 {
	top := V(0, 0, math.Inf(-1))
	for _, f := range m.Faces {
		for _, v := range f.V {
			if v.Z > top.Z+1e-9 || (math.Abs(v.Z-top.Z) < 1e-9 && v.X+v.Y > top.X+top.Y) {
				top = v
			}
		}
	}
	return V(top.X*0.55, top.Y*0.55, top.Z-0.2)
}

// Stele is a grave: a rounded standing stone on a low plinth.
func Stele(seed uint64) Mesh {
	r := Rng(seed)
	var m Mesh
	h := between(r, 1.8, 2.4)
	box(&m, Stone, -0.9, -0.5, 0, 0.9, 0.5, 0.3)
	box(&m, Stone, -0.6, -0.25, 0.3, 0.6, 0.25, h)
	box(&m, Stone, -0.4, -0.25, h, 0.4, 0.25, h+0.3)
	return m
}

// RelicStele is the stone a people raises to the treasure saved from the
// civilisation before: a tall plinth under a golden crystal (GDD 11.6).
func RelicStele() Mesh {
	var m Mesh
	box(&m, Stone, -2, -1.5, 0, 2, 1.5, 1)
	box(&m, Stone, -1, -0.8, 1, 1, 0.8, 6.5)
	var gem Mesh
	rim := ring(Rng(3), 5, 1.6, 0, 8, 0.3)
	fan(&gem, Self, rim, V(0, 0, 11))
	fan(&gem, Self, rim, V(0, 0, 6.5))
	gem.OrientOutward(V(0, 0, 8))
	m.Merge(gem)
	return m
}

// Sheep is a woolly grazer of the fields: a white fleece and a dark head.
func Sheep(seed uint64) Mesh {
	r := Rng(seed)
	var m Mesh
	l := between(r, 1.1, 1.4)
	box(&m, Wood, -l+0.3, -0.5, 0, l-0.3, 0.5, 0.5)    // legs
	box(&m, Plaster, -l, -0.75, 0.5, l, 0.75, 1.6)     // fleece
	box(&m, Wood, l-0.1, -0.35, 0.9, l+0.7, 0.35, 1.7) // head
	if r.IntN(2) == 0 {
		m.RotZ(math.Pi)
	}
	return m
}

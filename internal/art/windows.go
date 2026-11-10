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

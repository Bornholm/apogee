// Package geo holds the 2D geometry shared by the simulation and the
// rendering, so that both agree on where an island's edge is.
package geo

import (
	"math"
	"math/rand/v2"
)

// Vec2 is a point on an island's ground plane: X east, Y south.
type Vec2 struct{ X, Y float64 }

// Dist returns the distance between two points.
func (a Vec2) Dist(b Vec2) float64 { return math.Hypot(a.X-b.X, a.Y-b.Y) }

// Len returns the distance from the origin.
func (a Vec2) Len() float64 { return math.Hypot(a.X, a.Y) }

// Scale multiplies both coordinates by k.
func (a Vec2) Scale(k float64) Vec2 { return Vec2{a.X * k, a.Y * k} }

// Outline returns the irregular outline of an island of radius 1, centred on
// the origin and listed counter-clockwise. The shape depends only on the seed;
// an island grows by scaling it (GDD 3.2).
func Outline(seed uint64) []Vec2 {
	r := rand.New(rand.NewPCG(seed, 0x51ed270b2e9a1c4d))
	n := 18 + r.IntN(6)
	var phases [3]float64
	for i := range phases {
		phases[i] = r.Float64() * 2 * math.Pi
	}
	out := make([]Vec2, n)
	for i := range out {
		a := 2 * math.Pi * float64(i) / float64(n)
		k := 1 + 0.12*math.Sin(2*a+phases[0]) + 0.08*math.Sin(3*a+phases[1]) +
			0.05*math.Sin(5*a+phases[2]) + (r.Float64()*2-1)*0.04
		out[i] = Vec2{k * math.Cos(a), k * math.Sin(a)}
	}
	return out
}

// Inside reports whether p lies inside the polygon scaled by k around the
// origin, with the even-odd rule.
func Inside(poly []Vec2, k float64, p Vec2) bool {
	in := false
	for i, a := range poly {
		b := poly[(i+1)%len(poly)]
		ax, ay, bx, by := a.X*k, a.Y*k, b.X*k, b.Y*k
		if (ay > p.Y) != (by > p.Y) && p.X < ax+(p.Y-ay)/(by-ay)*(bx-ax) {
			in = !in
		}
	}
	return in
}

// EdgeDistance returns how far p is from the edge of the polygon scaled by k,
// approximated along the ray from the origin through p. It is negative
// outside. Placement uses it to keep footprints off the cliffs.
func EdgeDistance(poly []Vec2, k float64, p Vec2) float64 {
	a := math.Atan2(p.Y, p.X)
	n := len(poly)
	// The outline is sampled at regular angles: interpolate its radius.
	f := a / (2 * math.Pi) * float64(n)
	if f < 0 {
		f += float64(n)
	}
	i := int(f) % n
	t := f - math.Floor(f)
	r := (poly[i].Len()*(1-t) + poly[(i+1)%n].Len()*t) * k
	return r - p.Len()
}

// RiverWidth is how wide a stream is at its mouth, in island units; it
// narrows to its spring.
const RiverWidth = 3.6

// Rivers returns the streams of an island of a size: each a path from its
// spring inland to its mouth on the shore, where it falls over the cliff.
// The streams run towards the viewer, so that their falls show. Their
// course is fixed in island units: when the island grows, they only reach
// further to the new shore, and never run under what was built.
func Rivers(seed uint64, size float64) [][]Vec2 {
	outline := Outline(seed)
	var front []int
	for i, p := range outline {
		if p.Y > 0.35 {
			front = append(front, i)
		}
	}
	if len(front) == 0 {
		return nil
	}
	r := rand.New(rand.NewPCG(seed, 0x71fe2))
	n := 1 + int(seed%2)
	out := make([][]Vec2, 0, n)
	for i := range n {
		k := front[(int(seed>>3)+i*len(front)/2)%len(front)]
		a := math.Atan2(outline[k].Y, outline[k].X)
		phase, sway := r.Float64()*6, 1.5+r.Float64()*1.5
		var path []Vec2
		for d := 13.0; d < 1000; d += riverStep {
			lat := sway * math.Sin(d*0.28+phase)
			p := Vec2{d*math.Cos(a) - lat*math.Sin(a), d*math.Sin(a) + lat*math.Cos(a)}
			if e := EdgeDistance(outline, size, p); e < 1 {
				// The mouth, on the shore along the same ray.
				path = append(path, p.Scale((p.Len()+e)/p.Len()))
				break
			}
			path = append(path, p)
		}
		out = append(out, path)
	}
	return out
}

// SpringRadius is the size of the pool a stream springs from.
const SpringRadius = 2.2

// RiverDistance returns how far p is from the nearest stream's bank, or its
// spring's: zero or less on the water.
func RiverDistance(rivers [][]Vec2, p Vec2) float64 {
	best := math.Inf(1)
	for _, path := range rivers {
		if len(path) > 0 {
			best = math.Min(best, p.Dist(path[0])-SpringRadius)
		}
		for i := 1; i < len(path); i++ {
			d, t := segment(path[i-1], path[i], p)
			w := RiverHalfWidth(path, i-1, t)
			best = math.Min(best, d-w)
		}
	}
	return best
}

// RiverHalfWidth is the half width of a stream along its path, at segment i
// and fraction t of it: narrow at the spring, wider downstream. It depends
// on the distance from the spring only, so that it stays put as the island
// grows.
func RiverHalfWidth(path []Vec2, i int, t float64) float64 {
	k := math.Min((float64(i)+t)*riverStep/20, 1)
	return RiverWidth / 2 * (0.5 + 0.5*k)
}

// riverStep is the length of a segment of a stream's path.
const riverStep = 2.0

// segment returns the distance from p to the segment ab, and where along it
// the nearest point lies, from 0 to 1.
func segment(a, b, p Vec2) (float64, float64) {
	dx, dy := b.X-a.X, b.Y-a.Y
	l := dx*dx + dy*dy
	t := 0.0
	if l > 0 {
		t = math.Min(math.Max(((p.X-a.X)*dx+(p.Y-a.Y)*dy)/l, 0), 1)
	}
	return p.Dist(Vec2{a.X + t*dx, a.Y + t*dy}), t
}

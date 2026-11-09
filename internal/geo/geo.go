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

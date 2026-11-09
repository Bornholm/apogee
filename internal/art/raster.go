package art

import (
	"math"
	"slices"
)

// pt is a projected 2D point in virtual-pixel coordinates.
type pt struct{ X, Y float64 }

// fillPolygon calls plot for every pixel whose centre lies inside the polygon,
// using the even-odd rule and no anti-aliasing. A pixel (x, y) covers the
// square [x, x+1) × [y, y+1), so its centre is (x+0.5, y+0.5). Edges follow a
// half-open convention, which guarantees that two polygons sharing an edge
// never both cover the same pixel and never leave a gap between them.
func fillPolygon(poly []pt, plot func(x, y int)) {
	if len(poly) < 3 {
		return
	}
	minY, maxY := math.Inf(1), math.Inf(-1)
	for _, p := range poly {
		minY = math.Min(minY, p.Y)
		maxY = math.Max(maxY, p.Y)
	}
	xs := make([]float64, 0, 8)
	for y := int(math.Floor(minY)); y <= int(math.Ceil(maxY)); y++ {
		yc := float64(y) + 0.5
		xs = xs[:0]
		for i, a := range poly {
			b := poly[(i+1)%len(poly)]
			if (a.Y <= yc && b.Y > yc) || (b.Y <= yc && a.Y > yc) {
				t := (yc - a.Y) / (b.Y - a.Y)
				xs = append(xs, a.X+t*(b.X-a.X))
			}
		}
		slices.Sort(xs)
		for i := 0; i+1 < len(xs); i += 2 {
			x0 := int(math.Ceil(xs[i] - 0.5))
			x1 := int(math.Ceil(xs[i+1] - 0.5))
			for x := x0; x < x1; x++ {
				plot(x, y)
			}
		}
	}
}

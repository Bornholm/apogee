package art

import (
	"image"
	"image/color"
	"math"

	"apogee/internal/geo"
)

// Water tones: a light current along the middle, a deeper blue, a dark bank.
var (
	waterLit  = hex(0x9edcf6)
	waterMid  = hex(0x5ea8e0)
	waterBank = hex(0x3d76b0)
)

// PaintRivers paints the streams on the top of an island sprite, anchored
// on the island centre: each pixel of the ground is traced back to the
// ground plane and painted when it lies on the water.
func PaintRivers(cam Camera, sp Sprite, rivers [][]geo.Vec2) {
	if len(rivers) == 0 || sp.Ground == nil {
		return
	}
	s := math.Sin(cam.Pitch)
	// Only the pixels around the streams are traced back.
	area := image.Rectangle{}
	for _, path := range rivers {
		for _, p := range path {
			x, y := sp.Anchor.X+int(math.Floor(p.X)), sp.Anchor.Y+int(math.Floor(p.Y*s))
			r := int(math.Ceil(math.Max(geo.SpringRadius, geo.RiverWidth))) + 1
			area = area.Union(image.Rect(x-r, y-r, x+r+1, y+r+1))
		}
	}
	b := area.Intersect(sp.Img.Rect)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if !image.Pt(x, y).In(sp.Ground.Rect) || sp.Ground.AlphaAt(x, y).A == 0 {
				continue
			}
			p := geo.Vec2{X: float64(x-sp.Anchor.X) + 0.5, Y: (float64(y-sp.Anchor.Y) + 0.5) / s}
			if c, ok := waterAt(rivers, p); ok {
				sp.Img.SetRGBA(x, y, c)
			}
		}
	}
}

// waterAt returns the colour of the water at a ground point, if any.
func waterAt(rivers [][]geo.Vec2, p geo.Vec2) (color.RGBA, bool) {
	d := geo.RiverDistance(rivers, p)
	switch {
	case d > 0:
		return color.RGBA{}, false
	case d > -0.5:
		return waterBank, true
	case d > -0.9:
		return waterMid, true
	}
	return waterLit, true
}

// PaintPools paints round pools of warm water on the top of an island
// sprite: hot springs, with their pale steaming rim.
func PaintPools(cam Camera, sp Sprite, centres []geo.Vec2, radius float64) {
	if len(centres) == 0 || sp.Ground == nil {
		return
	}
	s := math.Sin(cam.Pitch)
	b := sp.Img.Rect
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if !image.Pt(x, y).In(sp.Ground.Rect) || sp.Ground.AlphaAt(x, y).A == 0 {
				continue
			}
			p := geo.Vec2{X: float64(x-sp.Anchor.X) + 0.5, Y: (float64(y-sp.Anchor.Y) + 0.5) / s}
			for _, c := range centres {
				switch d := p.Dist(c); {
				case d > radius:
				case d > radius-0.8:
					sp.Img.SetRGBA(x, y, hex(0xd8e4e0)) // a mineral rim
				case d > radius*0.45:
					sp.Img.SetRGBA(x, y, hex(0x6cc6c8))
				default:
					sp.Img.SetRGBA(x, y, hex(0x9fe0dc))
				}
			}
		}
	}
}

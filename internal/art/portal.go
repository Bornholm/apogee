package art

import (
	"image"
	"image/color"
	"math"
)

// VoidRamp holds the tones of the portal, from its luminous rim to the abyss.
var VoidRamp = []color.RGBA{
	hex(0xb9a6f0), hex(0x8a72d8), hex(0x5d49b0), hex(0x3b2c80), hex(0x221a52), hex(0x120d2e),
}

// PortalFrame draws one frame of the whirlpool at the centre of the sky
// (GDD 12.6). The vortex is an ellipse flattened by the camera pitch; spiral
// arms wind towards the centre, and their phase t ∈ [0, 1) turns them. Tones
// are quantized to VoidRamp, with no anti-aliasing.
func PortalFrame(cam Camera, radius float64, arms int, t float64) Sprite {
	squash := math.Sin(cam.Pitch)
	w := int(math.Ceil(radius))*2 + 1
	h := int(math.Ceil(radius*squash))*2 + 1
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	cx, cy := float64(w)/2, float64(h)/2
	for y := range h {
		for x := range w {
			dx := (float64(x) + 0.5 - cx) / radius
			dy := (float64(y) + 0.5 - cy) / (radius * squash)
			r := math.Hypot(dx, dy)
			if r > 1 {
				continue
			}
			theta := math.Atan2(dy, dx)
			// Logarithmic spiral: arms wind faster near the centre. The
			// whirlpool turns the way its arms trail, like water down a
			// drain: their outer ends lag behind.
			v := float64(arms)*theta/(2*math.Pi) + 2.2*math.Log(r+0.02) - t
			band := v - math.Floor(v) // position across an arm, in [0, 1)
			// Darker towards the centre, lighter on the arm crests.
			depth := r * float64(len(VoidRamp)-1)
			crest := 1 - math.Abs(band-0.5)*2
			idx := len(VoidRamp) - 1 - int(depth*0.75+crest*1.6)
			idx = min(max(idx, 0), len(VoidRamp)-1)
			if r < 0.12 {
				idx = len(VoidRamp) - 1
			}
			img.SetRGBA(x, y, VoidRamp[idx])
		}
	}
	return Sprite{Img: img, Anchor: image.Pt(w/2, h/2), Ground: image.NewAlpha(img.Rect)}
}

// Flatten composes an island scene into a single sprite, anchored on the
// island centre. The sky view uses it to draw whole islands at once. The
// canvas covers the island and every object, whichever side they overflow.
func (s IslandScene) Flatten(cam Camera) Sprite {
	return s.flatten(cam, s.Compose)
}

// FlattenGround composes the island and the shadows only, anchored on the
// island centre, for a client that draws the objects itself.
func (s IslandScene) FlattenGround(cam Camera) Sprite {
	return s.flatten(cam, s.ComposeGround)
}

func (s IslandScene) flatten(cam Camera, compose func(Camera, *image.RGBA, int, int)) Sprite {
	bounds := s.Island.Img.Rect.Sub(s.Island.Anchor)
	for _, o := range s.Objects {
		x, y := cam.Project(o.Pos)
		at := image.Pt(int(math.Round(x)), int(math.Round(y)))
		bounds = bounds.Union(o.Sprite.Img.Rect.Sub(o.Sprite.Anchor).Add(at))
	}
	img := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	cx, cy := -bounds.Min.X, -bounds.Min.Y
	compose(cam, img, cx, cy)
	return trim(Sprite{Img: img, Anchor: image.Pt(cx, cy)})
}

// trim crops the transparent margins of a sprite, keeping its anchor.
func trim(sp Sprite) Sprite {
	b := sp.Img.Rect
	minX, minY, maxX, maxY := b.Max.X, b.Max.Y, b.Min.X-1, b.Min.Y-1
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if sp.Img.RGBAAt(x, y).A != 0 {
				minX, minY = min(minX, x), min(minY, y)
				maxX, maxY = max(maxX, x), max(maxY, y)
			}
		}
	}
	if maxX < minX {
		return sp
	}
	r := image.Rect(minX, minY, maxX+1, maxY+1)
	img := image.NewRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	for y := r.Min.Y; y < r.Max.Y; y++ {
		copy(img.Pix[(y-r.Min.Y)*img.Stride:], sp.Img.Pix[y*sp.Img.Stride+4*r.Min.X:y*sp.Img.Stride+4*r.Max.X])
	}
	return Sprite{Img: img, Anchor: sp.Anchor.Sub(r.Min)}
}

// Haze blends a sprite towards c by amount a ∈ [0, 1], for the atmospheric
// depth of distant islands (GDD 12.6). It returns a new sprite.
func Haze(sp Sprite, c color.RGBA, a float64) Sprite {
	img := image.NewRGBA(sp.Img.Rect)
	copy(img.Pix, sp.Img.Pix)
	k := uint8(math.Round(min(max(a, 0), 1) * 255))
	for i := 0; i < len(img.Pix); i += 4 {
		if img.Pix[i+3] == 0 {
			continue
		}
		img.Pix[i] = mix(img.Pix[i], c.R, k)
		img.Pix[i+1] = mix(img.Pix[i+1], c.G, k)
		img.Pix[i+2] = mix(img.Pix[i+2], c.B, k)
	}
	out := sp
	out.Img = img
	return out
}

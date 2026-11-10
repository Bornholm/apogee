package art

import (
	"image"
	"image/color"
	"math"
	"sort"
)

// Placed is a sprite standing on an island at a ground position, in world
// coordinates relative to the island centre.
type Placed struct {
	Sprite Sprite
	Pos    Vec3
	// Sways is set on what bends in the wind: the pines.
	Sways bool
}

// IslandScene is everything needed to draw one island and what stands on it.
type IslandScene struct {
	Island  Sprite
	Objects []Placed
}

// Compose draws the island scene on dst, with the island centre at (cx, cy).
// It follows GDD 12.3: one shadow layer of uniform opacity, clipped to the
// island top, then objects sorted by the y of their base.
func (s IslandScene) Compose(cam Camera, dst *image.RGBA, cx, cy int) {
	s.ComposeGround(cam, dst, cx, cy)
	objs := s.Sorted()
	for _, o := range objs {
		x, y := screenOf(cam, o.Pos, cx, cy)
		Blit(dst, o.Sprite.Img, x-o.Sprite.Anchor.X, y-o.Sprite.Anchor.Y)
	}
}

// Sorted returns the objects far to near, the order they must be drawn in.
func (s IslandScene) Sorted() []Placed {
	objs := append([]Placed(nil), s.Objects...)
	sort.SliceStable(objs, func(i, j int) bool {
		if objs[i].Pos.Y != objs[j].Pos.Y {
			return objs[i].Pos.Y < objs[j].Pos.Y
		}
		return objs[i].Pos.X < objs[j].Pos.X
	})
	return objs
}

// ComposeGround draws the island and the shadows of its objects, but not
// the objects: a client that animates people draws the objects itself,
// sorted together with them.
func (s IslandScene) ComposeGround(cam Camera, dst *image.RGBA, cx, cy int) {
	Blit(dst, s.Island.Img, cx-s.Island.Anchor.X, cy-s.Island.Anchor.Y)

	shadow := image.NewAlpha(dst.Rect)
	for _, o := range s.Objects {
		if o.Sprite.Shadow == nil {
			continue
		}
		x, y := screenOf(cam, o.Pos, cx, cy)
		orMask(shadow, o.Sprite.Shadow, x-o.Sprite.ShadowAnchor.X, y-o.Sprite.ShadowAnchor.Y)
	}
	ground := image.NewAlpha(dst.Rect)
	orMask(ground, s.Island.Ground, cx-s.Island.Anchor.X, cy-s.Island.Anchor.Y)
	b := dst.Rect
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if shadow.Pix[shadow.PixOffset(x, y)] == 0 || ground.Pix[ground.PixOffset(x, y)] == 0 {
				continue
			}
			o := dst.PixOffset(x, y)
			for c, v := range [3]uint8{ShadowTint.R, ShadowTint.G, ShadowTint.B} {
				dst.Pix[o+c] = mix(dst.Pix[o+c], v, ShadowAlpha)
			}
		}
	}
}

// screenOf projects a world position and snaps it to the virtual pixel grid,
// so sprites never shimmer (GDD 12.3).
func screenOf(cam Camera, p Vec3, cx, cy int) (int, int) {
	x, y := cam.Project(p)
	return cx + int(math.Round(x)), cy + int(math.Round(y))
}

// Blit copies the opaque pixels of src onto dst at (x, y).
func Blit(dst, src *image.RGBA, x, y int) {
	b := src.Rect
	for sy := b.Min.Y; sy < b.Max.Y; sy++ {
		for sx := b.Min.X; sx < b.Max.X; sx++ {
			c := src.RGBAAt(sx, sy)
			if c.A == 0 {
				continue
			}
			dx, dy := x+sx-b.Min.X, y+sy-b.Min.Y
			if image.Pt(dx, dy).In(dst.Rect) {
				dst.SetRGBA(dx, dy, c)
			}
		}
	}
}

// orMask sets on dst every pixel set in src, placed at (x, y).
func orMask(dst, src *image.Alpha, x, y int) {
	b := src.Rect
	for sy := b.Min.Y; sy < b.Max.Y; sy++ {
		for sx := b.Min.X; sx < b.Max.X; sx++ {
			if src.AlphaAt(sx, sy).A == 0 {
				continue
			}
			dx, dy := x+sx-b.Min.X, y+sy-b.Min.Y
			if image.Pt(dx, dy).In(dst.Rect) {
				dst.SetAlpha(dx, dy, color.Alpha{0xff})
			}
		}
	}
}

func mix(a, b, alpha uint8) uint8 {
	return uint8((int(a)*(255-int(alpha)) + int(b)*int(alpha) + 127) / 255)
}

// FillSky paints the vertical sky gradient on dst.
func FillSky(dst *image.RGBA) {
	h := dst.Rect.Dy()
	for y := range h {
		t := float64(y) / float64(max(h-1, 1))
		c := color.RGBA{
			lerp8(SkyTop.R, SkyBottom.R, t),
			lerp8(SkyTop.G, SkyBottom.G, t),
			lerp8(SkyTop.B, SkyBottom.B, t),
			0xff,
		}
		for x := dst.Rect.Min.X; x < dst.Rect.Max.X; x++ {
			dst.SetRGBA(x, dst.Rect.Min.Y+y, c)
		}
	}
}

func lerp8(a, b uint8, t float64) uint8 {
	return uint8(math.Round(float64(a) + (float64(b)-float64(a))*t))
}

// Upscale returns src enlarged by an integer factor with nearest-neighbour
// sampling, which keeps pixels crisp (GDD 12.3). A factor below 1 is treated
// as 1.
func Upscale(src *image.RGBA, k int) *image.RGBA {
	k = max(k, 1)
	b := src.Rect
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx()*k, b.Dy()*k))
	for y := range dst.Rect.Dy() {
		for x := range dst.Rect.Dx() {
			dst.SetRGBA(x, y, src.RGBAAt(b.Min.X+x/k, b.Min.Y+y/k))
		}
	}
	return dst
}

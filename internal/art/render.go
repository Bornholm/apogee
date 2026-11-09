package art

import (
	"image"
	"image/color"
	"math"
	"sort"
)

// Camera is the oblique 3/4 orthographic view of the game (GDD 12.1).
type Camera struct {
	// Pitch is the angle between the view direction and the horizontal plane,
	// in radians: π/2 looks straight down, 0 looks from the side.
	Pitch float64
	// Light is the unit direction pointing towards the single fixed light.
	Light Vec3
}

// DefaultCamera looks down at about 50° with the light coming from the upper
// left of the screen, slightly from behind the scene.
var DefaultCamera = Camera{
	Pitch: 50 * math.Pi / 180,
	Light: V(-0.65, -0.2, 0.73).Norm(),
}

// Project maps a world point to virtual-pixel screen coordinates, with Y
// growing downwards. The world origin projects to (0, 0).
func (c Camera) Project(p Vec3) (x, y float64) {
	s, co := math.Sincos(c.Pitch)
	return p.X, p.Y*s - p.Z*co
}

// toViewer is the unit vector pointing from the scene towards the camera.
func (c Camera) toViewer() Vec3 {
	s, co := math.Sincos(c.Pitch)
	return V(0, co, s)
}

// depth grows towards the viewer: faces with a larger depth are drawn later.
func (c Camera) depth(p Vec3) float64 { return p.Dot(c.toViewer()) }

// ToneOf picks the tone of a face from its normal. The thresholds are tuned so
// that tops and west faces are lit, south faces are mid and east faces dark.
func (c Camera) ToneOf(normal Vec3) Tone {
	d := normal.Dot(c.Light)
	switch {
	case d > 0.6:
		return Lit
	case d > -0.3:
		return Mid
	default:
		return Dark
	}
}

// groundShadow projects p onto the ground plane z = 0 along the light.
func (c Camera) groundShadow(p Vec3) Vec3 {
	if p.Z <= 0 || c.Light.Z <= 0 {
		return V(p.X, p.Y, 0)
	}
	k := p.Z / c.Light.Z
	return V(p.X-c.Light.X*k, p.Y-c.Light.Y*k, 0)
}

// Sprite is a rendered mesh. Its anchor is the screen position of the world
// origin inside the images, which lets sprites be placed by their ground point.
type Sprite struct {
	Img    *image.RGBA
	Anchor image.Point
	// Shadow is the cast shadow silhouette on the ground plane, with its own
	// anchor. It is nil when the mesh has nothing above the ground.
	Shadow       *image.Alpha
	ShadowAnchor image.Point
	// Ground marks the pixels of horizontal top faces, where cast shadows of
	// other objects may fall. It shares Img's bounds and anchor.
	Ground *image.Alpha
}

// Render rasterizes the mesh with the camera: back faces are culled, the
// remaining faces are painted far to near, each with one flat tone.
func (c Camera) Render(m Mesh) Sprite {
	type drawn struct {
		poly  []pt
		col   color.RGBA
		depth float64
		top   bool
	}
	view := c.toViewer()
	var faces []drawn
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, f := range m.Faces {
		n := f.Normal()
		if n.Dot(view) <= 1e-9 {
			continue
		}
		poly := make([]pt, len(f.V))
		for i, v := range f.V {
			x, y := c.Project(v)
			poly[i] = pt{x, y}
			minX, minY = math.Min(minX, x), math.Min(minY, y)
			maxX, maxY = math.Max(maxX, x), math.Max(maxY, y)
		}
		faces = append(faces, drawn{
			poly:  poly,
			col:   Palette[f.Mat][c.ToneOf(n)],
			depth: c.depth(f.Centroid()),
			top:   n.Z > 0.999,
		})
	}
	sort.SliceStable(faces, func(i, j int) bool { return faces[i].depth < faces[j].depth })

	var sp Sprite
	if len(faces) == 0 {
		sp.Img = image.NewRGBA(image.Rect(0, 0, 0, 0))
		sp.Ground = image.NewAlpha(sp.Img.Rect)
	} else {
		x0, y0 := int(math.Floor(minX)), int(math.Floor(minY))
		x1, y1 := int(math.Ceil(maxX))+1, int(math.Ceil(maxY))+1
		sp.Img = image.NewRGBA(image.Rect(0, 0, x1-x0, y1-y0))
		sp.Ground = image.NewAlpha(sp.Img.Rect)
		sp.Anchor = image.Pt(-x0, -y0)
		for _, f := range faces {
			for i := range f.poly {
				f.poly[i].X -= float64(x0)
				f.poly[i].Y -= float64(y0)
			}
			fillPolygon(f.poly, func(x, y int) {
				sp.Img.SetRGBA(x, y, f.col)
				if f.top {
					sp.Ground.SetAlpha(x, y, color.Alpha{0xff})
				} else {
					sp.Ground.SetAlpha(x, y, color.Alpha{})
				}
			})
		}
	}
	sp.Shadow, sp.ShadowAnchor = c.renderShadow(m)
	return sp
}

// renderShadow rasterizes the silhouette cast on the ground by every face that
// rises above it. Overlapping faces simply merge into one opaque mask.
func (c Camera) renderShadow(m Mesh) (*image.Alpha, image.Point) {
	var polys [][]pt
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, f := range m.Faces {
		above := false
		for _, v := range f.V {
			if v.Z > 1e-6 {
				above = true
				break
			}
		}
		if !above {
			continue
		}
		poly := make([]pt, len(f.V))
		for i, v := range f.V {
			x, y := c.Project(c.groundShadow(v))
			poly[i] = pt{x, y}
			minX, minY = math.Min(minX, x), math.Min(minY, y)
			maxX, maxY = math.Max(maxX, x), math.Max(maxY, y)
		}
		polys = append(polys, poly)
	}
	if len(polys) == 0 {
		return nil, image.Point{}
	}
	x0, y0 := int(math.Floor(minX)), int(math.Floor(minY))
	x1, y1 := int(math.Ceil(maxX))+1, int(math.Ceil(maxY))+1
	mask := image.NewAlpha(image.Rect(0, 0, x1-x0, y1-y0))
	for _, poly := range polys {
		for i := range poly {
			poly[i].X -= float64(x0)
			poly[i].Y -= float64(y0)
		}
		fillPolygon(poly, func(x, y int) { mask.SetAlpha(x, y, color.Alpha{0xff}) })
	}
	return mask, image.Pt(-x0, -y0)
}

// Package client is Apogée's Ebitengine front-end. It only displays the sky
// and forwards the player's intentions: it never simulates (GDD 10.3).
package client

import (
	"image"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"apogee/internal/art"
)

// Zoom levels are integer upscale factors, one per view (GDD 3.4 and 12.3).
var zoomLevels = []int{2, 3, 4, 6}

// transitionTicks is the length of the animated transition between levels.
const transitionTicks = 12

// Game draws a pre-rendered island over the sky with drifting clouds.
type Game struct {
	world  *ebiten.Image // the island scene, in virtual pixels
	sky    *ebiten.Image // a 1×256 vertical gradient, stretched to the screen
	clouds []cloud

	level     int     // index in zoomLevels
	scale     float64 // displayed scale, animated towards the level
	fromScale float64
	anim      int // remaining transition ticks

	tick  int
	dirty bool // whether the screen must be redrawn

	outW, outH int

	// OnFrame, when set, is called after each redraw with the screen image.
	// The screenshot mode uses it.
	OnFrame func(screen *ebiten.Image)
}

type cloud struct {
	img      *ebiten.Image
	x, y     float64 // virtual pixels, relative to the world image centre
	speed    float64 // virtual pixels per tick
	lastDraw int     // x position at last redraw, to detect pixel moves
}

// NewGame renders the demo island once and prepares the clouds.
func NewGame(seed uint64) *Game {
	cam := art.DefaultCamera
	const w, h = 320, 240
	canvas := image.NewRGBA(image.Rect(0, 0, w, h))
	scene := art.DemoIsland(cam, seed, 60)
	scene.Compose(cam, canvas, w/2, h*2/5)

	grad := image.NewRGBA(image.Rect(0, 0, 1, 256))
	art.FillSky(grad)
	g := &Game{
		world: ebiten.NewImageFromImage(canvas),
		sky:   ebiten.NewImageFromImage(grad),
		level: 2, dirty: true,
	}
	g.scale = float64(zoomLevels[g.level])

	r := art.Rng(seed + 1000)
	for i := range 8 {
		sp := cam.Render(art.CloudPuff(seed*101 + uint64(i)))
		g.clouds = append(g.clouds, cloud{
			img:   ebiten.NewImageFromImage(sp.Img),
			x:     float64(r.IntN(2*w) - w),
			y:     float64(r.IntN(h) - h/4),
			speed: 0.02 + r.Float64()*0.03,
		})
	}
	return g
}

// Update advances animations at a fixed rate and decides whether a redraw is
// needed. Nothing is drawn when nothing visible changed.
func (g *Game) Update() error {
	g.tick++
	if _, wy := ebiten.Wheel(); wy != 0 {
		g.zoomBy(int(math.Copysign(1, wy)))
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEqual) || inpututil.IsKeyJustPressed(ebiten.KeyKPAdd) {
		g.zoomBy(1)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyMinus) || inpututil.IsKeyJustPressed(ebiten.KeyKPSubtract) {
		g.zoomBy(-1)
	}

	if g.anim > 0 {
		g.anim--
		t := 1 - float64(g.anim)/transitionTicks
		t = t * t * (3 - 2*t) // smoothstep
		target := float64(zoomLevels[g.level])
		g.scale = g.fromScale + (target-g.fromScale)*t
		g.dirty = true
	}

	for i := range g.clouds {
		c := &g.clouds[i]
		c.x += c.speed
		if c.x > 360 {
			c.x = -360
		}
		if int(c.x) != c.lastDraw {
			g.dirty = true
		}
	}
	return nil
}

func (g *Game) zoomBy(d int) {
	n := min(max(g.level+d, 0), len(zoomLevels)-1)
	if n == g.level {
		return
	}
	g.level = n
	g.fromScale = g.scale
	g.anim = transitionTicks
}

// Draw redraws only when something changed: the screen is not cleared every
// frame (see main), so skipping Draw keeps the previous image on screen.
func (g *Game) Draw(screen *ebiten.Image) {
	if !g.dirty {
		return
	}
	g.dirty = false
	var sky ebiten.DrawImageOptions
	sky.GeoM.Scale(float64(g.outW), float64(g.outH)/256)
	sky.Filter = ebiten.FilterLinear
	screen.DrawImage(g.sky, &sky)

	cx, cy := float64(g.outW)/2, float64(g.outH)/2
	wb := g.world.Bounds()
	// Snap the world origin to whole screen pixels for crisp output.
	ox := math.Round(cx - float64(wb.Dx())*g.scale/2)
	oy := math.Round(cy - float64(wb.Dy())*g.scale/2)

	// Clouds drift below the islands, so they are drawn first.
	for i := range g.clouds {
		c := &g.clouds[i]
		c.lastDraw = int(c.x)
		var co ebiten.DrawImageOptions
		co.GeoM.Translate(math.Floor(c.x)+float64(wb.Dx())/2, c.y+float64(wb.Dy())*0.75)
		co.GeoM.Scale(g.scale, g.scale)
		co.GeoM.Translate(ox, oy)
		screen.DrawImage(c.img, &co)
	}

	var op ebiten.DrawImageOptions
	op.GeoM.Scale(g.scale, g.scale)
	op.GeoM.Translate(ox, oy)
	screen.DrawImage(g.world, &op)

	if g.OnFrame != nil {
		g.OnFrame(screen)
	}
}

// Layout uses the window size in device pixels, so that the integer scale is
// applied to real pixels.
func (g *Game) Layout(outW, outH int) (int, int) {
	if outW != g.outW || outH != g.outH {
		g.outW, g.outH = outW, outH
		g.dirty = true
	}
	return outW, outH
}

// LayoutF keeps Ebitengine from rescaling the screen on high-DPI monitors.
func (g *Game) LayoutF(outW, outH float64) (float64, float64) {
	w, h := g.Layout(int(outW), int(outH))
	return float64(w), float64(h)
}

// Package client is Apogée's Ebitengine front-end. It only displays the sky
// and forwards the player's intentions: it never simulates (GDD 10.3).
package client

import (
	"fmt"
	"image"
	"math"
	"sort"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"apogee/internal/art"
	"apogee/internal/sim"
)

// Zoom levels are integer upscale factors, one per view (GDD 3.4 and 12.3):
// the first shows the whole sky, the others the player's island.
var zoomLevels = []int{2, 3, 4, 6}

// transitionTicks is the length of the animated transition between levels.
const transitionTicks = 12

// Sky view geometry, in virtual pixels at the sky zoom level.
const (
	rimPixels    = 240.0 // orbit radius of the rim
	portalPixels = 70.0
	portalFrames = 24
	portalFPS    = 6
)

// Game draws the sky, or the player's island, from the server's states.
type Game struct {
	src *Source
	cam art.Camera

	sky    *ebiten.Image // a 1×256 vertical gradient, stretched to the screen
	portal []*ebiten.Image
	clouds []cloud

	islandSprites map[sim.IslandID]sprite // sky view miniatures
	beacon        sprite                  // marks the player's island
	home          *ebiten.Image           // the player's island at full size

	level     int     // index in zoomLevels
	scale     float64 // displayed scale, animated towards the level
	fromScale float64
	anim      int // remaining transition ticks

	frame     int // current portal frame
	positions map[sim.IslandID]image.Point
	phase     string
	phaseLeft int // seconds, as last displayed
	dirty     bool

	// pan scrolls the view with the arrow keys when the window is too small
	// to show everything, in virtual pixels.
	panX, panY float64

	outW, outH int

	// OnFrame, when set, is called after each redraw with the screen image.
	// The screenshot mode uses it.
	OnFrame func(screen *ebiten.Image)
}

type sprite struct {
	img    *ebiten.Image
	anchor image.Point
}

type cloud struct {
	img      *ebiten.Image
	x, y     float64 // virtual pixels, relative to the screen centre
	speed    float64 // virtual pixels per tick
	lastDraw int     // x position at last redraw, to detect pixel moves
}

// NewGame prepares the static art; islands are rendered when they appear.
func NewGame(src *Source, seed uint64) *Game {
	cam := art.DefaultCamera
	grad := image.NewRGBA(image.Rect(0, 0, 1, 256))
	art.FillSky(grad)
	g := &Game{
		src:           src,
		cam:           cam,
		sky:           ebiten.NewImageFromImage(grad),
		islandSprites: map[sim.IslandID]sprite{},
		positions:     map[sim.IslandID]image.Point{},
		dirty:         true,
	}
	g.scale = float64(zoomLevels[g.level])
	b := cam.Render(art.Beacon())
	g.beacon = sprite{ebiten.NewImageFromImage(b.Img), b.Anchor}
	for i := range portalFrames {
		sp := art.PortalFrame(cam, portalPixels, 3, float64(i)/portalFrames)
		g.portal = append(g.portal, ebiten.NewImageFromImage(sp.Img))
	}
	r := art.Rng(seed + 1000)
	for i := range 12 {
		sp := cam.Render(art.CloudPuff(seed*101 + uint64(i)))
		g.clouds = append(g.clouds, cloud{
			img:   ebiten.NewImageFromImage(sp.Img),
			x:     float64(r.IntN(640) - 320),
			y:     float64(r.IntN(220) - 40),
			speed: 0.02 + r.Float64()*0.03,
		})
	}
	return g
}

// islandRadius gives each island a size from its seed, in world units.
func islandRadius(seed uint64) float64 { return 14 + float64(seed%7) }

func (g *Game) miniature(is sim.Island) sprite {
	if sp, ok := g.islandSprites[is.ID]; ok {
		return sp
	}
	flat := art.DemoIsland(g.cam, is.Seed, islandRadius(is.Seed)).Flatten(g.cam)
	sp := sprite{ebiten.NewImageFromImage(flat.Img), flat.Anchor}
	g.islandSprites[is.ID] = sp
	return sp
}

func (g *Game) homeIsland(islands []sim.Island) *ebiten.Image {
	if g.home != nil {
		return g.home
	}
	for _, is := range islands {
		if is.ID == g.src.Me {
			const w, h = 320, 240
			canvas := image.NewRGBA(image.Rect(0, 0, w, h))
			scene := art.DemoIsland(g.cam, is.Seed, 60)
			scene.Compose(g.cam, canvas, w/2, h*2/5)
			g.home = ebiten.NewImageFromImage(canvas)
		}
	}
	return g.home
}

// Update advances animations at a fixed rate and decides whether a redraw is
// needed. Nothing is drawn when nothing visible changed.
func (g *Game) Update() error {
	if err := g.src.Err(); err != nil {
		return fmt.Errorf("connection to the sky lost: %w", err)
	}
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

	now := time.Now()
	if phase, left, ok := g.src.Phase(now); ok {
		if phase != g.phase || int(left.Seconds()) != g.phaseLeft {
			g.phase, g.phaseLeft = phase, int(left.Seconds())
			g.dirty = true
		}
	}
	if g.level == 0 {
		f := int(now.UnixMilli()*portalFPS/1000) % portalFrames
		if f != g.frame {
			g.frame = f
			g.dirty = true
		}
		// Orbits are slow: redraw only when an island moves by a pixel.
		for _, is := range g.src.Islands(now) {
			p := g.orbitPixel(is)
			if g.positions[is.ID] != p {
				g.positions[is.ID] = p
				g.dirty = true
			}
		}
	}
	g.updatePan()
	return nil
}

// panSpeed is the scrolling speed in screen pixels per tick.
const panSpeed = 8

// updatePan scrolls with the arrow keys, within the bounds of the scene.
func (g *Game) updatePan() {
	dx, dy := 0.0, 0.0
	if ebiten.IsKeyPressed(ebiten.KeyArrowLeft) {
		dx++
	}
	if ebiten.IsKeyPressed(ebiten.KeyArrowRight) {
		dx--
	}
	if ebiten.IsKeyPressed(ebiten.KeyArrowUp) {
		dy++
	}
	if ebiten.IsKeyPressed(ebiten.KeyArrowDown) {
		dy--
	}
	x, y := g.panX+dx*panSpeed/g.scale, g.panY+dy*panSpeed/g.scale
	x, y = g.clampPan(x, y)
	if x != g.panX || y != g.panY {
		g.panX, g.panY = x, y
		g.dirty = true
	}
}

// sceneBounds returns the extent of the current scene around the screen
// centre, in virtual pixels.
func (g *Game) sceneBounds() image.Rectangle {
	if g.level == 0 {
		// The rim orbit plus room for the islands rising above it and their
		// rocky undersides below.
		w := int(rimPixels) + 50
		h := int(rimPixels * math.Sin(g.cam.Pitch))
		return image.Rect(-w, -h-60, w, h+70)
	}
	if g.home == nil {
		return image.Rectangle{}
	}
	b := g.home.Bounds()
	return image.Rect(-b.Dx()/2, -b.Dy()*2/5, b.Dx()-b.Dx()/2, b.Dy()-b.Dy()*2/5)
}

// clampPan keeps the scene edges from leaving the screen: on an axis where
// the scene fits, it stays centred.
func (g *Game) clampPan(x, y float64) (float64, float64) {
	b := g.sceneBounds()
	if b.Empty() || g.outW == 0 {
		return 0, 0
	}
	halfW, halfH := float64(g.outW)/2/g.scale, float64(g.outH)/2/g.scale
	clamp := func(v float64, lo, hi int, half float64) float64 {
		// Scrolling by +v moves the scene right: its left edge lo+v must stay
		// at or left of -half, its right edge hi+v at or right of +half.
		minV, maxV := half-float64(hi), -half-float64(lo)
		if minV > maxV {
			return 0
		}
		return math.Min(math.Max(v, minV), maxV)
	}
	return clamp(x, b.Min.X, b.Max.X, halfW), clamp(y, b.Min.Y, b.Max.Y, halfH)
}

func (g *Game) orbitPixel(is sim.Island) image.Point {
	d := is.Radius / sim.RimRadius * rimPixels
	x, y := g.cam.Project(art.V(d*math.Cos(is.Angle), d*math.Sin(is.Angle), 0))
	return image.Pt(int(math.Round(x)), int(math.Round(y)))
}

func (g *Game) zoomBy(d int) {
	n := min(max(g.level+d, 0), len(zoomLevels)-1)
	if n == g.level {
		return
	}
	// Entering or leaving the sky view swaps the scene: no scale animation
	// between two different scenes.
	if (n == 0) != (g.level == 0) {
		g.scale = float64(zoomLevels[n])
		g.anim = 0
	} else {
		g.fromScale = g.scale
		g.anim = transitionTicks
	}
	g.level = n
	g.dirty = true
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

	// Virtual-pixel coordinates are relative to the screen centre; the
	// origin is snapped to whole screen pixels for crisp output.
	g.panX, g.panY = g.clampPan(g.panX, g.panY)
	ox := math.Round(float64(g.outW)/2 + g.panX*g.scale)
	oy := math.Round(float64(g.outH)/2 + g.panY*g.scale)
	at := func(img *ebiten.Image, x, y float64) {
		var op ebiten.DrawImageOptions
		op.GeoM.Translate(x, y)
		op.GeoM.Scale(g.scale, g.scale)
		op.GeoM.Translate(ox, oy)
		screen.DrawImage(img, &op)
	}

	// Clouds drift below the islands, so they are drawn first.
	for i := range g.clouds {
		c := &g.clouds[i]
		c.lastDraw = int(c.x)
		at(c.img, math.Floor(c.x), c.y)
	}

	islands := g.src.Islands(time.Now())
	if g.level == 0 {
		pb := g.portal[g.frame].Bounds()
		at(g.portal[g.frame], -float64(pb.Dx()/2), -float64(pb.Dy()/2))
		sort.Slice(islands, func(i, j int) bool {
			return g.orbitPixel(islands[i]).Y < g.orbitPixel(islands[j]).Y
		})
		for _, is := range islands {
			sp, p := g.miniature(is), g.orbitPixel(is)
			at(sp.img, float64(p.X-sp.anchor.X), float64(p.Y-sp.anchor.Y))
			if is.ID == g.src.Me {
				// Float the beacon above the island's highest point.
				top := p.Y - sp.anchor.Y - 4
				at(g.beacon.img, float64(p.X-g.beacon.anchor.X), float64(top-g.beacon.anchor.Y))
			}
		}
	} else if home := g.homeIsland(islands); home != nil {
		b := home.Bounds()
		at(home, -float64(b.Dx()/2), -float64(b.Dy()*2/5))
	}

	if g.phase != "" {
		ebitenutil.DebugPrint(screen, fmt.Sprintf("%s · %s · %d:%02d · %d island(s)",
			g.src.Sky, g.phase, g.phaseLeft/60, g.phaseLeft%60, len(islands)))
	}
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

// Package client is Apogée's Ebitengine front-end. It only displays the sky
// and forwards the player's intentions: it never simulates (GDD 10.3).
package client

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"sort"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"apogee/internal/art"
	"apogee/internal/proto"
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

	islandSprites map[miniKey]sprite // sky view miniatures
	beacon        sprite             // marks the player's island
	swarm         sprite             // a wave of monsters
	swarmAt       image.Point        // its position at the last redraw
	banners       [sim.NumOuter]sprite
	warbands      [3]sprite // by raid kind
	// selected is the island the god looks at in the sky view, to whisper
	// an outer intention towards it (GDD 6.3).
	selected proto.IslandID
	// home is the player's island at full size, rendered from the state and
	// re-rendered only when its signature changes: the ground with its
	// shadows, and each object apart, so that people can walk behind them.
	home     sprite
	homeObjs []object
	homeSig  uint64

	// lastTick is the tick of the last state drawn: a new state means a
	// redraw, about once a second.
	lastTick uint64
	// buttons are the clickable areas of the last redraw.
	buttons []button
	// draft is the pie the player is composing before whispering it.
	draft    sim.Pie
	drafting bool
	// Panels shown on demand: culture and knowledge, the Annals.
	showCulture, showAnnals bool

	// Notify, when set, receives the chronicle entries the player chose to
	// hear of while the window is not focused (GDD 13.1).
	Notify func(text string)
	// NotifyKinds are the kinds of entries worth a notification.
	NotifyKinds map[sim.EntryKind]bool
	// life animates the people of the island in view; walkerArt holds the
	// villager and defender sprites.
	life      life
	dust      particles
	pixel     *ebiten.Image // one white pixel, tinted to draw particles
	walkerArt [2]sprite
	animTick  int
	// atTinted draws an image in world coordinates with a colour, during a
	// redraw.
	atTinted func(img *ebiten.Image, x, y float64, c color.RGBA)

	// heard is the tick of the newest chronicle entry already considered.
	heard  uint64
	primed bool
	// focused reports whether the window has the focus; tests replace it.
	focused func() bool

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
	// dsf is the monitor's device scale factor: the screen is in physical
	// pixels, so a zoom level covers level×dsf of them.
	dsf float64

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

// portalFrame returns the portal animation frame at a time in milliseconds.
// The modulo is taken in 64 bits, so the index stays valid where int is 32
// bits wide.
func portalFrame(ms int64) int {
	return int(ms * portalFPS / 1000 % portalFrames)
}

// pixel is the virtual-pixel column where the cloud is drawn. The same value
// decides redraws, so a cloud never moves on screen without a redraw.
func (c *cloud) pixel() int { return int(math.Floor(c.x)) }

// drift moves the cloud by one tick, wrapping it around beyond limit, and
// reports whether its drawn position changed.
func (c *cloud) drift(limit float64) bool {
	c.x += c.speed
	if c.x > limit {
		c.x = -limit
	}
	return c.pixel() != c.lastDraw
}

// WaveVisible reports whether a wave is flying across the sky view.
func (g *Game) WaveVisible() bool {
	_, ok := g.swarmPixel(g.src.Islands(time.Now()), time.Now())
	return ok && g.level == 0
}

// notifyNews passes new chronicle entries of the chosen kinds to Notify,
// only while the window is not focused: in front of the game, the chronicle
// says it all. Entries older than the first state are never notified.
func (g *Game) notifyNews(st *proto.State) {
	if st.Own == nil {
		return
	}
	newest := g.heard
	for _, e := range st.Own.Chronicle {
		if e.Tick <= g.heard && g.primed {
			continue
		}
		newest = max(newest, e.Tick)
		if g.primed && g.Notify != nil && g.NotifyKinds[e.Kind] && !g.isFocused() {
			g.Notify(e.Text)
		}
	}
	g.heard, g.primed = newest, true
}

func (g *Game) isFocused() bool {
	if g.focused != nil {
		return g.focused()
	}
	return ebiten.IsFocused()
}

// Phase returns the current time of day ("dawn", "day", "dusk", "night").
func (g *Game) Phase() string {
	phase, _, _ := g.src.Phase(time.Now())
	return phase
}

// Select picks the island the god looks at, as a click would.
func (g *Game) Select(id proto.IslandID) {
	g.selected = id
	g.dirty = true
}

// ShowPanels opens the culture and Annals panels, for screenshots.
func (g *Game) ShowPanels(culture, annals bool) {
	g.showCulture, g.showAnnals = culture, annals
	g.dirty = true
}

// SetZoom selects a zoom level by index, without animation.
func (g *Game) SetZoom(level int) {
	g.level = min(max(level, 0), len(zoomLevels)-1)
	g.scale = float64(zoomLevels[g.level])
	g.dirty = true
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
		islandSprites: map[miniKey]sprite{},
		positions:     map[sim.IslandID]image.Point{},
		dirty:         true,
	}
	g.scale = float64(zoomLevels[g.level])
	b := cam.Render(art.Beacon())
	g.beacon = sprite{ebiten.NewImageFromImage(b.Img), b.Anchor}
	for i, mat := range []art.Material{art.Roof, art.Team} { // villagers, defenders
		v := cam.Render(art.Person(seed+uint64(i), mat))
		g.walkerArt[i] = sprite{ebiten.NewImageFromImage(v.Img), v.Anchor}
	}
	g.pixel = ebiten.NewImage(1, 1)
	g.pixel.Fill(color.White)
	sw := cam.Render(art.Swarm(seed))
	g.swarm = sprite{ebiten.NewImageFromImage(sw.Img), sw.Anchor}
	for k, mat := range map[sim.Outer]art.Material{sim.Covet: art.Roof, sim.Annihilate: art.Blood} {
		b := cam.Render(art.Banner(mat))
		g.banners[k] = sprite{ebiten.NewImageFromImage(b.Img), b.Anchor}
	}
	for k, mat := range map[sim.RaidKind]art.Material{sim.Pillage: art.Team, sim.Assault: art.Blood, sim.Relief: art.Plaster} {
		w := cam.Render(art.Warband(seed+uint64(k), mat))
		g.warbands[k] = sprite{ebiten.NewImageFromImage(w.Img), w.Anchor}
	}
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

// miniKey identifies a sky view miniature: an island grows when its people
// push its edges, so the size is part of the key.
type miniKey struct {
	id   sim.IslandID
	size int
}

// miniScale shrinks islands for the sky view.
const miniScale = 3.5

func (g *Game) miniature(is proto.IslandSummary) sprite {
	key := miniKey{is.ID, int(is.Size)}
	if sp, ok := g.islandSprites[key]; ok {
		return sp
	}
	flat := art.DemoIsland(g.cam, is.Seed, max(is.Size, 1)/miniScale).Flatten(g.cam)
	sp := sprite{ebiten.NewImageFromImage(flat.Img), flat.Anchor}
	g.islandSprites[key] = sp
	return sp
}

// purgeMiniatures frees the miniatures of islands that left the sky or grew.
func (g *Game) purgeMiniatures(islands []proto.IslandSummary) {
	keep := make(map[miniKey]bool, len(islands))
	for _, is := range islands {
		keep[miniKey{is.ID, int(is.Size)}] = true
	}
	for k, sp := range g.islandSprites {
		if !keep[k] {
			sp.img.Deallocate()
			delete(g.islandSprites, k)
		}
	}
}

// homeIsland returns the player's island at full size, as the simulation
// describes it (GDD 3.5), re-rendering it only when something changed.
func (g *Game) homeIsland(st *proto.State) (sprite, bool) {
	if st == nil || st.Own == nil {
		return sprite{}, false
	}
	var sum proto.IslandSummary
	for _, is := range st.Islands {
		if is.ID == st.You {
			sum = is
		}
	}
	if sum.ID == 0 {
		return sprite{}, false
	}
	if sig := signature(sum, st.Own); sig != g.homeSig || g.home.img == nil {
		if g.home.img != nil {
			g.home.img.Deallocate()
		}
		for _, o := range g.homeObjs {
			o.img.Deallocate()
		}
		scene := islandScene(g.cam, sum, st.Own)
		ground := scene.FlattenGround(g.cam)
		g.home = sprite{ebiten.NewImageFromImage(ground.Img), ground.Anchor}
		g.homeObjs = g.homeObjs[:0]
		for _, o := range scene.Sorted() {
			g.homeObjs = append(g.homeObjs, object{
				sprite: sprite{ebiten.NewImageFromImage(o.Sprite.Img), o.Sprite.Anchor},
				x:      o.Pos.X, y: o.Pos.Y,
			})
		}
		g.homeSig = sig
	}
	return g.home, true
}

// Update advances animations at a fixed rate and decides whether a redraw is
// needed. Nothing is drawn when nothing visible changed.
func (g *Game) Update() error {
	if err := g.src.Err(); err != nil {
		return fmt.Errorf("connection to the sky lost: %w", err)
	}
	if st := g.src.State(); st != nil && st.Tick != g.lastTick {
		g.lastTick = st.Tick
		g.dirty = true
		// Free the miniatures of islands that fell or grew, whatever the
		// zoom level.
		g.purgeMiniatures(st.Islands)
		g.notifyNews(st)
		if !g.drafting && st.Own != nil {
			g.draft = st.Own.Pie
		}
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		x, y := ebiten.CursorPosition()
		// The interface is drawn over the world: its buttons, registered
		// last, are tried first.
		hit := false
		for i := len(g.buttons) - 1; i >= 0; i-- {
			if b := g.buttons[i]; image.Pt(x, y).In(b.r) {
				b.action()
				g.dirty = true
				hit = true
				break
			}
		}
		if !hit && g.selected != 0 {
			g.selected = 0 // a click in the void clears the selection
			g.dirty = true
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyC) {
		g.showCulture = !g.showCulture
		g.dirty = true
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyA) {
		g.showAnnals = !g.showAnnals
		g.dirty = true
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

	// Clouds wrap around once fully out of sight, whatever the window size,
	// zoom level or scrolling.
	limit := float64(g.outW)/(2*g.pixelScale()) + math.Abs(g.panX) + 48
	for i := range g.clouds {
		if g.clouds[i].drift(limit) {
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
		f := portalFrame(now.UnixMilli())
		if f != g.frame {
			g.frame = f
			g.dirty = true
		}
		// Orbits are slow: redraw only when an island moves by a pixel.
		islands := g.src.Islands(now)
		for _, is := range islands {
			p := g.orbitPixel(is)
			if g.positions[is.ID] != p {
				g.positions[is.ID] = p
				g.dirty = true
			}
		}
		if p, ok := g.swarmPixel(islands, now); ok && p != g.swarmAt {
			g.swarmAt = p
			g.dirty = true
		}
		if st := g.src.State(); st != nil && len(st.Raids) > 0 {
			g.dirty = true // warbands are on the move
		}
	}
	g.updatePan()
	g.animate()
	return nil
}

func (g *Game) dsfOr1() float64 {
	if g.dsf <= 0 {
		return 1
	}
	return g.dsf
}

// pixelScale is the number of physical pixels per virtual pixel. At rest it
// is rounded to a whole number, so pixels stay crisp even with a fractional
// device scale factor; during a zoom transition it follows the animation.
func (g *Game) pixelScale() float64 {
	k := g.scale * g.dsfOr1()
	if g.anim == 0 {
		k = math.Max(1, math.Round(k))
	}
	return k
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
	k := g.pixelScale()
	x, y := g.panX+dx*panSpeed*g.dsfOr1()/k, g.panY+dy*panSpeed*g.dsfOr1()/k
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
	if g.home.img == nil {
		return image.Rectangle{}
	}
	return g.home.img.Bounds().Sub(g.home.anchor)
}

// clampPan keeps the scene edges from leaving the screen: on an axis where
// the scene fits, it stays centred.
func (g *Game) clampPan(x, y float64) (float64, float64) {
	b := g.sceneBounds()
	if b.Empty() || g.outW == 0 {
		return 0, 0
	}
	k := g.pixelScale()
	halfW, halfH := float64(g.outW)/2/k, float64(g.outH)/2/k
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

// fallDuration matches the simulation's: the time the whirlpool takes to
// swallow an island, in milliseconds.
const fallDuration = 30_000.0

func (g *Game) orbitPixel(is proto.IslandSummary) image.Point {
	d := is.Radius / sim.RimRadius * rimPixels
	a := is.Angle
	if is.FallingMs > 0 {
		// Swallowed by the whirlpool: the island spirals into the portal
		// (GDD 12.6).
		f := float64(is.FallingMs) / fallDuration
		d *= f
		a += (1 - f) * 4 * math.Pi
	}
	x, y := g.cam.Project(art.V(d*math.Cos(a), d*math.Sin(a), 0))
	return image.Pt(int(math.Round(x)), int(math.Round(y)))
}

// swarmWarning is how long before striking a wave shows, flying from the
// portal to its target.
const swarmWarning = 40 * time.Second

func (g *Game) swarmPixel(islands []proto.IslandSummary, now time.Time) (image.Point, bool) {
	target, left, ok := g.src.Wave(now)
	if !ok || left > swarmWarning {
		return image.Point{}, false
	}
	for _, is := range islands {
		if is.ID == target {
			t := 1 - float64(left)/float64(swarmWarning)
			p := g.orbitPixel(is)
			return image.Pt(int(float64(p.X)*t), int(float64(p.Y)*t)), true
		}
	}
	return image.Point{}, false
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
	g.buttons = g.buttons[:0]
	screen.Clear()

	// Nearest-neighbour stretching: linear filtering would blend the one
	// pixel wide gradient with the transparent void around it, leaving the
	// sky half transparent over the previous frame.
	light := g.daylight()
	var sky ebiten.DrawImageOptions
	sky.GeoM.Scale(float64(g.outW), float64(g.outH)/256)
	sky.ColorScale = light
	screen.DrawImage(g.sky, &sky)

	// Virtual-pixel coordinates are relative to the screen centre; the
	// origin is snapped to whole screen pixels for crisp output.
	g.panX, g.panY = g.clampPan(g.panX, g.panY)
	k := g.pixelScale()
	ox := math.Round(float64(g.outW)/2 + g.panX*k)
	oy := math.Round(float64(g.outH)/2 + g.panY*k)
	atScaled := func(img *ebiten.Image, x, y float64, cs ebiten.ColorScale) {
		var op ebiten.DrawImageOptions
		op.GeoM.Translate(x, y)
		op.GeoM.Scale(k, k)
		op.GeoM.Translate(ox, oy)
		op.ColorScale = cs
		op.ColorScale.ScaleWithColorScale(light) // the time of day (GDD 12.5)
		screen.DrawImage(img, &op)
	}
	at := func(img *ebiten.Image, x, y float64) { atScaled(img, x, y, ebiten.ColorScale{}) }
	g.atTinted = func(img *ebiten.Image, x, y float64, c color.RGBA) {
		var cs ebiten.ColorScale
		cs.ScaleWithColor(c)
		atScaled(img, x, y, cs)
	}

	// Clouds drift below the islands, so they are drawn first.
	for i := range g.clouds {
		c := &g.clouds[i]
		c.lastDraw = c.pixel()
		at(c.img, float64(c.lastDraw), c.y)
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
			var cs ebiten.ColorScale
			switch {
			case is.FallingMs > 0:
				cs.ScaleAlpha(float32(float64(is.FallingMs) / fallDuration))
			case is.Declining:
				cs.Scale(0.8, 0.75, 0.95, 1) // ternished by the decline
			}
			atScaled(sp.img, float64(p.X-sp.anchor.X), float64(p.Y-sp.anchor.Y), cs)
			// Values from the wire are bounded before indexing: a corrupt
			// state must not crash the client.
			if is.Banner > 0 && int(is.Banner) < len(g.banners) && g.banners[is.Banner].img != nil && is.FallingMs == 0 {
				b := g.banners[is.Banner]
				top := p.Y - sp.anchor.Y + 6
				at(b.img, float64(p.X+sp.anchor.X/3-b.anchor.X), float64(top-b.anchor.Y))
			}
			if is.ID == g.src.Me() && is.FallingMs == 0 {
				// Float the beacon above the island's highest point.
				top := p.Y - sp.anchor.Y - 4
				at(g.beacon.img, float64(p.X-g.beacon.anchor.X), float64(top-g.beacon.anchor.Y))
			}
		}
		if p, ok := g.swarmPixel(islands, time.Now()); ok {
			at(g.swarm.img, float64(p.X-g.swarm.anchor.X), float64(p.Y-g.swarm.anchor.Y))
		}
		g.drawRaids(at, islands)
		g.registerIslands(islands, k, ox, oy)
	} else if home, ok := g.homeIsland(g.src.State()); ok {
		at(home.img, -float64(home.anchor.X), -float64(home.anchor.Y))
		g.drawWalkers(at)
	}

	g.drawHUD(screen, g.src.State())
	if g.OnFrame != nil {
		g.OnFrame(screen)
	}
}

// Layout returns the screen size it is given. Ebitengine calls LayoutF, which
// converts to physical pixels first.
func (g *Game) Layout(outW, outH int) (int, int) {
	if outW != g.outW || outH != g.outH {
		g.outW, g.outH = outW, outH
		g.dirty = true
	}
	return outW, outH
}

// LayoutF receives the window size in device-independent pixels and returns
// it in physical pixels, so that the integer zoom lands on real pixels and
// stays crisp on high-DPI monitors.
func (g *Game) LayoutF(outW, outH float64) (float64, float64) {
	f := 1.0
	if m := ebiten.Monitor(); m != nil {
		f = m.DeviceScaleFactor()
	}
	if f != g.dsf {
		g.dsf = f
		g.dirty = true
	}
	w, h := g.Layout(int(math.Round(outW*f)), int(math.Round(outH*f)))
	return float64(w), float64(h)
}

// drawRaids draws the groups crossing the sky, between the islands they
// left and the ones they head for (GDD 7.2: visible to all).
func (g *Game) drawRaids(at func(*ebiten.Image, float64, float64), islands []proto.IslandSummary) {
	st := g.src.State()
	if st == nil {
		return
	}
	where := map[proto.IslandID]image.Point{}
	for _, is := range islands {
		where[is.ID] = g.orbitPixel(is)
	}
	for _, r := range st.Raids {
		a, okA := where[r.From]
		b, okB := where[r.To]
		if !okA || !okB {
			continue
		}
		if r.Kind < 0 || int(r.Kind) >= len(g.warbands) {
			continue
		}
		t := min(max(r.Progress, 0), 1)
		x := float64(a.X) + float64(b.X-a.X)*t
		y := float64(a.Y) + float64(b.Y-a.Y)*t - 12 // they fly above the islands
		w := g.warbands[r.Kind]
		at(w.img, x-float64(w.anchor.X), y-float64(w.anchor.Y))
	}
}

// registerIslands makes the islands of the sky view clickable: a click
// selects one, to whisper an outer intention towards it.
func (g *Game) registerIslands(islands []proto.IslandSummary, k, ox, oy float64) {
	for _, is := range islands {
		if is.ID == g.src.Me() || is.FallingMs > 0 {
			continue
		}
		sp, p := g.miniature(is), g.orbitPixel(is)
		b := sp.img.Bounds()
		x0 := ox + float64(p.X-sp.anchor.X)*k
		y0 := oy + float64(p.Y-sp.anchor.Y)*k
		r := image.Rect(int(x0), int(y0), int(x0+float64(b.Dx())*k), int(y0+float64(b.Dy())*k))
		id := is.ID
		g.buttons = append(g.buttons, button{r, func() { g.selected = id }})
	}
}

// animate moves the people of the island in view, and asks for a redraw
// about fifteen times a second while they walk (GDD: a living island).
func (g *Game) animate() {
	st := g.src.State()
	if g.level == 0 || st == nil || st.Own == nil {
		return
	}
	var sum proto.IslandSummary
	for _, is := range st.Islands {
		if is.ID == st.You {
			sum = is
		}
	}
	if sum.ID == 0 {
		return
	}
	g.life.sync(sum, st.Own)
	dt := 1.0 / float64(ebiten.TPS())
	g.life.step(st.Own, dt, st.Phase == "night")
	for i := range g.life.walkers {
		if w := &g.life.walkers[i]; w.busy() {
			g.dust.emit(w, dt)
		}
	}
	g.dust.step(dt)
	g.animTick++
	if g.animTick%4 == 0 && len(g.life.walkers) > 0 {
		g.dirty = true
	}
}

// object is a tree, a rock or a building of the island in view, drawn apart
// from the ground so that people can pass behind it.
type object struct {
	sprite
	x, y float64 // island units
}

// drawWalkers draws the island's objects and its people together, far to
// near: a villager behind a house is hidden by it.
func (g *Game) drawWalkers(at func(*ebiten.Image, float64, float64)) {
	type item struct {
		sp     sprite
		x, y   float64
		lift   float64 // pixels up, for a busy villager's bob
		person bool
	}
	items := make([]item, 0, len(g.homeObjs)+len(g.life.walkers))
	for _, o := range g.homeObjs {
		items = append(items, item{sp: o.sprite, x: o.x, y: o.y})
	}
	bob := (g.animTick / 12) % 2
	for _, w := range g.life.walkers {
		sp := g.walkerArt[0]
		if w.guard {
			sp = g.walkerArt[1]
		}
		it := item{sp: sp, x: w.x, y: w.y, person: true}
		if w.busy() && bob == 1 {
			it.lift = 1 // hammering, harvesting
		}
		items = append(items, it)
	}
	// Stable: objects keep their order among themselves; at equal depth a
	// person stands in front of the object it works at.
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].y != items[j].y {
			return items[i].y < items[j].y
		}
		return !items[i].person && items[j].person
	})
	for _, it := range items {
		x, y := g.cam.Project(artV(it.x, it.y))
		px, py := math.Round(x), math.Round(y)-it.lift
		at(it.sp.img, px-float64(it.sp.anchor.X), py-float64(it.sp.anchor.Y))
	}
	// Particles float above everything, one virtual pixel each.
	for _, q := range g.dust.ps {
		x, y := g.cam.Project(art.V(q.x, q.y, q.z))
		g.atTinted(g.pixel, math.Round(x), math.Round(y), q.col)
	}
}

func artV(x, y float64) art.Vec3 { return art.V(x, y, 0) }

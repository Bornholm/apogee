// Package client is Apogée's Ebitengine front-end. It only displays the sky
// and forwards the player's intentions: it never simulates (GDD 10.3).
package client

import (
	"image"
	"image/color"
	"math"
	"sort"
	"time"
	"unicode"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"apogee/internal/art"
	"apogee/internal/geo"
	"apogee/internal/proto"
	"apogee/internal/sim"
	"apogee/internal/sound"
	"apogee/internal/update"
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
	// viewing is the other island shown in the island view, or 0 for one's
	// own.
	viewing proto.IslandID

	// The tutorial (tutorial.go): whether it shows, its step, and whether
	// the step already set its zoom level.
	tutorial       bool
	tutStep        int
	tutZoomed      bool
	onTutorialDone func()
	// areas are the screen areas of the HUD panels, for the tutorial.
	areas map[string]image.Rectangle
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
	// Panels shown on demand: culture and knowledge, the Annals; the pie
	// in the island view, and the whole chronicle.
	book bookTab // the tab of the book panel open, if any
	// stockLog samples the reserves, for their trend (civ.go).
	stockLog              []stockSample
	pieOpen, chronicleAll bool
	// pieHidden puts the pie away in the sky view, where it shows by
	// default; pieOpen shows it in the island view, where it does not.
	pieHidden bool
	// targetAt is where the island selected in the sky view stands on
	// screen, for its card.
	targetAt image.Rectangle

	// Notify, when set, receives the chronicle entries the player chose to
	// hear of while the window is not focused (GDD 13.1).
	Notify func(text string)
	// NotifyKinds are the kinds of entries worth a notification.
	NotifyKinds map[sim.EntryKind]bool
	// life animates the people of the island in view; walkerArt holds the
	// villager and defender sprites.
	// life is the player's own people, which live on whatever the view;
	// seen is the people of the island observed, if any.
	life life
	seen life
	// battle plays out the last attack on the player's island; foeArt
	// holds the sprites of the assailants.
	battle   battle
	rehearse *sim.Battle // a battle to show at the next state, for screenshots
	// nextBattle waits for the battle being played out to end.
	nextBattle *sim.Battle
	// seenBattle plays out the last attack on the island observed, for
	// seenOf; the whole sky saw it.
	seenBattle battle
	seenOf     proto.IslandID
	// signs make the events of the player's island visible (signs.go),
	// troops the groups that come and go (troops.go).
	signs     signs
	troops    troops
	allyArt   sprite
	foeArt    [foeArts]sprite
	dust      particles
	pixel     *ebiten.Image // one white pixel, tinted to draw particles
	walkerArt [3]sprite     // villager, defender, child
	dogArt    sprite
	steleArt  sprite
	sheepArt  [2]sprite // grazing, walking
	// homeChimneys smoke over the island in view; birds wheel above it.
	homeChimneys []art.Vec3
	birds        birds
	// scenery floats the island in view (float.go).
	scenery scenery
	// wx is the weather of the moment (weather.go); ForceWeather names one
	// for screenshots. atSkew draws a sprite leaning by a shear, from its
	// base, during a redraw.
	wx           weather
	ForceWeather string
	atSkew       func(img *ebiten.Image, x, y, baseY, shear float64)
	animTick     int
	skyTick      int // counts every tick, the sky view included
	// atTinted draws an image in world coordinates with a colour, during a
	// redraw.
	atTinted func(img *ebiten.Image, x, y float64, c color.RGBA)
	// Lights of the night (lights.go): the halos, the lamps of the island
	// in view, and glowAt, which draws one during a redraw untouched by
	// the time of day, added to the scene when add is set.
	halos     [len(haloRadii)]*ebiten.Image
	homeLamps []lamp
	glowAt    func(img *ebiten.Image, x, y float64, c color.RGBA, strength float64, add bool)
	// ForcePhase, when set, stands for the time of day, for screenshots.
	ForcePhase string

	// Newer, when set, is a version of the game published since this one:
	// the top bar offers its page.
	Newer *update.Release

	// connection is shown while the connection to the sky is lost;
	// OnGiveUp, when set, takes the player back to the home screen.
	connection string
	OnGiveUp   func()
	// OnHome, when set, ends the game and goes back to the home screen.
	OnHome func()

	// Sound, when set, plays the effects and the music (GDD 12.7); mood
	// is the last one told to the music. OnMute is called when the player
	// silences the game or brings the sound back, to remember it.
	Sound  *sound.Engine
	mood   sound.Mood
	played bool // an action of the last click played its own effect
	// lastFx and lastAny space out the effects of events (mayPlay).
	lastFx  [sound.NumEffects]time.Time
	lastAny time.Time
	onPlay  func(sound.Effect) // tests listen here
	chars   []rune             // characters typed this tick
	OnMute  func(muted bool)

	// heard is the tick of the newest chronicle entry already considered;
	// held are the outcomes of battles still played out (outcomePending).
	heard  uint64
	held   []sim.Entry
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
	loudest := -1
	for _, e := range st.Own.Chronicle {
		if e.Tick <= g.heard && g.primed {
			continue
		}
		newest = max(newest, e.Tick)
		if !g.primed {
			continue
		}
		if g.outcomePending(e) {
			g.held = append(g.held, e) // told when the battle ends
			continue
		}
		if g.Notify != nil && g.NotifyKinds[e.Kind] && !g.isFocused() {
			g.Notify(e.Text)
		}
		if r := newsRank(e.Kind); r > loudest {
			loudest = r
		}
	}
	g.heard, g.primed = newest, true
	// Several entries in one state make one sound: the weightiest.
	if loudest >= 0 {
		g.play(newsSounds[loudest].fx)
	}
}

// outcomeKinds are the entries that tell how an attack on the island ended.
func outcomeKind(k sim.EntryKind) bool {
	return k == sim.VictoryNews || k == sim.WaveNews || k == sim.RaidNews
}

// outcomePending reports whether an entry tells the outcome of a battle
// still being played out, or waiting its turn: the simulation resolves an
// attack at once, but the player must not read how it ended before seeing
// it (GDD 13.1 bis).
func (g *Game) outcomePending(e sim.Entry) bool {
	if !outcomeKind(e.Kind) {
		return false
	}
	return (g.battle.active() && g.battle.b.Tick == e.Tick) || (g.nextBattle != nil && g.nextBattle.Tick == e.Tick)
}

// releaseOutcomes tells the outcomes held back, once their battle is over:
// the chronicle shows them, the sound plays, the notification goes.
func (g *Game) releaseOutcomes() {
	if len(g.held) == 0 {
		return
	}
	kept := g.held[:0]
	loudest := -1
	for _, e := range g.held {
		if g.outcomePending(e) {
			kept = append(kept, e)
			continue
		}
		if g.Notify != nil && g.NotifyKinds[e.Kind] && !g.isFocused() {
			g.Notify(e.Text)
		}
		loudest = max(loudest, newsRank(e.Kind))
	}
	g.held = kept
	// The blow on the heart was heard during the battle.
	if loudest >= 0 && newsSounds[loudest].fx != sound.Struck {
		g.play(newsSounds[loudest].fx)
	}
	g.dirty = true
}

// isHeld reports whether an entry waits for the end of its battle.
func (g *Game) isHeld(e sim.Entry) bool {
	for _, h := range g.held {
		if h.Tick == e.Tick && h.Text == e.Text {
			return true
		}
	}
	return false
}

// newsSounds are the effects of the chronicle's entries, from the least to
// the most important.
var newsSounds = []struct {
	kind sim.EntryKind
	fx   sound.Effect
}{
	{sim.BuiltNews, sound.Built},
	{sim.BirthNews, sound.Birth},
	{sim.DiscoveryNews, sound.Discovery},
	{sim.SuppliqueNews, sound.Supplique},
	{sim.VictoryNews, sound.Victory},
	{sim.DeclineNews, sound.Decline},
	{sim.RaidNews, sound.Raid},
	{sim.WaveNews, sound.Struck},
	{sim.FallNews, sound.Fall},
}

// newsRank returns the index of an entry's sound in newsSounds, or -1 when
// it makes none.
func newsRank(k sim.EntryKind) int {
	for i, n := range newsSounds {
		if n.kind == k {
			return i
		}
	}
	return -1
}

// play starts an effect, if the game has sound.
func (g *Game) play(fx sound.Effect) {
	g.played = true
	if !g.mayPlay(fx, time.Now()) {
		return
	}
	if g.onPlay != nil {
		g.onPlay(fx)
	}
	if g.Sound != nil {
		g.Sound.Play(fx)
	}
}

// playScene plays a sound of what the island shows, a battle: it follows
// the animation, without the spacing of the news.
func (g *Game) playScene(fx sound.Effect) {
	if g.onPlay != nil {
		g.onPlay(fx)
	}
	if g.Sound != nil {
		g.Sound.Play(fx)
	}
}

// Effects of events are spaced out: the same one waits effectGap before it
// sounds again, and any two wait eventGap. What the player does (a click,
// a whisper, an answer) always sounds at once.
const (
	effectGap = 20 * time.Second
	eventGap  = 4 * time.Second
)

// mayPlay tells whether an effect may sound now, and notes it if so.
func (g *Game) mayPlay(fx sound.Effect, now time.Time) bool {
	switch fx {
	case sound.Click, sound.Whisper, sound.Answer:
		return true
	}
	if fx < 0 || fx >= sound.NumEffects || now.Sub(g.lastFx[fx]) < effectGap || now.Sub(g.lastAny) < eventGap {
		return false
	}
	g.lastFx[fx], g.lastAny = now, now
	return true
}

// shortcut runs the keyboard shortcut of a typed character. The keypad's
// + and − type characters too.
func (g *Game) shortcut(r rune) {
	switch unicode.ToLower(r) {
	case 'h':
		g.OpenTutorial()
	case 'm':
		g.ToggleMute()
	case 's':
		g.togglePie()
	case 'i':
		g.togglePanel(civTab)
	case 'c':
		g.togglePanel(cultureTab)
	case 'a':
		g.togglePanel(annalsTab)
	case '+', '=':
		g.zoomBy(1)
	case '-', '−':
		g.zoomBy(-1)
	}
}

// ToggleMute silences the game, or brings the sound back.
func (g *Game) ToggleMute() {
	if g.Sound == nil {
		return
	}
	m := !g.Sound.Muted()
	g.Sound.SetMuted(m)
	if g.OnMute != nil {
		g.OnMute(m)
	}
	g.dirty = true
}

// updateMood tells the music the time of day, and whether the island
// declines.
func (g *Game) updateMood(st *proto.State) {
	if g.Sound == nil {
		return
	}
	m := sound.Mood{Phase: g.phase}
	if st != nil {
		for _, is := range st.Islands {
			if is.ID == st.You {
				m.Decline = is.Declining
			}
		}
	}
	if m != g.mood {
		g.mood = m
		g.Sound.SetMood(m)
	}
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

// ShowPanels opens a tab of the book panel, for screenshots: the
// civilization, the culture or the Annals.
func (g *Game) ShowPanels(civ, culture, annals bool) {
	switch {
	case civ:
		g.book = civTab
	case culture:
		g.book = cultureTab
	case annals:
		g.book = annalsTab
	}
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
	ch := cam.Render(art.Child(seed+5, art.Crop))
	g.walkerArt[2] = sprite{ebiten.NewImageFromImage(ch.Img), ch.Anchor}
	dog := cam.Render(art.Dog(seed))
	g.dogArt = sprite{ebiten.NewImageFromImage(dog.Img), dog.Anchor}
	st := cam.Render(art.Stele(seed))
	g.steleArt = sprite{ebiten.NewImageFromImage(st.Img), st.Anchor}
	for i := range g.sheepArt {
		sh := cam.Render(art.Sheep(seed + uint64(i)))
		g.sheepArt[i] = sprite{ebiten.NewImageFromImage(sh.Img), sh.Anchor}
	}
	for i := range foeArts {
		var m art.Mesh
		switch i {
		case colossusArt:
			m = art.Creature(seed+99, 4.5)
		case raiderArt:
			m = art.Person(seed+7, art.Blood)
		case swarmArt:
			m = art.Creature(seed+11, 1.1)
		case devourerArt:
			m = art.CreatureOf(seed+13, 2, art.Foliage)
		case breakerArt:
			m = art.CreatureOf(seed+17, 2.8, art.Blood)
		case shadeArt:
			m = art.CreatureOf(seed+19, 2.2, art.Shade)
		default:
			m = art.Creature(seed+uint64(i), 2)
		}
		f := cam.Render(m)
		g.foeArt[i] = sprite{ebiten.NewImageFromImage(f.Img), f.Anchor}
	}
	ally := cam.Render(art.Person(seed+3, art.Plaster))
	g.allyArt = sprite{ebiten.NewImageFromImage(ally.Img), ally.Anchor}
	g.pixel = ebiten.NewImage(1, 1)
	g.pixel.Fill(color.White)
	for i, r := range haloRadii {
		g.halos[i] = haloImage(r)
	}
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

// viewed returns the island shown in the island view: the player's own, or
// the one they observe (GDD pilier 2). An observed island is drawn from its
// public view only: buildings and people.
func (g *Game) viewed(st *proto.State) (proto.IslandSummary, *proto.Own, bool) {
	if st == nil {
		return proto.IslandSummary{}, nil, false
	}
	id, view := st.You, st.Own
	if g.viewing != 0 {
		if st.Seen == nil || st.Seen.Island != g.viewing {
			return proto.IslandSummary{}, nil, false // not received yet
		}
		id = g.viewing
		view = &proto.Own{Buildings: st.Seen.Buildings, Features: st.Seen.Features,
			Population: st.Seen.Population, Defenders: st.Seen.Defenders}
	}
	if view == nil {
		return proto.IslandSummary{}, nil, false
	}
	for _, is := range st.Islands {
		if is.ID == id {
			return is, view, true
		}
	}
	return proto.IslandSummary{}, nil, false
}

// Observe looks at another island up close; 0 comes back home.
func (g *Game) Observe(id proto.IslandID) {
	if st := g.src.State(); st != nil && id == st.You {
		id = 0
	}
	if id != g.viewing {
		g.dust.ps = nil // the dust of the island left behind
	}
	g.viewing = id
	g.src.Look(id)
	if id != 0 && g.level == 0 {
		g.SetZoom(2)
	}
	g.dirty = true
}

// homeIsland returns the player's island at full size, as the simulation
// describes it (GDD 3.5), re-rendering it only when something changed.
func (g *Game) homeIsland(st *proto.State) (sprite, bool) {
	sum, view, ok := g.viewed(st)
	if !ok {
		return sprite{}, false
	}
	if g.signs.rehearse == "relic" && g.viewing == 0 && sum.Relic == sim.NoRelic {
		sum.Relic = sim.Shield // for screenshots
	}
	if sig := signature(sum, view); sig != g.homeSig || g.home.img == nil {
		if g.home.img != nil {
			g.home.img.Deallocate()
		}
		for _, o := range g.homeObjs {
			o.img.Deallocate()
		}
		scene := islandScene(g.cam, sum, view)
		ground := scene.FlattenGround(g.cam)
		g.home = sprite{ebiten.NewImageFromImage(ground.Img), ground.Anchor}
		g.homeObjs = g.homeObjs[:0]
		for _, o := range scene.Sorted() {
			g.homeObjs = append(g.homeObjs, object{
				sprite: sprite{ebiten.NewImageFromImage(o.Sprite.Img), o.Sprite.Anchor},
				x:      o.Pos.X, y: o.Pos.Y, sways: o.Sways,
			})
		}
		g.homeLamps = islandLamps(sum, view)
		g.homeChimneys = islandChimneys(sum, view)
		g.birds.keep(sum.Size, sum.Seed)
		g.homeSig = sig
	}
	return g.home, true
}

// Update advances animations at a fixed rate and decides whether a redraw is
// needed. Nothing is drawn when nothing visible changed.
func (g *Game) Update() error {
	// A lost connection does not stop the game: the app reconnects, and the
	// island lives on meanwhile from the last state heard.
	if st := g.src.State(); st != nil && st.Tick != g.lastTick {
		g.lastTick = st.Tick
		g.dirty = true
		// Free the miniatures of islands that fell or grew, whatever the
		// zoom level.
		g.purgeMiniatures(st.Islands)
		g.logStock(st.Own, time.Now())
		// The battle first: the news of its outcome waits for its end.
		g.watchBattle(st)
		g.notifyNews(st)
		g.watchSigns(st)
		if g.viewing != 0 && g.viewing == st.You {
			g.Observe(0) // asked to observe one's own island before knowing it
		}
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
				g.played = false
				b.action()
				if !g.played && !b.silent {
					g.play(sound.Click)
				}
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
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) && g.viewing != 0 {
		g.Observe(0)
	}
	// Letter shortcuts follow the characters typed, not the physical keys:
	// they stay on their letter whatever the layout, AZERTY or QWERTY.
	g.chars = ebiten.AppendInputChars(g.chars[:0])
	for _, r := range g.chars {
		g.shortcut(r)
	}
	if _, wy := ebiten.Wheel(); wy != 0 {
		g.zoomBy(int(math.Copysign(1, wy)))
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
	g.updateMood(g.src.State())
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
	// islandTint is the colour the island in view takes, falling,
	// declining or struck; the leaning pines take it too.
	var islandTint ebiten.ColorScale
	g.atSkew = func(img *ebiten.Image, x, y, baseY, shear float64) {
		var op ebiten.DrawImageOptions
		op.GeoM.Translate(0, -baseY)
		op.GeoM.Skew(-shear, 0)
		op.GeoM.Translate(x, y+baseY)
		op.GeoM.Scale(k, k)
		op.GeoM.Translate(ox, oy)
		op.ColorScale = islandTint
		op.ColorScale.ScaleWithColorScale(light)
		screen.DrawImage(img, &op)
	}
	g.atTinted = func(img *ebiten.Image, x, y float64, c color.RGBA) {
		var cs ebiten.ColorScale
		cs.ScaleWithColor(c)
		atScaled(img, x, y, cs)
	}
	g.glowAt = func(img *ebiten.Image, x, y float64, c color.RGBA, f float64, add bool) {
		var op ebiten.DrawImageOptions
		op.GeoM.Translate(x, y)
		op.GeoM.Scale(k, k)
		op.GeoM.Translate(ox, oy)
		op.ColorScale.ScaleWithColor(c)
		op.ColorScale.ScaleAlpha(float32(f))
		if add {
			op.Blend = ebiten.BlendLighter
		}
		screen.DrawImage(img, &op)
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
		g.drawSkyLife(islands, func(p image.Point) (float64, float64) { return float64(p.X), float64(p.Y) })
		g.registerIslands(islands, k, ox, oy)
	} else if home, ok := g.homeIsland(g.src.State()); ok {
		// The island rises and sinks gently as it floats.
		oy -= bob(float64(g.animTick)/60, uint64(g.scenery.id)) * k
		if f := g.signs.state.falling; g.viewing == 0 && f > 0 {
			// The island falls into the whirlpool: it shakes, sinks and
			// fades into the sky.
			ox += math.Round(math.Sin(float64(g.animTick)*1.7) * 2 * k)
			oy += math.Round(f * f * 240 * k)
			var cs ebiten.ColorScale
			cs.ScaleAlpha(float32(1 - f))
			at = func(img *ebiten.Image, x, y float64) { atScaled(img, x, y, cs) }
			islandTint = cs
		} else if g.viewing == 0 && g.signs.state.declining && g.battle.strike() == 0 {
			// A declining island loses its colours.
			var cs ebiten.ColorScale
			cs.Scale(0.8, 0.76, 0.88, 1)
			at = func(img *ebiten.Image, x, y float64) { atScaled(img, x, y, cs) }
			islandTint = cs
		} else {
			// The heart struck: the island shakes and flushes red.
			if s := g.shownBattle().strike(); s > 0 {
				ox += math.Round(math.Sin(float64(g.animTick)*2.1) * s * 2 * k)
				var cs ebiten.ColorScale
				cs.Scale(1, float32(1-0.45*s), float32(1-0.45*s), 1)
				at = func(img *ebiten.Image, x, y float64) { atScaled(img, x, y, cs) }
				islandTint = cs
			}
		}
		at(home.img, -float64(home.anchor.X), -float64(home.anchor.Y))
		g.drawWalkers(at)
		if g.viewing == 0 {
			g.drawStorm(atScaled)
		}
		g.drawFog(atScaled, g.scenery.size)
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
	g.targetAt = image.Rectangle{}
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
		g.buttons = append(g.buttons, button{r: r, action: func() { g.selected = id }})
		if id == g.selected {
			g.targetAt = r
		}
	}
}

// animate moves the people of the island in view, and asks for a redraw
// about fifteen times a second while they walk (GDD: a living island). The
// player's own people keep walking in the sky view or while another island
// is observed: coming back, they are where their day took them.
func (g *Game) animate() {
	st := g.src.State()
	if st == nil {
		return
	}
	dt := 1.0 / float64(ebiten.TPS())
	phase, _, _ := g.moment()
	night := phase == "night"
	if st.Own != nil {
		for _, is := range st.Islands {
			if is.ID == st.You {
				g.life.sync(is, st.Own, signature(is, st.Own))
				g.life.rally = g.battle.rally()
				if w := g.signs.state.waveIn; w >= 0 && w < 45 && g.life.rally == nil {
					// A wave is coming: the guards gather at the heart.
					g.life.rally = []geo.Vec2{g.signs.heart}
				}
				g.answerSigns()
				g.life.step(dt, night)
				break
			}
		}
	}
	// The battle runs on whatever the view; its shots and bursts only show
	// on the island.
	home := g.level > 0 && g.viewing == 0
	if g.battle.active() {
		var guards []walker
		for _, w := range g.life.walkers {
			if w.guard {
				guards = append(guards, w)
			}
		}
		g.battle.step(dt, &g.dust, home, guards)
		for _, fx := range g.battle.sfx {
			g.playScene(fx)
		}
		g.battle.sfx = g.battle.sfx[:0]
		if !g.battle.active() {
			defer g.releaseOutcomes()
		}
		if c, ok := g.battle.reaction(); ok {
			g.life.call(c)
		}
		if home {
			g.dirty = true
		}
	} else if g.nextBattle != nil {
		bt := *g.nextBattle
		g.nextBattle = nil
		if st.Own != nil {
			g.startBattle(st, bt)
		}
	}
	g.signs.step(dt, &g.dust, home)
	// The weather and the ambience follow the sky, whatever the view.
	g.wx = g.weatherNow(st)
	if g.Sound != nil && g.skyTick%15 == 0 {
		g.Sound.SetAmbience(g.ambience())
	}
	g.skyTick++
	g.troops.step(dt)
	if home && (g.troops.active() || g.signs.state.waveIn >= 0) && g.animTick%4 == 0 {
		g.dirty = true
	}
	if g.level == 0 {
		return
	}
	l := g.shown()
	if g.viewing != 0 {
		sum, view, ok := g.viewed(st)
		if !ok {
			return
		}
		l.sync(sum, view, signature(sum, view))
		g.watchSeenBattle(st, sum, view)
		if b := &g.seenBattle; b.active() && g.seenOf == g.viewing {
			l.rally = b.rally()
			var guards []walker
			for _, w := range l.walkers {
				if w.guard {
					guards = append(guards, w)
				}
			}
			b.step(dt, &g.dust, true, guards)
			for _, fx := range b.sfx {
				g.playScene(fx)
			}
			b.sfx = b.sfx[:0]
			if c, ok := b.reaction(); ok {
				l.call(c)
			}
			g.dirty = true
		} else {
			l.rally = nil
		}
		l.step(dt, night)
	}
	for i := range l.walkers {
		if w := &l.walkers[i]; w.busy() {
			g.dust.emit(w, dt)
		}
	}
	g.dust.smoke(g.homeChimneys, g.wx.wind, dt)
	if sum, _, ok := g.viewed(st); ok {
		g.dust.rainOver(sum.Size, g.wx.rain, g.wx.wind, dt)
	}
	if sum, view, ok := g.viewed(st); ok {
		g.scenery.keep(sum)
		g.scenery.keepSprings(view)
		g.scenery.step(dt, &g.dust)
	}
	// The birds flee the night, a wave on its way and a battle.
	threat := g.shownBattle().active() || (g.viewing == 0 && g.signs.state.waveIn >= 0 && g.signs.state.waveIn < 45)
	g.birds.step(dt, threat || g.dark() > 0.6 || g.wx.rain > 0.5)
	g.dust.step(dt)
	g.animTick++
	if g.animTick%4 == 0 && (len(l.walkers) > 0 || len(g.dust.ps) > 0 || g.dark() > 0) {
		g.dirty = true
	}
}

// seenBattleWindow is how late an attack on an observed island may be
// seen: the god who saw the wave strike it in the sky clicks a moment
// after, and the battle is played out from its start.
const seenBattleWindow = 15 * time.Second

// shownBattle is the battle of the island in view.
func (g *Game) shownBattle() *battle {
	if g.viewing != 0 {
		if g.seenOf == g.viewing {
			return &g.seenBattle
		}
		return &battle{}
	}
	return &g.battle
}

// watchSeenBattle plays out the last attack on the island observed, when
// it is recent.
func (g *Game) watchSeenBattle(st *proto.State, sum proto.IslandSummary, view *proto.Own) {
	if st.Seen == nil || st.Seen.Island != g.viewing {
		return
	}
	if g.seenOf != g.viewing {
		g.seenOf, g.seenBattle = g.viewing, battle{}
	}
	bt := st.Seen.Battle
	if bt.Tick == 0 || bt.Tick <= g.seenBattle.tick || g.seenBattle.active() {
		return
	}
	g.seenBattle.tick = bt.Tick
	if st.Tick > bt.Tick+uint64(seenBattleWindow/sim.TickDuration) {
		return // long over
	}
	g.seenBattle.start(bt, shore{geo.Outline(sum.Seed), sum.Size}, view)
}

// watchBattle starts playing out a new attack on the player's island. An
// attack older than a few seconds when first heard of is not shown: the
// player was away.
func (g *Game) watchBattle(st *proto.State) {
	if st.Own == nil {
		return
	}
	bt := st.Own.Battle
	if g.rehearse != nil {
		bt, g.rehearse = *g.rehearse, nil
		bt.Tick = max(st.Tick, g.battle.tick+1)
	}
	if bt.Tick == 0 || bt.Tick <= g.battle.tick {
		return
	}
	g.battle.tick = bt.Tick
	if st.Tick > bt.Tick+uint64(5*time.Second/sim.TickDuration) {
		return
	}
	if g.battle.active() {
		// A second attack while one is played out: it follows, rather
		// than cutting the first short.
		g.nextBattle = &bt
		return
	}
	g.startBattle(st, bt)
}

// startBattle plays out an attack on the player's island.
func (g *Game) startBattle(st *proto.State, bt sim.Battle) {
	for _, is := range st.Islands {
		if is.ID == st.You {
			g.battle.start(bt, shore{geo.Outline(is.Seed), is.Size}, st.Own)
			return
		}
	}
}

// ShowBattle plays out an attack on the player's island, as if it had just
// happened: for screenshots and for trying the scene.
func (g *Game) ShowBattle(by sim.Foe, sp sim.Species, foes int, repelled bool) {
	g.rehearse = &sim.Battle{By: by, Species: sp, Foes: foes, Repelled: repelled}
	g.lastTick = 0 // hear the current state again
}

// watchSigns raises the signs of what changed on the player's island.
func (g *Game) watchSigns(st *proto.State) {
	if st.Own == nil {
		return
	}
	for _, is := range st.Islands {
		if is.ID == st.You {
			g.signs.observe(is, st.Own)
			g.answerSigns()
			g.troops.observe(st, is, st.Own)
			if g.signs.feasts.begun {
				g.signs.feasts.begun = false
				g.play(sound.Feast)
			}
			g.troops.rehearse(g.signs.rehearse)
			return
		}
	}
}

// answerSigns passes the gatherings the signs ask for to the people.
func (g *Game) answerSigns() {
	for _, c := range g.signs.calls {
		g.life.call(c)
	}
	for _, k := range g.signs.lifts {
		g.life.lift(k)
	}
	g.signs.calls, g.signs.lifts = g.signs.calls[:0], g.signs.lifts[:0]
	g.life.slow = g.signs.starving
}

// drawStorm gathers dark clouds over the island while a wave is on its way:
// more and darker as it comes.
func (g *Game) drawStorm(atScaled func(*ebiten.Image, float64, float64, ebiten.ColorScale)) {
	in := g.signs.state.waveIn
	if in < 0 || in > 90 || len(g.clouds) == 0 {
		return
	}
	k := 1 - in/90
	n := int(math.Ceil(8 * k))
	size := math.Max(g.signs.size, 20)
	drift := float64(g.animTick) / 60
	for i := range n {
		a := 2.2 + float64(i)*0.55 + math.Sin(drift*0.2+float64(i))*0.1
		d := size * (0.55 + 0.35*float64(i%3)/2)
		x, y := g.cam.Project(art.V(d*math.Cos(a), d*math.Sin(a)*0.6, 38+float64(i%2)*6))
		c := g.clouds[i%len(g.clouds)].img
		b := c.Bounds()
		var cs ebiten.ColorScale
		cs.Scale(0.42, 0.32, 0.55, 1)
		cs.ScaleAlpha(float32(0.35 + 0.55*k))
		atScaled(c, math.Round(x)-float64(b.Dx()/2), math.Round(y)-float64(b.Dy()/2), cs)
	}
}

// SetConnection shows a message while the connection to the sky is lost,
// or hides it when msg is empty.
func (g *Game) SetConnection(msg string) {
	if msg != g.connection {
		g.connection = msg
		g.dirty = true
	}
}

// Resume carries on with a new connection to the same sky, after one was
// lost: the island, its people and the view stay as they were.
func (g *Game) Resume(src *Source) {
	g.src = src
	g.lastTick = 0
	if g.viewing != 0 {
		src.Look(g.viewing)
	}
	g.dirty = true
}

// shown returns the people of the island in view.
func (g *Game) shown() *life {
	if g.viewing != 0 {
		return &g.seen
	}
	return &g.life
}

// object is a tree, a rock or a building of the island in view, drawn apart
// from the ground so that people can pass behind it.
type object struct {
	sprite
	x, y  float64 // island units
	sways bool    // a pine, bending in the wind
}

// drawWalkers draws the island's objects and its people together, far to
// near: a villager behind a house is hidden by it.
func (g *Game) drawWalkers(at func(*ebiten.Image, float64, float64)) {
	type item struct {
		sp     sprite
		x, y   float64
		lift   float64 // pixels up, for a busy villager's bob
		z      float64 // island units above the ground, for an assailant
		person bool
		tint   *color.RGBA
		// lamp is a light, drawn at the depth of what holds it.
		lamp *lamp
		// shear leans a pine in the wind.
		shear float64
	}
	people := g.shown().walkers
	items := make([]item, 0, len(g.homeObjs)+len(people))
	// During a famine, the fields wither.
	var farms []geo.Vec2
	if st := g.src.State(); g.viewing == 0 && g.signs.starving && st != nil && st.Own != nil {
		for _, b := range st.Own.Buildings {
			if b.Kind == sim.Farm {
				farms = append(farms, geo.Vec2{X: b.X, Y: b.Y})
			}
		}
	}
	withered := color.RGBA{0xc8, 0xa0, 0x70, 0xff}
	clock := float64(g.animTick) / 60
	for _, o := range g.homeObjs {
		it := item{sp: o.sprite, x: o.x, y: o.y}
		if o.sways {
			it.shear = g.wx.sway(clock, o.x)
		}
		for _, f := range farms {
			if math.Abs(f.X-o.x) < 0.5 && math.Abs(f.Y-o.y) < 0.5 {
				it.tint = &withered
			}
		}
		items = append(items, it)
	}
	bob := (g.animTick / 12) % 2
	for _, w := range people {
		sp := g.walkerArt[0]
		if w.guard {
			sp = g.walkerArt[1]
		} else if w.child {
			sp = g.walkerArt[2]
		}
		it := item{sp: sp, x: w.x, y: w.y, person: true}
		if w.busy() && bob == 1 {
			it.lift = 1 // hammering, harvesting
		}
		t := float64(g.animTick) / 60
		switch {
		case w.at(chat) && math.Sin(t*5+w.phase) > 0.5:
			it.lift = 1 // speaking, in turn
		case w.at(sit):
			it.lift = -1 // seated by the door
		case w.state == idling && w.leisure == play && math.Sin(t*14+w.phase) > 0:
			it.lift = 1 // running, skipping
		}
		if w.gathered() {
			t := float64(g.animTick) / 60
			switch w.act {
			case hide:
				continue // sheltering at home
			case cheer:
				it.lift = math.Round(math.Abs(math.Sin(t*7+w.phase)) * 3)
			case pray:
				if math.Sin(t*2.5+w.phase) > 0 {
					it.lift = -1 // bowing
				}
			case dance:
				it.lift = math.Round(math.Abs(math.Sin(t*6+w.phase*3)) * 2) // skipping
			}
		}
		items = append(items, it)
	}
	for i, s := range g.shown().flock {
		sp := g.sheepArt[i%2]
		it := item{sp: sp, x: s.x, y: s.y, person: true}
		if s.head && (g.animTick/20+i)%3 == 0 {
			it.lift = -1 // head down in the grass
		}
		items = append(items, it)
	}
	if g.viewing == 0 {
		// The steles of the dead, in the graveyard.
		for _, p := range g.signs.graves(g.signs.dead) {
			items = append(items, item{sp: g.steleArt, x: p.X, y: p.Y})
		}
	}
	if p := g.shown().pet; p != nil {
		it := item{sp: g.dogArt, x: p.x, y: p.y, person: true}
		if p.x != p.tx || p.y != p.ty {
			it.lift = float64((g.animTick / 6) % 2) // trotting
		}
		items = append(items, it)
	}
	if g.viewing == 0 {
		// The troops coming and going, and the banners of the islands that
		// covet this one, planted on its shore.
		for _, f := range g.troops.figures(float64(g.animTick) / 60) {
			sp := g.walkerArt[1]
			switch f.kind {
			case allyFigure:
				sp = g.allyArt
			case raiderFigure:
				sp = g.foeArt[raiderArt]
			}
			items = append(items, item{sp: sp, x: f.x, y: f.y, z: f.z, person: true})
		}
		for _, b := range g.troops.banners {
			if int(b.kind) < len(g.banners) && g.banners[b.kind].img != nil {
				items = append(items, item{sp: g.banners[b.kind], x: b.at.X, y: b.at.Y})
			}
		}
	}
	if b := g.shownBattle(); b.active() {
		for _, f := range b.foes {
			if !f.down && f.art >= 0 && f.art < foeArts {
				items = append(items, item{sp: g.foeArt[f.art], x: f.x, y: f.y, z: f.z, person: true})
			}
		}
	}
	// Stable: objects keep their order among themselves; at equal depth a
	// person stands in front of the object it works at.
	dark := g.dark()
	var lamps []lamp
	if dark > 0 {
		lamps = append(append(lamps, g.homeLamps...), lanterns(people)...)
	}
	if g.viewing == 0 {
		lamps = append(lamps, g.signs.festiveLamps()...)
	}
	for i := range lamps {
		l := &lamps[i]
		// A lamp sorts just after what holds it.
		items = append(items, item{x: l.x, y: l.by + 0.001, z: l.z, person: true, lamp: l})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].y != items[j].y {
			return items[i].y < items[j].y
		}
		return !items[i].person && items[j].person
	})
	for _, it := range items {
		if it.lamp != nil {
			x, y := g.cam.Project(art.V(it.lamp.x, it.lamp.y, it.lamp.z))
			g.drawLamp(*it.lamp, math.Round(x), math.Round(y), dark, clock)
			continue
		}
		x, y := g.cam.Project(art.V(it.x, it.y, it.z))
		px, py := math.Round(x), math.Round(y)-it.lift
		if it.shear != 0 && g.atSkew != nil {
			g.atSkew(it.sp.img, px-float64(it.sp.anchor.X), py-float64(it.sp.anchor.Y), float64(it.sp.anchor.Y), it.shear)
			continue
		}
		if it.tint != nil {
			g.atTinted(it.sp.img, px-float64(it.sp.anchor.X), py-float64(it.sp.anchor.Y), *it.tint)
			continue
		}
		at(it.sp.img, px-float64(it.sp.anchor.X), py-float64(it.sp.anchor.Y))
	}
	// Particles float above everything, one virtual pixel each.
	for _, q := range g.dust.ps {
		x, y := g.cam.Project(art.V(q.x, q.y, q.z))
		px, py := math.Round(x), math.Round(y)
		g.atTinted(g.pixel, px, py, q.col)
		if q.streak {
			// A raindrop: a short line slanting with the wind.
			lean := math.Round(q.vx / 12)
			g.atTinted(g.pixel, px-lean, py-1, q.col)
			g.atTinted(g.pixel, px-2*lean, py-2, q.col)
		}
	}
	// The birds, higher still: a body and two wings, beating.
	if g.birds.visible() {
		for i := range g.birds.flock {
			bx, by, bz, up := g.birds.at(i, float64(g.animTick)/60)
			x, y := g.cam.Project(art.V(bx, by, bz))
			px, py := math.Round(x), math.Round(y)
			wing := 0.0
			if up {
				wing = -1
			}
			g.atTinted(g.pixel, px, py, birdColour)
			g.atTinted(g.pixel, px-1, py+wing, birdColour)
			g.atTinted(g.pixel, px+1, py+wing, birdColour)
		}
	}
}

package client

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"apogee/internal/proto"
	"apogee/internal/sim"
	"apogee/internal/sound"
	"apogee/internal/update"
)

// The HUD follows GDD 13.2, kept small so that the world stays in view: a
// thin bar across the top for the time, the strength and the threats; the
// pie of intentions on the left, the chronicle on the right, the reserves
// along the bottom. In the island view only the bar, the reserves and the
// latest news stay. Culture and knowledge (C) and the Annals (A) share a
// panel. The god only ever acts through the pie and the answers (GDD
// pilier 4).

var phaseNames = map[string]string{"dawn": "Aube", "day": "Jour", "dusk": "Crépuscule", "night": "Nuit"}

func clock(ms int64) string {
	s := max(ms/1000, 0)
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

// uiScale is the integer scale of the interface: the fonts stay crisp.
func (g *Game) uiScale() float64 { return math.Max(1, math.Round(g.dsfOr1())) }

// Layout of the HUD, in interface pixels.
const (
	hudMargin      = 12 // from the edges of the window
	barTop         = 10 // the top bar
	barHeight      = 32
	panelTop       = barTop + barHeight + 10 // panels start below the bar
	pieWidth       = 290
	chronicleWidth = 320
)

func (g *Game) drawHUD(screen *ebiten.Image, st *proto.State) {
	p := painter{dst: screen, k: g.uiScale(), buttons: &g.buttons}
	m := p.px(hudMargin)
	if st == nil {
		p.text("Connexion au ciel…", m, m, textColor)
		return
	}
	g.drawTopBar(p, st)
	o := st.Own
	if o == nil {
		msg := "Ton peuple attend l'aube pour renaître."
		p.styled(msg, (g.outW-p.widthOf(msg, large))/2, g.outH/2, goldColor, large)
		g.drawConnection(p)
		return
	}
	top := p.px(panelTop)
	g.areas = map[string]image.Rectangle{}
	sky := g.level == 0
	book := g.book != noBook // it takes the left side
	left := top
	if g.pieShown() && !book {
		pie := g.drawPie(p, image.Rect(m, top, m+p.px(pieWidth), top), o)
		g.areas["pie"] = pie
		left = pie.Max.Y + p.px(8)
	}
	// The pie and the outer intention wait behind the book.
	if !book {
		if r, ok := g.drawOuterStatus(p, st, o, image.Rect(m, left, m+p.px(pieWidth), left)); ok {
			g.areas["outer"] = r
		} else if sky && g.selected == 0 {
			p.styled("Clique une île pour la viser.", m, left, dimColor, body)
		}
	}

	if sky {
		if r, ok := g.drawTarget(p, st, o); ok {
			g.areas["outer"] = r
		}
	}
	alertY := top
	if g.viewing != 0 && !sky {
		alertY = g.drawObserving(p, st, top).Max.Y + p.px(8) // refusals go below
	}
	if msg := g.src.Refusal(6 * time.Second); msg != "" {
		msg = refusalText(msg)
		p.text(msg, (g.outW-p.width(msg))/2, alertY, alertColor)
	}
	right := image.Rect(g.outW-m-p.px(chronicleWidth), top, g.outW-m, top)
	if sky {
		g.areas["chronicle"] = g.drawChronicle(p, st, right, o)
	} else if g.viewing == 0 {
		g.areas["chronicle"] = g.drawNews(p, st, right, o)
	}
	g.areas["stock"] = g.drawStock(p, o)
	if o.Pending != nil {
		g.drawSupplique(p, o.Pending)
	} else if g.signs.rehearse == "supplique" {
		// For screenshots: a supplique as the people ask it in a famine.
		g.drawSupplique(p, &proto.Supplique{Question: "Les greniers sont vides. Que devons-nous faire ?",
			Options: []string{"Rationner : les soldats retournent aux champs", "Bâtir des fermes avant toute chose"}, LeftMs: 220_000})
	}
	if book {
		g.drawBook(p, st, o)
	}
	g.drawTutorial(p)
	g.drawConnection(p)
}

// part is a piece of a row of the top bar: its width, and how to draw it.
type part struct {
	w    int
	draw func(x, y int)
}

// drawTopBar draws the bar across the top: the sky and the time of day,
// the strength of the island, the threats, and the buttons of the panels.
func (g *Game) drawTopBar(p painter, st *proto.State) {
	h := p.px(barHeight)
	y := p.px(barTop)
	gap := p.px(8)
	label := func(s string, c color.Color, s2 style) part {
		return part{p.widthOf(s, s2), func(x, y int) { p.styled(s, x, y+(h-p.lineOf(s2))/2, c, s2) }}
	}
	// block draws a framed group of parts and returns where the next goes.
	block := func(x int, fill, edge color.Color, parts ...part) int {
		w := p.px(10)*2 + gap*(len(parts)-1)
		for _, pt := range parts {
			w += pt.w
		}
		r := image.Rect(x, y, x+w, y+h)
		p.fill(r, fill)
		p.frame(r, 2, edge)
		p.block(r)
		cx := x + p.px(10)
		for _, pt := range parts {
			pt.draw(cx, y)
			cx += pt.w + gap
		}
		return r.Max.X + gap
	}
	sep := part{p.px(2), func(x, y int) { p.fill(image.Rect(x, y+p.px(8), x+p.px(2), y+h-p.px(8)), panelEdge) }}
	name := g.src.Sky
	if g.level > 0 {
		name = "Ton île"
		if g.viewing != 0 {
			name = "Île observée"
		}
	}
	day := part{p.px(60), func(x, y int) {
		total := phaseLength[st.Phase]
		share := 0.0
		if total > 0 {
			share = 1 - float64(st.PhaseLeftMs)/float64(total.Milliseconds())
		}
		p.gauge(image.Rect(x, y+(h-p.px(6))/2, x+p.px(60), y+(h+p.px(6))/2), share, paleGold)
	}}
	parts := []part{label(name, goldColor, bold), phaseIcon(p, st.Phase, h), day, label(clock(st.PhaseLeftMs), dimColor, small)}
	if o := st.Own; o != nil {
		parts = append(parts, sep, label(fmt.Sprintf("Puissance %.0f", o.Power), textColor, body), label(fmt.Sprintf("Défense %.0f", o.Defense), textColor, body))
		if g.level > 0 && g.viewing == 0 {
			parts = append(parts, sep, g.pieGlimpse(p, o, h))
		}
	}
	x := block(p.px(hudMargin), panelFill, panelEdge, parts...)
	if o := st.Own; o != nil && o.WaveInMs >= 0 {
		if o.WaveInMs < 60_000 {
			x = block(x, alertFill, alertColor, label("Une vague dans", alertText, body), label(clock(o.WaveInMs), textColor, bold))
		} else {
			x = block(x, panelFill, panelEdge, label("Une vague dans", dimColor, body), label(clock(o.WaveInMs), textColor, body))
		}
	}
	if st.BossInMs >= 0 {
		if st.BossInMs < 5*60_000 {
			block(x, alertFill, alertColor, label("Colosse dans", alertText, body), label(clock(st.BossInMs), textColor, bold))
		} else {
			block(x, panelFill, panelEdge, label("Colosse dans", dimColor, body), label(clock(st.BossInMs), textColor, body))
		}
	}
	// The buttons, from the right edge.
	bx := g.outW - p.px(hudMargin)
	square := func(label string, on bool, action func()) {
		w := max(h, p.widthOf(label, body)+p.px(16))
		r := image.Rect(bx-w, y, bx, y+h)
		kind := plainButton
		if on {
			kind = primaryButton
		}
		p.buttonAs(r, label, action, kind, panelEdge)
		bx -= w + p.px(4)
	}
	sound := "Son"
	if g.Sound != nil && g.Sound.Muted() {
		sound = "Muet"
	}
	var mute func()
	if g.Sound != nil {
		mute = g.ToggleMute
	}
	if g.OnHome != nil {
		square("Accueil", false, g.OnHome)
	}
	square(sound, false, mute)
	square("A", g.book == annalsTab, func() { g.togglePanel(annalsTab) })
	square("C", g.book == cultureTab, func() { g.togglePanel(cultureTab) })
	square("S", g.pieShown() && g.book == noBook, g.togglePie)
	square("I", g.book == civTab, func() { g.togglePanel(civTab) })
	if g.level > 0 && g.viewing == 0 {
		square("‹ Ciel", false, func() { g.SetZoom(0) })
	}
	if r := g.Newer; r != nil {
		label := "Version " + r.Version + " disponible"
		w := p.width(label) + p.px(20)
		p.fill(image.Rect(bx-w, y, bx, y+h), panelFill) // readable over the sky
		p.buttonAs(image.Rect(bx-w, y, bx, y+h), label, func() { _ = update.Open(r.URL) }, ghostButton, aetherColor)
	}
}

// phaseIcon draws the sun, or the moon at night, in pixels.
func phaseIcon(p painter, phase string, h int) part {
	n := p.px(1)
	return part{p.px(12), func(x, y int) {
		cy := y + (h-p.px(12))/2
		dot := func(dx, dy, w, hh int, c color.Color) {
			p.fill(image.Rect(x+dx*n, cy+dy*n, x+(dx+w)*n, cy+(dy+hh)*n), c)
		}
		switch phase {
		case "night":
			dot(3, 1, 6, 10, textColor)
			dot(6, 0, 6, 9, panelFill)
			dot(7, 0, 5, 9, barBack)
		default:
			c := paleGold
			if phase != "day" {
				c = alertText // dawn and dusk: a low, warm sun
			}
			dot(3, 3, 6, 6, c)
			dot(5, 0, 2, 2, c)
			dot(5, 10, 2, 2, c)
			dot(0, 5, 2, 2, c)
			dot(10, 5, 2, 2, c)
		}
	}}
}

// pieGlimpse is the pie as a small bar in the top bar of the island view;
// a click opens the pie to change it.
func (g *Game) pieGlimpse(p painter, o *proto.Own, h int) part {
	w := p.px(80)
	return part{w, func(x, y int) {
		r := image.Rect(x, y+(h-p.px(8))/2, x+w, y+(h+p.px(8))/2)
		p.fill(r, barBack)
		cx := r.Min.X
		for i := range sim.NumIntentions {
			if v := o.Pie[i]; v > 0 {
				dw := w * v / 100
				p.fill(image.Rect(cx, r.Min.Y, cx+dw-p.px(1), r.Max.Y), intentionColor[i])
				cx += dw
			}
		}
		*p.buttons = append(*p.buttons, button{r: image.Rect(x, y, x+w, y+h), action: g.togglePie})
	}}
}

// intentionColor tells the intentions apart in the glimpse of the pie.
var intentionColor = [sim.NumIntentions]color.Color{
	goldColor, aetherColor, color.RGBA{0xb9, 0xb4, 0xd6, 0xff}, alertColor, color.RGBA{0xb4, 0x8c, 0xff, 0xff},
}

// pieShown reports whether the pie is on screen: by default in the sky
// view, on demand in the island view.
func (g *Game) pieShown() bool {
	if g.level == 0 {
		return !g.pieHidden
	}
	return g.pieOpen
}

// togglePie shows or hides the pie, in the view at hand; it closes the
// book panel, which takes the same place.
func (g *Game) togglePie() {
	if g.level == 0 {
		g.pieHidden = !g.pieHidden
	} else {
		g.pieOpen = !g.pieOpen
	}
	if g.pieShown() {
		g.book = noBook
	}
	g.dirty = true
}

// bookTab is a tab of the book panel, on the left.
type bookTab int

const (
	noBook     bookTab = iota
	civTab             // the state of the civilization (civ.go)
	cultureTab         // culture and knowledge
	annalsTab          // the Annals of the sky
)

// togglePanel opens a tab of the book panel, or closes it if it shows.
func (g *Game) togglePanel(tab bookTab) {
	if g.book == tab {
		g.book = noBook
	} else {
		g.book = tab
	}
	g.dirty = true
}

// drawConnection tells the player the connection to the sky is lost, while
// the app tries again, and offers to go back home.
func (g *Game) drawConnection(p painter) {
	if g.connection == "" {
		return
	}
	bh := p.px(30)
	w := max(p.width(g.connection)+p.px(24), p.px(340))
	h := p.line() + bh + p.px(28)
	top := p.px(panelTop)
	r := image.Rect((g.outW-w)/2, top, (g.outW+w)/2, top+h)
	in := p.panelEdged(r, alertColor)
	p.text(g.connection, in.Min.X+(in.Dx()-p.width(g.connection))/2, in.Min.Y, alertText)
	y := in.Max.Y - bh
	bw := p.width("Revenir à l'accueil") + p.px(24)
	p.button(image.Rect(in.Min.X+(in.Dx()-bw)/2, y, in.Min.X+(in.Dx()+bw)/2, y+bh), "Revenir à l'accueil", g.OnGiveUp)
}

// drawPie draws the pie of intentions the player composes and whispers
// (GDD 6.2): one line per intention, its share in tenths, and the people's
// actual conviction under it.
func (g *Game) drawPie(p painter, r image.Rectangle, o *proto.Own) image.Rectangle {
	rows := int(sim.NumIntentions)
	row := p.px(24)
	bh := p.px(28)
	slices := 0
	for _, v := range g.draft {
		if v > 0 {
			slices++
		}
	}
	// With three intentions, a fourth needs room first: say so, since
	// the + of the others is then off.
	var full []string
	if slices >= sim.MaxSlices {
		full = p.wrap("Trois intentions au plus : baisse-en une à 0 % pour en souffler une autre.", r.Dx()-p.px(20))
	}
	r.Max.Y = r.Min.Y + p.px(20) + p.line() + p.px(6) + rows*row + p.px(6) + bh + len(full)*p.line()
	if len(full) > 0 {
		r.Max.Y += p.px(6)
	}
	in := p.panel(r)
	p.styled("Souffles", in.Min.X, in.Min.Y, goldColor, bold)
	count := fmt.Sprintf("%d / %d", slices, sim.MaxSlices)
	p.styled(count, in.Max.X-p.widthOf(count, small), in.Min.Y+(p.line()-p.lineOf(small))/2, dimColor, small)
	y := in.Min.Y + p.line() + p.px(6)
	nameW := p.px(96)
	btn := p.px(20)
	pctW := p.px(40)
	for i := range sim.NumIntentions {
		v := g.draft[i]
		col := color.Color(dimColor)
		if v > 0 {
			col = textColor
		}
		p.text(i.Name(), in.Min.X, y+(row-p.line())/2, col)
		right := in.Max.X
		cy := y + (row-btn)/2
		p.button(image.Rect(right-btn, cy, right, cy+btn), "+", g.adjust(i, +10))
		pct := fmt.Sprintf("%d%%", v)
		p.styled(pct, right-btn-pctW+(pctW-p.widthOf(pct, small))/2, y+(row-p.lineOf(small))/2, textColor, small)
		minus := right - btn - pctW - btn
		p.button(image.Rect(minus, cy, minus+btn, cy+btn), "−", g.adjust(i, -10))
		// Ten notches for the share, a thread of gold for the conviction.
		bx0, bx1 := in.Min.X+nameW, minus-p.px(6)
		by := y + (row-p.px(9))/2
		cell := (bx1 - bx0 + p.px(1)) / 10
		for c := range 10 {
			fill := buttonFill
			if c < v/10 {
				fill = goldColor
			}
			p.fill(image.Rect(bx0+c*cell, by, bx0+(c+1)*cell-p.px(1), by+p.px(6)), fill)
		}
		p.gauge(image.Rect(bx0, by+p.px(7), bx0+10*cell-p.px(1), by+p.px(9)), o.Conviction[i], paleGold)
		y += row
	}
	y += p.px(6)
	var whisper func()
	if g.drafting && g.draft.Validate() == nil && g.draft != o.Pie {
		whisper = func() {
			g.src.Whisper(g.draft)
			g.drafting = false
			g.play(sound.Whisper)
			g.signs.whispered()
		}
	}
	for _, l := range full {
		p.text(l, in.Min.X, y, dimColor)
		y += p.line()
	}
	if len(full) > 0 {
		y += p.px(6)
	}
	split := in.Min.X + (in.Dx()-p.px(6))*2/3
	p.buttonAs(image.Rect(in.Min.X, y, split, y+bh), "Souffler", whisper, primaryButton, panelEdge)
	var cancel func()
	if g.drafting {
		cancel = func() {
			g.draft = o.Pie
			g.drafting = false
		}
	}
	p.buttonAs(image.Rect(split+p.px(6), y, in.Max.X, y+bh), "Annuler", cancel, ghostButton, panelEdge)
	return r
}

// drawObserving shows whose island the god is looking at, and the way back.
func (g *Game) drawObserving(p painter, st *proto.State, top int) image.Rectangle {
	name := "une île disparue"
	for _, is := range st.Islands {
		if is.ID == g.viewing {
			name = is.Name
			if is.AI {
				name += " (IA)"
			}
		}
	}
	label := "Tu observes l'île de " + name
	back := "Revenir à mon île (Échap)"
	bh := p.px(28)
	w := max(p.widthOf(label, bold), p.width(back)) + p.px(40)
	r := image.Rect((g.outW-w)/2, top, (g.outW+w)/2, top+p.line()+bh+p.px(26))
	in := p.panelEdged(r, aetherColor)
	p.styled(label, in.Min.X+(in.Dx()-p.widthOf(label, bold))/2, in.Min.Y, aetherColor, bold)
	p.buttonAs(image.Rect(in.Min.X, in.Max.Y-bh, in.Max.X, in.Max.Y), back, func() { g.Observe(0) }, ghostButton, aetherColor)
	return r
}

// refusals translates the server's refusals for the player.
var refusals = map[string]string{
	"this young civilisation is still protected":   "Cette jeune civilisation est encore protégée.",
	"only a declining island can be annihilated":   "Seule une île en déclin peut être anéantie.",
	"this island was pillaged by you a moment ago": "Ton peuple vient de piller cette île : il faut attendre.",
	"a people cannot raid its own island":          "Un peuple ne peut pas attaquer sa propre île.",
	"no such island":                               "Cette île n'existe plus.",
	"no living island":                             "Ton peuple attend l'aube pour renaître.",
}

func refusalText(msg string) string {
	if t, ok := refusals[msg]; ok {
		return t
	}
	return "Le ciel refuse : " + msg
}

func islandName(st *proto.State, id proto.IslandID) string {
	for _, is := range st.Islands {
		if is.ID == id {
			return is.Name
		}
	}
	return "une île disparue"
}

// drawOuterStatus tells, under the pie, the intention turned towards
// another island, with the way to call it off (GDD 6.3).
func (g *Game) drawOuterStatus(p painter, st *proto.State, o *proto.Own, r image.Rectangle) (image.Rectangle, bool) {
	away := false
	for _, rd := range st.Raids {
		if rd.From == st.You {
			away = true
		}
	}
	if o.Outer == 0 && !away {
		return r, false
	}
	bh := p.px(26)
	r.Max.Y = r.Min.Y + p.px(20) + p.line() + p.px(6) + bh
	in := p.panel(r)
	status := "Tes guerriers sont en route."
	if o.Outer != 0 {
		status = fmt.Sprintf("%s : %s", o.Outer.Name(), islandName(st, o.OuterTarget))
	}
	p.styled(status, in.Min.X, in.Min.Y, goldColor, body)
	p.buttonAs(image.Rect(in.Min.X, in.Max.Y-bh, in.Max.X, in.Max.Y), "Rappeler", func() { g.src.Aim(sim.NoOuter, 0) }, ghostButton, panelEdge)
	return r, true
}

// drawTarget draws, by the island selected in the sky view, what the god
// may whisper towards it.
func (g *Game) drawTarget(p painter, st *proto.State, o *proto.Own) (image.Rectangle, bool) {
	var target *proto.IslandSummary
	for i := range st.Islands {
		if st.Islands[i].ID == g.selected {
			target = &st.Islands[i]
		}
	}
	if target == nil || g.targetAt.Empty() {
		return image.Rectangle{}, false
	}
	// The island is framed in gold.
	p.frame(g.targetAt.Inset(-p.px(4)), 2, goldColor)
	w := p.px(250)
	bh := p.px(28)
	h := p.px(20) + 3*p.line() + p.px(10) + 3*(bh+p.px(4)) - p.px(4)
	x := g.targetAt.Max.X + p.px(12)
	if x+w > g.outW-p.px(hudMargin) {
		x = g.targetAt.Min.X - p.px(12) - w // no room on the right
	}
	y := min(max(g.targetAt.Min.Y, p.px(panelTop)), g.outH-h-p.px(50))
	r := image.Rect(x, y, x+w, y+h)
	in := p.panelEdged(r, goldColor)
	name := target.Name
	if target.AI {
		name += " (IA)"
	}
	p.styled(name, in.Min.X, in.Min.Y, textColor, bold)
	figures := fmt.Sprintf("P %.0f · cœur %d%%", target.Power, int(math.Round(target.Heart*100)))
	p.styled(figures, in.Max.X-p.widthOf(figures, small), in.Min.Y+(p.line()-p.lineOf(small))/2, dimColor, small)
	ty := in.Min.Y + p.line()
	culture := min(max(target.Culture, 0), int(sim.NumFamilies)-1)
	p.text("Vénère "+familyArticle(sim.Family(culture)), in.Min.X, ty, familyColor[culture])
	ty += p.line()
	switch {
	case target.Banner != 0 && target.BannerTarget == st.You && target.Banner != sim.Rescue:
		p.text("Te menace de sa bannière.", in.Min.X, ty, alertText)
	case target.Protected:
		p.text("Protégée encore "+countdown(target.ProtectedMs)+".", in.Min.X, ty, aetherColor)
	case target.Declining:
		p.text("En déclin.", in.Min.X, ty, alertText)
	default:
		p.text("Rien ne vous oppose.", in.Min.X, ty, dimColor)
	}
	ty += p.line() + p.px(10)
	id := target.ID
	aim := func(kind sim.Outer, ok bool) func() {
		if !ok {
			return nil
		}
		return func() {
			g.src.Aim(kind, id)
			g.play(sound.Whisper)
		}
	}
	raidable := !target.Protected
	half := (in.Dx() - p.px(4)) / 2
	cell := func(col, row int, label string, action func()) {
		x0 := in.Min.X + col*(half+p.px(4))
		y0 := ty + row*(bh+p.px(4))
		p.button(image.Rect(x0, y0, x0+half, y0+bh), label, action)
	}
	// Punir leaves at once and Secourir lends spare defenders: both need
	// someone to send. Convoiter and Anéantir prepare first.
	cell(0, 0, "Convoiter", aim(sim.Covet, raidable))
	cell(1, 0, "Punir", aim(sim.Punish, raidable && o.Assailants >= 1))
	cell(0, 1, "Anéantir", aim(sim.Annihilate, raidable && target.Declining))
	cell(1, 1, "Secourir", aim(sim.Rescue, o.Defenders >= 3))
	oy := ty + 2*(bh+p.px(4))
	p.buttonAs(image.Rect(in.Min.X, oy, in.Max.X, oy+bh), "Observer de près", func() { g.Observe(id) }, ghostButton, aetherColor)
	return r, true
}

// adjust returns the action moving 10 % of the pie to or from intention i,
// keeping the pie whole: the slice comes from, or goes to, the largest other
// slice. It returns nil when the move is impossible.
func (g *Game) adjust(i sim.Intention, d int) func() {
	next := g.draft
	largest := sim.Intention(-1)
	for j := range sim.NumIntentions {
		if j != i && next[j] > 0 && (largest < 0 || next[j] > next[largest]) {
			largest = j
		}
	}
	switch {
	case d > 0 && largest >= 0:
		next[i] += 10
		next[largest] -= 10
	case d < 0 && next[i] >= 10 && largest >= 0:
		next[i] -= 10
		next[largest] += 10
	default:
		return nil
	}
	if next.Validate() != nil {
		return nil // a fourth slice, most likely
	}
	return func() {
		g.draft = next
		g.drafting = true
	}
}

// newsColor marks each kind of entry of the chronicle.
func newsColor(k sim.EntryKind) color.Color {
	switch k {
	case sim.RaidNews, sim.WaveNews:
		return alertColor
	case sim.VictoryNews, sim.BirthNews:
		return aetherColor
	case sim.DeclineNews, sim.FallNews:
		return familyColor[3]
	case sim.BuiltNews, sim.DiscoveryNews, sim.SuppliqueNews:
		return goldColor
	}
	return dimColor
}

// countdown tells a time left to the minute: 5 h 12, 47 min.
func countdown(ms int64) string {
	m := max((ms+59_999)/60_000, 1)
	if m < 60 {
		return fmt.Sprintf("%d\u00a0min", m)
	}
	return fmt.Sprintf("%d\u00a0h\u00a0%02d", m/60, m%60)
}

// ago tells how long ago an entry was written.
func ago(st *proto.State, e sim.Entry) string {
	if e.Tick >= st.Tick {
		return "à l'instant"
	}
	d := time.Duration(st.Tick-e.Tick) * sim.TickDuration
	switch {
	case d < time.Minute:
		return "à l'instant"
	case d < time.Hour:
		return fmt.Sprintf("%d\u00a0min", int(d.Minutes()))
	}
	return fmt.Sprintf("%d\u00a0h", int(d.Hours()))
}

// plural counts people of a kind, as French says it: one is singular, and
// so is none.
func plural(n float64, word string) string {
	v := int(math.Round(n))
	if v > 1 {
		word += "s"
	}
	return fmt.Sprintf("%d %s", v, word)
}

// chronicleShort is how many entries the chronicle shows, unless the god
// asks for all of them.
const chronicleShort = 3

// drawChronicle draws the latest entries of the chronicle, newest first:
// three, or as many as fit when the god reads it all.
func (g *Game) drawChronicle(p painter, st *proto.State, r image.Rectangle, o *proto.Own) image.Rectangle {
	inW := r.Dx() - p.px(20)
	textW := inW - p.px(12)
	type entry struct {
		lines []string
		e     sim.Entry
	}
	var entries []entry
	h := p.line() + p.px(6)
	limit := g.outH - p.px(120) - r.Min.Y
	for i := len(o.Chronicle) - 1; i >= 0; i-- {
		if !g.chronicleAll && len(entries) == chronicleShort {
			break
		}
		if g.isHeld(o.Chronicle[i]) {
			continue // the battle is still being fought
		}
		lines := p.wrap(o.Chronicle[i].Text+" · "+ago(st, o.Chronicle[i]), textW)
		eh := len(lines)*p.line() + p.px(4)
		if h+eh > limit {
			break // only whole entries
		}
		entries = append(entries, entry{lines, o.Chronicle[i]})
		h += eh
	}
	r.Max.Y = r.Min.Y + h + p.px(20)
	in := p.panel(r)
	p.styled("Chronique", in.Min.X, in.Min.Y, goldColor, bold)
	more := "Tout lire"
	if g.chronicleAll {
		more = "Réduire"
	}
	mw := p.widthOf(more, small)
	mr := image.Rect(in.Max.X-mw-p.px(4), in.Min.Y, in.Max.X, in.Min.Y+p.line())
	p.styled(more, in.Max.X-mw, in.Min.Y+(p.line()-p.lineOf(small))/2, dimColor, small)
	*p.buttons = append(*p.buttons, button{r: mr, action: func() { g.chronicleAll = !g.chronicleAll }})
	y := in.Min.Y + p.line() + p.px(6)
	for i, e := range entries {
		c := color.Color(textColor)
		if i > 0 {
			c = color.NRGBA{0xf2, 0xee, 0xe3, uint8(max(255-i*40, 150))}
		}
		p.dot(in.Min.X, y+(p.line()-p.px(6))/2, 6, newsColor(e.e.Kind))
		for _, l := range e.lines {
			p.text(l, in.Min.X+p.px(12), y, c)
			y += p.line()
		}
		y += p.px(4)
	}
	return r
}

// drawNews shows, in the island view, the latest entry of the chronicle.
func (g *Game) drawNews(p painter, st *proto.State, r image.Rectangle, o *proto.Own) image.Rectangle {
	i := len(o.Chronicle) - 1
	for i >= 0 && g.isHeld(o.Chronicle[i]) {
		i-- // the battle is still being fought
	}
	if i < 0 {
		return image.Rectangle{}
	}
	e := o.Chronicle[i]
	lines := p.wrap(e.Text, r.Dx()-p.px(32))
	if len(lines) > 3 {
		lines = lines[:3]
	}
	r.Max.Y = r.Min.Y + len(lines)*p.line() + p.px(16)
	p.fill(r, panelFill)
	p.frame(r, 2, panelEdge)
	p.block(r)
	p.dot(r.Min.X+p.px(10), r.Min.Y+p.px(8)+(p.line()-p.px(6))/2, 6, newsColor(e.Kind))
	for i, l := range lines {
		p.text(l, r.Min.X+p.px(22), r.Min.Y+p.px(8)+i*p.line(), textColor)
	}
	return r
}

// drawStock draws the reserves along the bottom, each with its gauge
// against the cap, and the people.
func (g *Game) drawStock(p painter, o *proto.Own) image.Rectangle {
	h := p.px(30)
	gap := p.px(14)
	names := [...]string{"Bois", "Pierre", "Vivres", "Aether"}
	res := [...]sim.Resource{sim.Wood, sim.Stone, sim.Food, sim.Aether}
	people := fmt.Sprintf("%.0f/%d âmes", o.Population, o.Housing)
	soldiers := plural(o.Defenders, "défenseur")
	if o.Assailants >= 1 {
		soldiers += " · " + plural(o.Assailants, "guerrier")
	}
	w := p.px(20)
	for i, r := range res {
		w += p.px(12) + p.width(names[i]) + p.px(4) + p.widthOf(fmt.Sprintf("%.0f", o.Stock[r]), bold) + p.px(6) + p.px(24) + gap
	}
	w += p.px(2) + gap + p.width(people) + gap + p.width(soldiers)
	y := g.outH - p.px(10) - h
	box := image.Rect(p.px(hudMargin), y, p.px(hudMargin)+w, y+h)
	p.fill(box, panelFill)
	p.frame(box, 2, panelEdge)
	p.block(box)
	x := box.Min.X + p.px(10)
	ty := y + (h-p.line())/2
	for i, r := range res {
		p.dot(x, y+(h-p.px(6))/2, 6, resourceColor[i])
		x += p.px(12)
		p.text(names[i], x, ty, dimColor)
		x += p.width(names[i]) + p.px(4)
		v := fmt.Sprintf("%.0f", o.Stock[r])
		p.styled(v, x, ty, textColor, bold)
		x += p.widthOf(v, bold) + p.px(6)
		share := 0.0
		if o.Cap > 0 {
			share = o.Stock[r] / o.Cap
		}
		p.gauge(image.Rect(x, y+(h-p.px(4))/2, x+p.px(24), y+(h+p.px(4))/2), share, resourceColor[i])
		x += p.px(24) + gap
	}
	p.fill(image.Rect(x, y+p.px(8), x+p.px(2), y+h-p.px(8)), panelEdge)
	x += p.px(2) + gap
	p.text(people, x, ty, textColor)
	x += p.width(people) + gap
	p.text(soldiers, x, ty, dimColor)
	return box
}

// drawSupplique draws the question of the people and its answers
// (GDD 6.5). Unanswered, the people decide alone when the time runs out.
func (g *Game) drawSupplique(p painter, s *proto.Supplique) {
	w := p.px(460)
	inW := w - p.px(20)
	q := p.wrap(s.Question, inW)
	bh := p.px(30)
	cols := 2
	if len(s.Options) == 1 || len(s.Options) > 4 {
		cols = 1
	}
	// Each answer in its column, on as many lines as it needs.
	colW := (inW - p.px(6)*(cols-1)) / cols
	rows, lines := (len(s.Options)+cols-1)/cols, 1
	for _, opt := range s.Options {
		lines = max(lines, len(p.wrap(opt, colW-p.px(12))))
	}
	bh = max(bh, lines*p.line()+p.px(10))
	h := p.px(20) + p.line() + p.px(10) + len(q)*p.line() + p.px(8) + rows*(bh+p.px(6)) - p.px(6)
	bottom := g.outH - p.px(50)
	r := image.Rect((g.outW-w)/2, bottom-h, (g.outW+w)/2, bottom)
	in := p.panelEdged(r, goldColor)
	p.styled("Ton peuple s'adresse à toi", in.Min.X, in.Min.Y, goldColor, bold)
	left := "seul dans " + clock(s.LeftMs)
	p.styled(left, in.Max.X-p.widthOf(left, small), in.Min.Y+(p.line()-p.lineOf(small))/2, dimColor, small)
	y := in.Min.Y + p.line() + p.px(3)
	p.gauge(image.Rect(in.Min.X, y, in.Max.X, y+p.px(3)), float64(s.LeftMs)/1000/sim.SuppliqueTimeout, goldColor)
	y += p.px(7)
	for _, l := range q {
		p.text(l, in.Min.X, y, textColor)
		y += p.line()
	}
	y += p.px(8)
	for i, opt := range s.Options {
		id, option := s.ID, i
		x0 := in.Min.X + (i%cols)*(colW+p.px(6))
		y0 := y + (i/cols)*(bh+p.px(6))
		br := image.Rect(x0, y0, x0+colW, y0+bh)
		p.fill(br, buttonFill)
		p.frame(br, 1, panelEdge)
		ol := p.wrap(opt, colW-p.px(12))
		for j, l := range ol {
			p.text(l, x0+p.px(6), y0+(bh-len(ol)*p.line())/2+j*p.line(), textColor)
		}
		*p.buttons = append(*p.buttons, button{r: br, action: func() {
			g.src.Answer(id, option)
			g.play(sound.Answer)
		}})
	}
}

// bookLine is a line of the book panel: its height, and how to draw it.
type bookLine struct {
	h    int
	draw func(y int)
}

// drawBook is the panel of culture and knowledge, and of the Annals, on
// the left, beside the world rather than over it.
func (g *Game) drawBook(p painter, st *proto.State, o *proto.Own) {
	m := p.px(hudMargin)
	top := p.px(panelTop)
	w := min(p.px(560), g.outW-2*m)
	tabH := p.px(28)
	inW := w - p.px(20)
	x0 := m + p.px(10)
	var lines []bookLine
	switch g.book {
	case civTab:
		lines = g.civLines(p, st, o, x0, inW)
	case cultureTab:
		total := 0.0
		for _, v := range o.Culture {
			total += v
		}
		for f := range sim.NumFamilies {
			h := p.px(26)
			lines = append(lines, bookLine{h, func(y int) {
				ty := y + (h-p.line())/2
				p.styled(f.Name(), x0, ty, familyColor[f], bold)
				px := x0 + p.px(92)
				for t := range sim.TiersPerAxis {
					c := color.Color(panelDim)
					if t < o.Tiers[f] {
						c = familyColor[f]
					}
					p.dot(px+t*p.px(10), y+(h-p.px(8))/2, 8, c)
				}
				share, progress := 0.0, 0.0
				if total > 0 {
					share = o.Culture[f] / total
				}
				note := "en sommeil"
				noteC := color.Color(dimColor)
				if int(f) == o.Axis {
					progress, note, noteC = o.Progress, "les savants y travaillent", textColor
				}
				gx := px + p.px(10)*sim.TiersPerAxis + p.px(10)
				gw := x0 + inW - p.width("les savants y travaillent") - p.px(10) - gx
				p.gauge(image.Rect(gx, y+(h-p.px(10))/2, gx+gw, y+(h-p.px(10))/2+p.px(6)), share, familyColor[f])
				p.gauge(image.Rect(gx, y+(h-p.px(10))/2+p.px(8), gx+gw, y+(h-p.px(10))/2+p.px(10)), progress, textColor)
				p.text(note, gx+gw+p.px(10), ty, noteC)
			}})
		}
		found := "Aucune découverte encore."
		if len(o.Discoveries) > 0 {
			found = "Découvertes : " + strings.Join(o.Discoveries, ", ") + "."
		}
		text := p.wrap(found, inW)
		lines = append(lines, bookLine{p.px(10) + len(text)*p.line(), func(y int) {
			for i, l := range text {
				p.text(l, x0, y+p.px(10)+i*p.line(), dimColor)
			}
		}})
	case annalsTab:
		if len(st.Annals) == 0 {
			lines = append(lines, bookLine{p.line(), func(y int) {
				p.text("Aucune civilisation n'est encore tombée.", x0, y, dimColor)
			}})
		}
		for i := len(st.Annals) - 1; i >= 0 && len(lines) < 8; i-- {
			a := st.Annals[i]
			fam := min(max(a.Culture, 0), int(sim.NumFamilies)-1)
			text := p.wrap(fmt.Sprintf("%.1f jours · apogée %.0f · %d paliers · %s", a.DaysLived, a.PeakPower, a.Tiers, a.Cause), inW)
			lines = append(lines, bookLine{(len(text)+1)*p.line() + p.px(6), func(y int) {
				p.styled(fmt.Sprintf("%s, génération %d", a.Name, a.Generation), x0, y, familyColor[fam], bold)
				for j, l := range text {
					p.text(l, x0, y+(j+1)*p.line(), dimColor)
				}
			}})
		}
	}
	h := 0
	for _, l := range lines {
		h += l.h
	}
	r := image.Rect(m, top, m+w, top+p.px(20)+tabH+p.px(10)+h)
	in := p.panel(r)
	tab := func(x int, label string, t bookTab) int {
		tw := p.widthOf(label, bold) + p.px(20)
		kind := ghostButton
		if g.book == t {
			kind = primaryButton
		}
		p.buttonAs(image.Rect(x, in.Min.Y, x+tw, in.Min.Y+tabH), label, func() { g.book = t }, kind, panelEdge)
		return x + tw + p.px(4)
	}
	x := tab(in.Min.X, "Civilisation", civTab)
	x = tab(x, "Culture et savoir", cultureTab)
	tab(x, "Annales", annalsTab)
	p.buttonAs(image.Rect(in.Max.X-tabH, in.Min.Y, in.Max.X, in.Min.Y+tabH), "×", func() { g.book = noBook }, ghostButton, panelEdge)
	y := in.Min.Y + tabH + p.px(10)
	for _, l := range lines {
		l.draw(y)
		y += l.h
	}
}

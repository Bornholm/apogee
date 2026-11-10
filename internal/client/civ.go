package client

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"apogee/internal/proto"
	"apogee/internal/sim"
)

// The civilization tab of the book panel (I) tells where the people stand
// (GDD 13.2): their age and their way, how far the island has sunk towards
// the whirlpool and when it will decline, its heart, people and soldiers,
// its buildings and sites, its reserves and how they move, the threats.

// stockSample is the reserves at a moment, to tell how they move.
type stockSample struct {
	t time.Time
	s sim.Stock
}

// Reserves are sampled every so often over a span, for their trend.
const (
	stockEvery = 15 * time.Second
	stockSpan  = 15 * time.Minute
	stockMin   = 2 * time.Minute // shorter, a trend means nothing
)

// logStock notes the reserves of a new state.
func (g *Game) logStock(o *proto.Own, now time.Time) {
	if o == nil {
		return
	}
	if n := len(g.stockLog); n > 0 && now.Sub(g.stockLog[n-1].t) < stockEvery {
		return
	}
	g.stockLog = append(g.stockLog, stockSample{now, o.Stock})
	for len(g.stockLog) > 1 && now.Sub(g.stockLog[0].t) > stockSpan {
		g.stockLog = g.stockLog[1:]
	}
}

// trend returns how a reserve moved over the samples, per hour, and
// whether there are enough of them to say.
func (g *Game) trend(r sim.Resource) (float64, bool) {
	if len(g.stockLog) < 2 {
		return 0, false
	}
	first, last := g.stockLog[0], g.stockLog[len(g.stockLog)-1]
	d := last.t.Sub(first.t)
	if d < stockMin {
		return 0, false
	}
	return (last.s[r] - first.s[r]) / d.Hours(), true
}

var ordinals = []string{"", "Première", "Deuxième", "Troisième", "Quatrième", "Cinquième", "Sixième", "Septième", "Huitième", "Neuvième", "Dixième"}

// generation names a generation.
func generation(n int) string {
	if n > 0 && n < len(ordinals) {
		return ordinals[n] + " génération"
	}
	return fmt.Sprintf("Génération %d", n)
}

// span tells a duration as a person would, roughly.
func span(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "moins d'une minute"
	case d < time.Hour:
		return plural(math.Round(d.Minutes()), "minute")
	case d < 24*time.Hour:
		return plural(math.Floor(d.Hours()), "heure")
	}
	days := math.Floor(d.Hours() / 24)
	hours := math.Floor(d.Hours() - days*24)
	if hours == 0 {
		return plural(days, "jour")
	}
	return plural(days, "jour") + " et " + plural(hours, "heure")
}

// buildingLabels name the kinds of buildings in the plural, for the grid.
var buildingLabels = [sim.NumKinds]string{
	sim.Core: "Cœur", sim.House: "Maisons", sim.Farm: "Fermes", sim.Woodcutter: "Cabanes", sim.Quarry: "Carrières",
	sim.Extractor: "Extract.", sim.Warehouse: "Dépôts", sim.Tower: "Tours", sim.Barracks: "Casernes", sim.Library: "Biblio.",
}

// civLines builds the lines of the civilization tab.
func (g *Game) civLines(p painter, st *proto.State, o *proto.Own, x0, w int) []bookLine {
	var sum proto.IslandSummary
	for _, is := range st.Islands {
		if is.ID == st.You {
			sum = is
		}
	}
	var lines []bookLine
	gap := p.px(10)
	label := func(s string, y int) { p.styled(s, x0, y, goldColor, bold) }

	// Who they are, and how they fare.
	age := time.Duration(o.Age) * sim.TickDuration
	worship := 0
	for f := range sim.NumFamilies {
		if o.Culture[f] > o.Culture[worship] {
			worship = int(f)
		}
	}
	state, stateC := "En ascension", aetherColor
	switch {
	case sum.FallingMs > 0:
		state, stateC = "En chute", alertColor
	case sum.Declining:
		state, stateC = "En déclin", familyColor[3]
	case o.Starving:
		state, stateC = "Famine", alertColor
	}
	var trait []string
	if t := sum.Trait; t.Name() != "" {
		trait = p.wrap(t.Name()+" : "+t.Text(), w)
	}
	var relic []string
	if r := sum.Relic; r.Name() != "" {
		relic = p.wrap("Relique : "+r.Name()+", "+r.Text()+".", w)
	}
	civ := sum.CivName
	if civ == "" {
		civ = generation(o.Generation)
	}
	lineage := strings.ToLower(generation(o.Generation)[:1]) + generation(o.Generation)[1:]
	if l := sum.Lineage.Name; l != "" {
		lineage = l + " · " + lineage
	}
	lines = append(lines, bookLine{(3+len(trait)+len(relic))*p.line() + gap, func(y int) {
		if g.renaming {
			field := image.Rect(x0, y-p.px(2), x0+w-p.px(170), y+p.line()+p.px(2))
			p.fill(field, barBack)
			p.frame(field, 1, goldColor)
			text := string(g.newName)
			if (time.Now().UnixMilli()/500)%2 == 0 {
				text += "_"
			}
			p.styled(text, field.Min.X+p.px(6), y, textColor, bold)
			p.buttonAs(image.Rect(field.Max.X+p.px(6), field.Min.Y, field.Max.X+p.px(80), field.Max.Y), "Nommer", g.confirmName, primaryButton, panelEdge)
			p.buttonAs(image.Rect(field.Max.X+p.px(86), field.Min.Y, x0+w, field.Max.Y), "Annuler", func() { g.renaming = false }, ghostButton, panelEdge)
		} else {
			p.styled(civ, x0, y, textColor, bold)
			rw := p.width("Renommer") + p.px(16)
			rx := x0 + p.widthOf(civ, bold) + p.px(10)
			p.buttonAs(image.Rect(rx, y-p.px(1), rx+rw, y+p.line()+p.px(1)), "Renommer", func() { g.startRenaming(civ) }, ghostButton, panelEdge)
		}
		y += p.line()
		p.text(lineage, x0, y, dimColor)
		p.text("Vit depuis "+span(age)+" · vénère "+familyArticle(sim.Family(worship)), x0, y+p.line(), dimColor)
		for i, l := range trait {
			p.text(l, x0, y+(2+i)*p.line(), aetherColor)
		}
		for i, l := range relic {
			p.text(l, x0, y+(2+len(trait)+i)*p.line(), goldColor)
		}
		bw := p.widthOf(state, small) + p.px(12)
		badge := image.Rect(x0+w-bw, y+p.px(4), x0+w, y+p.px(4)+p.lineOf(small)+p.px(4))
		p.fill(badge, stateC)
		p.styled(state, badge.Min.X+p.px(6), badge.Min.Y+p.px(2), inkColor, small)
	}})

	// The way down to the whirlpool.
	exposure := sim.Exposure(sum.Radius)
	threshold := sim.Exposure(sim.DeclineRadius)
	speed := o.Descent
	if speed <= 0 { // an older sky does not tell
		speed = sim.DescentSpeed(o.Power)
	}
	var forecast string
	switch {
	case sum.FallingMs > 0:
		forecast = "Le tourbillon engloutit l'île."
	case sum.Declining:
		forecast = "Le tourbillon l'emportera dans " + span(time.Duration((sum.Radius-sim.PortalRadius)/speed)*time.Second) + " environ."
	default:
		forecast = "Le déclin commencera dans " + span(time.Duration((sum.Radius-sim.DeclineRadius)/speed)*time.Second) + " environ. Plus l'île est puissante, plus elle descend vite."
	}
	text := p.wrap(forecast, w)
	lines = append(lines, bookLine{p.line() + p.px(30) + p.lineOf(small) + len(text)*p.line() + gap, func(y int) {
		label("Trajectoire", y)
		figures := fmt.Sprintf("puissance %.0f · apogée %.0f", o.Power, math.Max(o.Peak, o.Power))
		p.text(figures, x0+w-p.width(figures), y, dimColor)
		y += p.line() + p.px(8)
		bar := image.Rect(x0, y, x0+w, y+p.px(14))
		p.gauge(bar, exposure, familyColor[3])
		mark := func(share float64, c color.Color, wide, over int) {
			x := x0 + int(float64(w)*share)
			p.fill(image.Rect(x-p.px(wide)/2, y-p.px(over), x+p.px(wide)/2+p.px(1), y+p.px(14+over)), c)
		}
		mark(threshold, alertColor, 2, 3)
		mark(exposure, goldColor, 6, 4)
		y += p.px(22)
		p.styled("Bord du ciel", x0, y, dimColor, small)
		p.styled("Déclin", x0+int(float64(w)*threshold)-p.widthOf("Déclin", small)/2, y, alertText, small)
		p.styled("Tourbillon", x0+w-p.widthOf("Tourbillon", small), y, dimColor, small)
		y += p.lineOf(small)
		for _, l := range text {
			p.text(l, x0, y, textColor)
			y += p.line()
		}
	}})

	// Heart, people, soldiers.
	heartNote := "intact"
	switch {
	case o.Battle.Tick > 0 && !o.Battle.Repelled && st.Tick > o.Battle.Tick:
		heartNote = "frappé il y a " + span(time.Duration(st.Tick-o.Battle.Tick)*sim.TickDuration)
	case sum.Heart < 1:
		heartNote = "se répare"
	}
	type vital struct {
		label, value, note string
		share              float64
		c                  color.Color
	}
	vitals := []vital{
		{"Cœur", fmt.Sprintf("%d%%", int(math.Round(sum.Heart*100))), heartNote, sum.Heart, aetherColor},
		{"Peuple", fmt.Sprintf("%.0f / %d", o.Population, o.Housing), fmt.Sprintf("+%d nés · %s", int(o.Births), plural(math.Floor(o.Deaths), "mort")), o.Population / math.Max(float64(o.Housing), 1), textColor},
		{"Soldats", fmt.Sprintf("%.0f · %.0f", o.Defenders, o.Assailants), "défense · attaque", math.Min((o.Defenders+o.Assailants)/math.Max(o.Population, 1), 1), familyColor[1]},
	}
	boxH := p.lineOf(small) + p.line() + p.px(6) + p.line() + p.px(12)
	lines = append(lines, bookLine{boxH + gap, func(y int) {
		bw := (w - 2*p.px(6)) / 3
		for i, v := range vitals {
			bx := x0 + i*(bw+p.px(6))
			r := image.Rect(bx, y, bx+bw, y+boxH)
			p.fill(r, buttonFill)
			p.frame(r, 1, panelDim)
			iy := y + p.px(6)
			p.styled(v.label, bx+p.px(8), iy, dimColor, small)
			iy += p.lineOf(small)
			p.styled(v.value, bx+p.px(8), iy, v.c, bold)
			iy += p.line()
			p.gauge(image.Rect(bx+p.px(8), iy, bx+bw-p.px(8), iy+p.px(4)), v.share, v.c)
			iy += p.px(6)
			p.text(v.note, bx+p.px(8), iy, dimColor)
		}
	}})

	// Buildings, and the sites in progress.
	var counts [sim.NumKinds]int
	var sites []sim.Building
	done := 0
	for _, b := range o.Buildings {
		if !validKind(b.Kind) {
			continue
		}
		if b.Progress < 1 {
			sites = append(sites, b)
			continue
		}
		counts[b.Kind]++
		done++
	}
	rows := (int(sim.NumKinds) + 4) / 5
	lines = append(lines, bookLine{p.line() + p.px(4) + rows*(p.line()+p.px(4)) + len(sites)*p.line() + gap, func(y int) {
		label("Bâtiments", y)
		summary := fmt.Sprintf("%s · %s", plural(float64(done), "achevé"), plural(float64(len(sites)), "chantier"))
		p.text(summary, x0+w-p.width(summary), y, dimColor)
		y += p.line() + p.px(4)
		cw := (w - 4*p.px(4)) / 5
		for k := range sim.NumKinds {
			cx := x0 + int(k)%5*(cw+p.px(4))
			cy := y + int(k)/5*(p.line()+p.px(4))
			r := image.Rect(cx, cy, cx+cw, cy+p.line())
			p.fill(r, buttonFill)
			c := color.Color(textColor)
			if counts[k] == 0 {
				c = offColor
			}
			name := buildingLabels[k]
			n := fmt.Sprint(counts[k])
			for p.width(name) > cw-p.px(14)-p.widthOf(n, bold) && len([]rune(name)) > 4 {
				name = string([]rune(name)[:len([]rune(name))-2]) + "."
			}
			p.text(name, cx+p.px(4), cy, c)
			p.styled(n, cx+cw-p.px(4)-p.widthOf(n, bold), cy, c, bold)
		}
		y += rows * (p.line() + p.px(4))
		for _, s := range sites {
			name := "Chantier : " + sim.Specs[s.Kind].Name
			p.text(name, x0, y, textColor)
			gx := x0 + p.px(230)
			p.gauge(image.Rect(gx, y+(p.line()-p.px(6))/2, x0+w-p.px(50), y+(p.line()+p.px(6))/2), s.Progress, goldColor)
			pct := fmt.Sprintf("%d%%", int(s.Progress*100))
			p.styled(pct, x0+w-p.widthOf(pct, small), y+(p.line()-p.lineOf(small))/2, dimColor, small)
			y += p.line()
		}
	}})

	// The reserves against the cap, and their trend.
	names := [...]string{"Bois", "Pierre", "Vivres", "Aether"}
	res := [...]sim.Resource{sim.Wood, sim.Stone, sim.Food, sim.Aether}
	lines = append(lines, bookLine{p.line() + len(res)*p.line() + gap, func(y int) {
		label("Réserves", y)
		y += p.line()
		for i, r := range res {
			v := o.Stock[r]
			p.text(names[i], x0, y, dimColor)
			gx := x0 + p.px(70)
			gw := w - p.px(70) - p.px(190)
			p.gauge(image.Rect(gx, y+(p.line()-p.px(6))/2, gx+gw, y+(p.line()+p.px(6))/2), v/math.Max(o.Cap, 1), resourceColor[i])
			amount := fmt.Sprintf("%.0f / %.0f", v, o.Cap)
			p.text(amount, gx+gw+p.px(10), y, textColor)
			trend, tc := "…", color.Color(dimColor)
			if v >= o.Cap*0.99 {
				trend = "plein"
			} else if d, ok := g.trend(r); ok {
				switch {
				case d > 0.5:
					trend, tc = fmt.Sprintf("▲ +%.0f/h", d), color.Color(color.RGBA{0x9b, 0xe2, 0x9b, 0xff})
				case d < -0.5:
					trend, tc = fmt.Sprintf("▼ %.0f/h", d), color.Color(alertText)
				default:
					trend = "stable"
				}
			}
			p.text(trend, x0+w-p.width(trend), y, tc)
			y += p.line()
		}
	}})

	// The threats.
	var threats []string
	if o.WaveInMs >= 0 {
		threats = append(threats, o.WaveSpecies.Short()+" dans "+clock(o.WaveInMs)+".")
		if hint := o.WaveSpecies.Hint(); hint != "" {
			threats = append(threats, p.wrap(hint, w)...)
		}
	}
	if st.BossInMs >= 0 {
		threats = append(threats, "Le colosse dans "+clock(st.BossInMs)+".")
	}
	var banners []string
	for _, is := range st.Islands {
		if is.BannerTarget == st.You && hostile(is.Banner) && is.ID != st.You {
			banners = append(banners, is.Name+" dresse sa bannière "+bannerWord(is.Banner)+" vers toi.")
		}
	}
	if len(threats)+len(banners) == 0 {
		threats = append(threats, "Rien ne menace l'île pour l'instant.")
	}
	var shield []string
	if sum.Protected {
		still := "pour l'instant"
		if sum.ProtectedMs > 0 {
			still = "encore " + countdown(sum.ProtectedMs)
		}
		shield = p.wrap("Jeune civilisation : aucune île ne peut l'attaquer "+still+", tant qu'elle n'attaque pas elle-même.", w)
	}
	lines = append(lines, bookLine{p.px(8) + p.line() + (len(shield)+len(threats)+len(banners))*p.line(), func(y int) {
		p.fill(image.Rect(x0, y, x0+w, y+p.px(2)), buttonFill)
		y += p.px(8)
		label("Menaces", y)
		y += p.line()
		for _, l := range shield {
			p.text(l, x0, y, aetherColor)
			y += p.line()
		}
		for _, t := range threats {
			p.text(t, x0, y, textColor)
			y += p.line()
		}
		for _, b := range banners {
			p.text(b, x0, y, alertText)
			y += p.line()
		}
	}})
	return lines
}

// familyArticle names a family as one worships it.
func familyArticle(f sim.Family) string {
	switch f {
	case sim.Family(0):
		return "les Armes"
	case sim.Family(1):
		return "les Murailles"
	case sim.Family(2):
		return "l'Industrie"
	}
	return "les Arcanes"
}

// bannerWord names the intent of a banner.
func bannerWord(o sim.Outer) string {
	switch o {
	case sim.Annihilate:
		return "d'anéantissement"
	case sim.Punish:
		return "de châtiment"
	}
	return "de convoitise"
}

// startRenaming opens the name of the civilisation to typing.
func (g *Game) startRenaming(current string) {
	g.renaming, g.newName = true, []rune(current)
	g.dirty = true
}

// typeName types into the name of the civilisation: Enter sends it to the
// sky, Escape gives up.
func (g *Game) typeName() {
	for _, r := range g.chars {
		if r >= ' ' && len(g.newName) < sim.MaxName {
			g.newName = append(g.newName, r)
		}
	}
	if d := inpututil.KeyPressDuration(ebiten.KeyBackspace); len(g.newName) > 0 && (d == 1 || (d > 30 && d%4 == 0)) {
		g.newName = g.newName[:len(g.newName)-1]
	}
	switch {
	case inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyKPEnter):
		g.confirmName()
	case inpututil.IsKeyJustPressed(ebiten.KeyEscape):
		g.renaming = false
	}
	g.dirty = true
}

// confirmName sends the new name to the sky.
func (g *Game) confirmName() {
	if name := sim.CleanName(string(g.newName)); name != "" {
		g.src.Rename(name)
	}
	g.renaming = false
	g.dirty = true
}

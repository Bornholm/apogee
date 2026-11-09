package client

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"apogee/internal/proto"
	"apogee/internal/sim"
)

// The HUD follows GDD 13.2: the pie of intentions, the pending supplique,
// the chronicle, the resources, the threat and the time of day. Culture and
// knowledge (C) and the Annals (A) open on demand. The god only ever acts
// through the pie and the answers (GDD pilier 4).

var phaseNames = map[string]string{"dawn": "Aube", "day": "Jour", "dusk": "Crépuscule", "night": "Nuit"}

func clock(ms int64) string {
	s := max(ms/1000, 0)
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

// uiScale is the integer scale of the interface: the font stays crisp.
func (g *Game) uiScale() float64 { return math.Max(1, math.Round(g.dsfOr1())) }

func (g *Game) drawHUD(screen *ebiten.Image, st *proto.State) {
	p := painter{dst: screen, k: g.uiScale(), buttons: &g.buttons}
	k := int(p.k)
	gap := 8 * k
	if st == nil {
		p.text("Connexion au ciel…", gap, gap, textColor)
		return
	}

	// Top bar: the sky, the time of day, the threats.
	bar := image.Rect(0, 0, g.outW, p.line()+gap)
	p.fill(bar, panelFill)
	x := gap
	put := func(s string, c color.Color) {
		p.text(s, x, gap/2, c)
		x += p.width(s) + 3*gap
	}
	put(g.src.Sky, goldColor)
	put(fmt.Sprintf("%s · %s", phaseNames[st.Phase], clock(st.PhaseLeftMs)), textColor)
	if o := st.Own; o != nil {
		put(fmt.Sprintf("Puissance %.0f · Défense %.0f", o.Power, o.Defense), textColor)
		if o.WaveInMs >= 0 {
			c := dimColor
			if o.WaveInMs < 60_000 {
				c = alertColor
			}
			put("Une vague approche : "+clock(o.WaveInMs), c)
		}
	}
	if st.BossInMs >= 0 {
		c := dimColor
		if st.BossInMs < 5*60_000 {
			c = alertColor
		}
		put("Colosse dans "+clock(st.BossInMs), c)
	}

	o := st.Own
	if o == nil {
		msg := "Ton peuple attend l'aube pour renaître."
		p.text(msg, (g.outW-p.width(msg))/2, g.outH/2, goldColor)
		g.drawHelp(p)
		return
	}
	top := bar.Max.Y + gap
	g.areas = map[string]image.Rectangle{}
	pie := g.drawPie(p, image.Rect(gap, top, gap+230*k, top+0), o)
	g.areas["pie"] = pie
	g.areas["outer"] = g.drawOuter(p, st, o, image.Rect(gap, pie.Max.Y+gap, gap+230*k, pie.Max.Y+gap))
	alertY := bar.Max.Y + gap
	if g.viewing != 0 && g.level > 0 {
		alertY = g.drawObserving(p, st, bar.Max.Y+gap).Max.Y + gap // refusals go below
	}
	if msg := g.src.Refusal(6 * time.Second); msg != "" {
		msg = refusalText(msg)
		p.text(msg, (g.outW-p.width(msg))/2, alertY, alertColor)
	}
	g.areas["chronicle"] = g.drawChronicle(p, image.Rect(g.outW-gap-300*k, top, g.outW-gap, top), o)
	g.areas["stock"] = g.drawStock(p, o)
	if o.Pending != nil {
		g.drawSupplique(p, o.Pending)
	}
	if g.showCulture {
		g.drawCulture(p, o)
	}
	if g.showAnnals {
		g.drawAnnals(p, st.Annals)
	}
	g.drawHelp(p)
	g.drawTutorial(p)
}

func (g *Game) drawHelp(p painter) {
	help := "Molette : zoom · Flèches : défiler · C : culture et savoir · A : Annales · H : tutoriel"
	p.text(help, g.outW-p.width(help)-int(8*p.k), g.outH-p.line()-int(4*p.k), dimColor)
}

// drawPie draws the pie of intentions the player composes and whispers
// (GDD 6.2): target slices, and the people's actual conviction under each.
func (g *Game) drawPie(p painter, r image.Rectangle, o *proto.Own) image.Rectangle {
	k := int(p.k)
	rows := int(sim.NumIntentions)
	// Title, one line and a gauge per intention, the buttons, the padding.
	r.Max.Y = r.Min.Y + p.line() + 4*k + rows*(p.line()+10*k) + p.line() + 4*k + 12*k
	in := p.panel(r)
	p.text("Souffles", in.Min.X, in.Min.Y, goldColor)
	y := in.Min.Y + p.line() + 4*k
	bw := 16 * k
	for i := range sim.NumIntentions {
		v := g.draft[i]
		p.text(i.Name(), in.Min.X, y, textColor)
		right := in.Max.X
		p.button(image.Rect(right-bw, y-k, right, y+p.line()-k), "+", g.adjust(i, +10))
		pct := fmt.Sprintf("%d %%", v)
		p.text(pct, right-bw-6*k-p.width("100 %"), y, textColor)
		left := right - bw - 10*k - p.width("100 %") - bw
		p.button(image.Rect(left, y-k, left+bw, y+p.line()-k), "−", g.adjust(i, -10))
		y += p.line()
		p.bar(image.Rect(in.Min.X, y, in.Max.X, y+4*k), float64(v)/100, o.Conviction[i])
		y += 4*k + 6*k
	}
	var whisper func()
	if g.drafting && g.draft.Validate() == nil && g.draft != o.Pie {
		whisper = func() {
			g.src.Whisper(g.draft)
			g.drafting = false
		}
	}
	half := (in.Dx() - 4*k) / 2
	p.button(image.Rect(in.Min.X, y, in.Min.X+half, y+p.line()+4*k), "Souffler", whisper)
	var cancel func()
	if g.drafting {
		cancel = func() {
			g.draft = o.Pie
			g.drafting = false
		}
	}
	p.button(image.Rect(in.Max.X-half, y, in.Max.X, y+p.line()+4*k), "Annuler", cancel)
	return r
}

// drawObserving shows whose island the god is looking at, and the way back.
func (g *Game) drawObserving(p painter, st *proto.State, top int) image.Rectangle {
	k := int(p.k)
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
	w := max(p.width(label), p.width(back)) + 24*k
	r := image.Rect((g.outW-w)/2, top, (g.outW+w)/2, top+2*p.line()+20*k)
	in := p.panel(r)
	p.text(label, in.Min.X+(in.Dx()-p.width(label))/2, in.Min.Y, goldColor)
	p.button(image.Rect(in.Min.X, in.Min.Y+p.line()+2*k, in.Max.X, in.Max.Y), back, func() { g.Observe(0) })
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

// drawOuter shows the intention turned towards another island, the island
// selected in the sky view, and what can be whispered towards it
// (GDD 6.3).
func (g *Game) drawOuter(p painter, st *proto.State, o *proto.Own, r image.Rectangle) image.Rectangle {
	k := int(p.k)
	name := func(id proto.IslandID) string {
		for _, is := range st.Islands {
			if is.ID == id {
				return is.Name
			}
		}
		return "une île disparue"
	}
	away := false
	for _, rd := range st.Raids {
		if rd.From == st.You {
			away = true
		}
	}
	var target *proto.IslandSummary
	for i := range st.Islands {
		if st.Islands[i].ID == g.selected {
			target = &st.Islands[i]
		}
	}
	if o.Outer == 0 && !away && target == nil {
		if g.level == 0 {
			p.text("Clique une île pour la viser.", r.Min.X, r.Min.Y, textColor)
		}
		r.Max.Y = r.Min.Y + p.line()
		return r
	}
	// Status line, then for a target four lines of description and two rows
	// of buttons; the recall button closes the panel.
	bh := p.line() + 4*k
	h := p.line() + 2*k + bh + 12*k
	if target != nil {
		h += 4*p.line() + 4*k + 3*(bh+4*k)
	}
	r.Max.Y = r.Min.Y + h
	in := p.panel(r)
	y := in.Min.Y
	status := "Vers les autres îles"
	if o.Outer != 0 {
		status = fmt.Sprintf("%s : %s", o.Outer.Name(), name(o.OuterTarget))
	} else if away {
		status = "Tes guerriers sont en route."
	}
	p.text(status, in.Min.X, y, goldColor)
	y += p.line() + 2*k
	if target != nil {
		who := target.Name
		if target.AI {
			who += " (IA)"
		}
		p.text(who, in.Min.X, y, textColor)
		y += p.line()
		culture := min(max(target.Culture, 0), int(sim.NumFamilies)-1)
		p.text(fmt.Sprintf("Culture %s · puissance %.0f", sim.Family(culture).Name(), target.Power), in.Min.X, y, familyColor[culture])
		y += p.line()
		p.text(fmt.Sprintf("Cœur %d %%", int(math.Round(target.Heart*100))), in.Min.X, y, textColor)
		y += p.line()
		switch {
		case target.Protected:
			p.text("Jeune civilisation : protégée.", in.Min.X, y, dimColor)
		case target.Declining:
			p.text("En déclin.", in.Min.X, y, alertColor)
		}
		y += p.line() + 4*k
		half := (in.Dx() - 4*k) / 2
		id := target.ID
		aim := func(kind sim.Outer, ok bool) func() {
			if !ok {
				return nil
			}
			return func() { g.src.Aim(kind, id) }
		}
		raidable := !target.Protected
		// Punir leaves at once and Secourir lends spare defenders: both need
		// someone to send. Convoiter and Anéantir prepare first.
		p.button(image.Rect(in.Min.X, y, in.Min.X+half, y+bh), "Convoiter", aim(sim.Covet, raidable))
		p.button(image.Rect(in.Max.X-half, y, in.Max.X, y+bh), "Punir", aim(sim.Punish, raidable && o.Assailants >= 1))
		y += bh + 4*k
		p.button(image.Rect(in.Min.X, y, in.Min.X+half, y+bh), "Anéantir", aim(sim.Annihilate, raidable && target.Declining))
		p.button(image.Rect(in.Max.X-half, y, in.Max.X, y+bh), "Secourir", aim(sim.Rescue, o.Defenders >= 3))
		y += bh + 4*k
		p.button(image.Rect(in.Min.X, y, in.Max.X, y+bh), "Observer cette île", func() { g.Observe(id) })
		y += bh + 4*k
	}
	var recall func()
	if o.Outer != 0 || away {
		recall = func() { g.src.Aim(sim.NoOuter, 0) }
	}
	p.button(image.Rect(in.Min.X, y, in.Max.X, y+bh), "Rappeler", recall)
	return r
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

// drawChronicle draws the latest lines of the chronicle, newest first.
func (g *Game) drawChronicle(p painter, r image.Rectangle, o *proto.Own) image.Rectangle {
	k := int(p.k)
	r.Max.Y = r.Min.Y + p.line()*14 + 12*k
	in := p.panel(r)
	p.text("Chronique", in.Min.X, in.Min.Y, goldColor)
	y := in.Min.Y + p.line() + 4*k
	for i := len(o.Chronicle) - 1; i >= 0; i-- {
		lines := p.wrap(o.Chronicle[i].Text, in.Dx())
		if y+len(lines)*p.line() > in.Max.Y {
			break // only whole entries: an older one that does not fit waits
		}
		for _, l := range lines {
			p.text(l, in.Min.X, y, textColor)
			y += p.line()
		}
		y += 2 * k
	}
	return r
}

// drawStock draws the resources with their cap, and the people.
func (g *Game) drawStock(p painter, o *proto.Own) image.Rectangle {
	k := int(p.k)
	lines := []string{
		fmt.Sprintf("Bois %.0f · Pierre %.0f · Vivres %.0f · Aether %.0f  (stock max %.0f)",
			o.Stock[sim.Wood], o.Stock[sim.Stone], o.Stock[sim.Food], o.Stock[sim.Aether], o.Cap),
		fmt.Sprintf("Peuple %.0f/%d · Défenseurs %.0f · Assaillants %.0f · Génération %d",
			o.Population, o.Housing, o.Defenders, o.Assailants, o.Generation),
	}
	w := 0
	for _, l := range lines {
		w = max(w, p.width(l))
	}
	r := image.Rect(8*k, g.outH-8*k-2*p.line()-12*k-p.line(), 8*k+w+12*k, g.outH-8*k-p.line())
	in := p.panel(r)
	for i, l := range lines {
		p.text(l, in.Min.X, in.Min.Y+i*p.line(), textColor)
	}
	return r
}

// drawSupplique draws the question of the people and its answers
// (GDD 6.5). Unanswered, the people decide alone when the time runs out.
func (g *Game) drawSupplique(p painter, s *proto.Supplique) {
	k := int(p.k)
	w := 440 * k
	q := p.wrap(s.Question, w-12*k)
	h := p.line()*(len(q)+1) + len(s.Options)*(p.line()+10*k) + 16*k
	r := image.Rect((g.outW-w)/2, g.outH-h-p.line()-70*k, (g.outW+w)/2, g.outH-p.line()-70*k)
	in := p.panel(r)
	p.text("Supplique · ton peuple décidera seul dans "+clock(s.LeftMs), in.Min.X, in.Min.Y, goldColor)
	y := in.Min.Y + p.line()
	for _, l := range q {
		p.text(l, in.Min.X, y, textColor)
		y += p.line()
	}
	y += 4 * k
	for i, opt := range s.Options {
		id, option := s.ID, i
		p.button(image.Rect(in.Min.X, y, in.Max.X, y+p.line()+6*k), opt, func() { g.src.Answer(id, option) })
		y += p.line() + 10*k
	}
}

// drawCulture shows the culture and the knowledge of the people; the god
// only reads it (GDD 13.2).
func (g *Game) drawCulture(p painter, o *proto.Own) {
	k := int(p.k)
	w := 360 * k
	nf := int(sim.NumFamilies)
	h := p.line() + 4*k + nf*(p.line()+10*k) + 4*k + p.line() + max(1, len(o.Discoveries))*p.line() + 12*k
	r := image.Rect((g.outW-w)/2, (g.outH-h)/2, (g.outW+w)/2, (g.outH+h)/2)
	in := p.panel(r)
	p.text("Culture et savoir", in.Min.X, in.Min.Y, goldColor)
	y := in.Min.Y + p.line() + 4*k
	total := 0.0
	for _, v := range o.Culture {
		total += v
	}
	for f := range sim.NumFamilies {
		label := fmt.Sprintf("%s · palier %d/%d", f.Name(), o.Tiers[f], sim.TiersPerAxis)
		if int(f) == o.Axis {
			label += " · les savants y travaillent"
		}
		p.text(label, in.Min.X, y, familyColor[f])
		y += p.line()
		share := 0.0
		if total > 0 {
			share = o.Culture[f] / total
		}
		progress := 0.0
		if int(f) == o.Axis {
			progress = o.Progress
		}
		p.bar(image.Rect(in.Min.X, y, in.Max.X, y+4*k), share, progress)
		y += 10 * k
	}
	y += 4 * k
	p.text("Découvertes", in.Min.X, y, goldColor)
	y += p.line()
	if len(o.Discoveries) == 0 {
		p.text("Aucune encore.", in.Min.X, y, dimColor)
	}
	for _, d := range o.Discoveries {
		p.text("· "+d, in.Min.X, y, textColor)
		y += p.line()
	}
}

// drawAnnals lists the fallen civilisations of the sky (GDD 11.5).
func (g *Game) drawAnnals(p painter, annals []proto.Annal) {
	k := int(p.k)
	w := 520 * k
	lines := []string{}
	for i := len(annals) - 1; i >= 0; i-- {
		a := annals[i]
		lines = append(lines, fmt.Sprintf("%s, génération %d : %.1f jours, apogée %.0f, %d paliers, culture %s.",
			a.Name, a.Generation, a.DaysLived, a.PeakPower, a.Tiers, sim.Family(min(max(a.Culture, 0), int(sim.NumFamilies)-1)).Name()))
		lines = append(lines, "   "+a.Cause)
	}
	if len(lines) == 0 {
		lines = []string{"Aucune civilisation n'est encore tombée."}
	}
	h := p.line()*(len(lines)+1) + 16*k
	r := image.Rect((g.outW-w)/2, (g.outH-h)/2, (g.outW+w)/2, (g.outH+h)/2)
	in := p.panel(r)
	p.text("Annales du ciel", in.Min.X, in.Min.Y, goldColor)
	for i, l := range lines {
		c := textColor
		if i%2 == 1 {
			c = dimColor
		}
		p.text(l, in.Min.X, in.Min.Y+(i+1)*p.line(), c)
	}
}

package client

import (
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"

	"apogee/internal/art"
	"apogee/internal/sim"
)

// The « Ma lignée » screen of the home (GDD 11.7): the name of the god's
// line, the colour of their roofs and cloth, their emblem, the style of
// their buildings, with a preview of an island so dressed. None of it
// weighs on the game.

// lineagePreview renders a small island in a style and a colour, as the
// sky would show the god's.
func lineagePreview(style, colour int) *ebiten.Image {
	cam := art.DefaultCamera
	mesh, _ := art.IslandStyled(31, 26, art.IslandStyle{})
	sc := art.IslandScene{Island: cam.Render(mesh)}
	add := func(m art.Mesh, x, y float64) {
		sc.Objects = append(sc.Objects, art.Placed{Sprite: cam.Render(art.Dress(m, colour)), Pos: art.V(x, y, 0)})
	}
	st := art.Style(style)
	add(art.Core(5), 0, 0)
	add(art.HouseIn(11, st), 13, -4)
	add(art.HouseIn(12, st), -12, 3)
	add(art.HouseIn(14, st), 6, 12)
	add(art.BarnIn(15, st), -6, -13)
	add(art.BarracksIn(16, st), 14, 9)
	add(art.Pine(3), -14, -8)
	add(art.Pine(4), -17, -4)
	add(art.Pine(6), 3, -17)
	return ebiten.NewImageFromImage(sc.Flatten(cam).Img)
}

// preview returns the cached preview of a style and a colour.
func (a *App) preview(style, colour int) *ebiten.Image {
	key := [2]int{style, colour}
	if a.previews == nil {
		a.previews = map[[2]int]*ebiten.Image{}
	}
	if img, ok := a.previews[key]; ok {
		return img
	}
	img := lineagePreview(style, colour)
	a.previews[key] = img
	return img
}

// OpenLineage shows the lineage screen, for screenshots.
func (a *App) OpenLineage() { a.openLineage() }

// openLineage edits the lineage, from the one saved or the default.
func (a *App) openLineage() {
	a.editing = sim.Lineage{}
	if l := a.settings.Lineage; l != nil {
		a.editing = *l
	}
	a.lineName = field{value: a.editing.Name, max: sim.MaxName}
	a.screen, a.status = lineageScreen, ""
}

// saveLineage keeps the lineage, for the next sky joined.
func (a *App) saveLineage() {
	l := a.editing
	l.Name = sim.CleanName(a.lineName.value)
	l = l.Clean()
	a.settings.Lineage = &l
	a.settings.Save()
	a.screen = homeScreen
	a.status = "Ta lignée sera connue du ciel à ta prochaine venue."
}

// drawLineage draws the lineage screen's column, and the preview on the
// right; it returns where the column ends.
func (a *App) drawLineage(p painter, r image.Rectangle) int {
	x, w := r.Min.X, r.Dx()
	y := r.Min.Y
	l := &a.editing
	ramp := art.LineageColours[l.Colour]

	y = heading(p, "Nom de la lignée", x, y)
	a.drawField(p, image.Rect(x, y, x+min(w, p.px(320)), y+p.px(30)), &a.lineName)
	y += p.px(30) + p.px(20)

	label := func(title, choice string) {
		p.styled(title, x, y, dimColor, small)
		p.text(choice, x+w-p.width(choice), y-p.px(4), textColor)
		y += p.lineOf(small) + p.px(6)
	}
	label("Couleur · toits et étoffes", art.ColourNames[l.Colour])
	sw := (w - 7*p.px(8)) / 8
	for i, c := range art.LineageColours {
		sr := image.Rect(x+i*(sw+p.px(8)), y, x+i*(sw+p.px(8))+sw, y+p.px(32))
		p.fill(sr, c[0])
		p.fill(image.Rect(sr.Min.X, sr.Min.Y+sr.Dy()/2, sr.Max.X, sr.Max.Y), c[1])
		p.fill(image.Rect(sr.Min.X, sr.Max.Y-p.px(6), sr.Max.X, sr.Max.Y), c[2])
		if int8(i) == l.Colour {
			p.frame(sr.Inset(-p.px(4)), 2, goldColor)
		}
		i := i
		*p.buttons = append(*p.buttons, button{r: sr, action: func() { l.Colour = int8(i) }})
	}
	y += p.px(32) + p.px(20)

	label("Emblème", art.EmblemNames[l.Emblem])
	ew := (w - 11*p.px(6)) / 12
	for i := range art.Emblems {
		er := image.Rect(x+i*(ew+p.px(6)), y, x+i*(ew+p.px(6))+ew, y+ew)
		p.fill(er, buttonFill)
		c := color.Color(dimColor)
		if int8(i) == l.Emblem {
			p.frame(er, 2, goldColor)
			c = ramp[0]
		} else {
			p.frame(er, 1, panelEdge)
		}
		n := max(1, (ew-p.px(8))/(7*p.px(1)))
		size := 7 * n * p.px(1)
		emblem(p, er.Min.X+(ew-size)/2, er.Min.Y+(ew-size)/2, n, i, c)
		i := i
		*p.buttons = append(*p.buttons, button{r: er, action: func() { l.Emblem = int8(i) }})
	}
	y += ew + p.px(20)

	label("Architecture", art.StyleNames[l.Style])
	cw := (w - 2*p.px(8)) / 3
	ch := p.px(150)
	for i := range art.NumStyles {
		cr := image.Rect(x+int(i)*(cw+p.px(8)), y, x+int(i)*(cw+p.px(8))+cw, y+ch)
		p.fill(cr, buttonFill)
		if int8(i) == l.Style {
			p.frame(cr, 2, goldColor)
		} else {
			p.frame(cr, 1, panelEdge)
		}
		img := a.preview(int(i), int(l.Colour))
		b := img.Bounds()
		s := float64(max(1, (cr.Dy()-p.line()-p.px(10))/b.Dy()))
		var op ebiten.DrawImageOptions
		op.GeoM.Scale(s, s)
		op.GeoM.Translate(float64(cr.Min.X+(cw-int(float64(b.Dx())*s))/2), float64(cr.Min.Y+p.px(4)))
		p.dst.DrawImage(img, &op)
		name := art.StyleNames[i]
		p.styled(name, cr.Min.X+(cw-p.widthOf(name, bold))/2, cr.Max.Y-p.line()-p.px(2), textColor, bold)
		i := i
		*p.buttons = append(*p.buttons, button{r: cr, action: func() { l.Style = int8(i) }})
	}
	y += ch + p.px(24)

	bh := p.px(40)
	p.buttonAs(image.Rect(x, y, x+p.px(160), y+bh), "Enregistrer", a.saveLineage, primaryButton, panelEdge)
	p.buttonAs(image.Rect(x+p.px(170), y, x+p.px(300), y+bh), "Retour", func() { a.screen = homeScreen }, ghostButton, panelEdge)
	y += bh

	// The preview, on the right: the pennant over the island.
	if a.outW > p.px(1100) {
		img := a.preview(int(l.Style), int(l.Colour))
		b := img.Bounds()
		s := float64(max(2, int(float64(a.outH)*0.55/float64(b.Dy()))))
		cx := a.outW - p.px(80) - int(float64(b.Dx())*s)/2
		top := p.px(200)
		var op ebiten.DrawImageOptions
		op.GeoM.Scale(s, s)
		op.GeoM.Translate(float64(cx)-float64(b.Dx())*s/2, float64(top))
		p.dst.DrawImage(img, &op)
		name := sim.CleanName(a.lineName.value)
		if name == "" {
			name = "Ta lignée"
		}
		fw := max(p.widthOf(name, bold)+p.px(60), p.px(200))
		f := image.Rect(cx-fw/2, top-p.px(80), cx+fw/2, top-p.px(30))
		p.fill(f, panelFill)
		p.frame(f, 2, ramp[0])
		mark := image.Rect(f.Min.X+p.px(10), f.Min.Y+p.px(10), f.Min.X+p.px(40), f.Min.Y+p.px(40))
		p.fill(mark, ramp[1])
		emblem(p, mark.Min.X+p.px(4), mark.Min.Y+p.px(4), 3, int(l.Emblem), textColor)
		p.styled(name, mark.Max.X+p.px(10), f.Min.Y+(f.Dy()-p.line())/2, textColor, bold)
		p.fill(image.Rect(cx-p.px(1), f.Max.Y, cx+p.px(1), top+p.px(20)), ramp[0])
	}
	return y
}

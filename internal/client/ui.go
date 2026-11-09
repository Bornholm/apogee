package client

import (
	"image"
	"image/color"
	"strings"

	"github.com/hajimehoshi/bitmapfont/v4"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// The interface is drawn in pixel art too (GDD 13.1): a 12 px bitmap font
// and flat panels, enlarged by an integer factor like the world.

var face = text.NewGoXFace(bitmapfont.Face)

// lineHeight is the height of a line of text, before scaling.
const lineHeight = 14

// Interface colours.
var (
	panelFill   = color.RGBA{0x1d, 0x22, 0x3a, 0xd8}
	panelEdge   = color.RGBA{0x5c, 0x6a, 0x9c, 0xff}
	textColor   = color.RGBA{0xf2, 0xee, 0xe3, 0xff}
	dimColor    = color.RGBA{0xa8, 0xae, 0xc8, 0xff}
	goldColor   = color.RGBA{0xf2, 0xb1, 0x34, 0xff}
	alertColor  = color.RGBA{0xff, 0x7a, 0x5c, 0xff}
	buttonFill  = color.RGBA{0x34, 0x3d, 0x66, 0xff}
	buttonOff   = color.RGBA{0x2a, 0x2f, 0x48, 0xff}
	barBack     = color.RGBA{0x10, 0x13, 0x22, 0xff}
	barTarget   = color.RGBA{0x5c, 0x6a, 0x9c, 0xff}
	barActual   = color.RGBA{0xf2, 0xb1, 0x34, 0xff}
	familyColor = [...]color.RGBA{
		{0xe0, 0x6a, 0x4f, 0xff}, // Armes
		{0xa2, 0x9c, 0x90, 0xff}, // Murailles
		{0xe4, 0xb9, 0x6c, 0xff}, // Industrie
		{0x8a, 0x72, 0xd8, 0xff}, // Arcanes
	}
)

// button is a clickable area registered while drawing; clicks are handled in
// the next Update. The interface is redrawn on every change, so the areas
// always match what is on screen.
type button struct {
	r      image.Rectangle
	action func()
}

// painter draws the interface at an integer scale.
type painter struct {
	dst     *ebiten.Image
	k       float64 // interface scale
	buttons *[]button
}

func (p painter) text(s string, x, y int, c color.Color) {
	var op text.DrawOptions
	op.GeoM.Scale(p.k, p.k)
	op.GeoM.Translate(float64(x), float64(y))
	op.ColorScale.ScaleWithColor(c)
	text.Draw(p.dst, s, face, &op)
}

// width returns the width of a string on screen.
func (p painter) width(s string) int {
	w, _ := text.Measure(s, face, lineHeight)
	return int(w * p.k)
}

func (p painter) line() int { return int(lineHeight * p.k) }

func (p painter) fill(r image.Rectangle, c color.Color) {
	vector.FillRect(p.dst, float32(r.Min.X), float32(r.Min.Y), float32(r.Dx()), float32(r.Dy()), c, false)
}

// panel draws a framed panel and returns its inner area.
func (p painter) panel(r image.Rectangle) image.Rectangle {
	p.fill(r, panelFill)
	vector.StrokeRect(p.dst, float32(r.Min.X), float32(r.Min.Y), float32(r.Dx()), float32(r.Dy()),
		float32(p.k), panelEdge, false)
	pad := int(6 * p.k)
	return r.Inset(pad)
}

// strokeRect frames a rectangle with a line as thick as one interface pixel.
func (p painter) strokeRect(r image.Rectangle, c color.Color) {
	vector.StrokeRect(p.dst, float32(r.Min.X), float32(r.Min.Y), float32(r.Dx()), float32(r.Dy()),
		float32(p.k), c, false)
}

// button draws a button and registers its action; a nil action draws it
// disabled.
func (p painter) button(r image.Rectangle, label string, action func()) {
	fill := buttonFill
	col := color.Color(textColor)
	if action == nil {
		fill = buttonOff
		col = dimColor
	}
	p.fill(r, fill)
	vector.StrokeRect(p.dst, float32(r.Min.X), float32(r.Min.Y), float32(r.Dx()), float32(r.Dy()),
		float32(p.k), panelEdge, false)
	p.text(label, r.Min.X+(r.Dx()-p.width(label))/2, r.Min.Y+(r.Dy()-p.line())/2+int(p.k), col)
	if action != nil {
		*p.buttons = append(*p.buttons, button{r, action})
	}
}

// bar draws a gauge: a target mark and the actual value, both in [0, 1].
func (p painter) bar(r image.Rectangle, target, actual float64) {
	p.fill(r, barBack)
	w := func(v float64) int { return int(float64(r.Dx()) * min(max(v, 0), 1)) }
	p.fill(image.Rect(r.Min.X, r.Min.Y, r.Min.X+w(target), r.Max.Y), barTarget)
	in := r.Inset(int(p.k))
	p.fill(image.Rect(in.Min.X, in.Min.Y, in.Min.X+w(actual)-int(2*p.k), in.Max.Y), barActual)
}

// nbsp glues French punctuation to its word, as French typography wants:
// « and », %, : and ; never start or end a line alone.
var nbsp = strings.NewReplacer("« ", "«\u00a0", " »", "\u00a0»", " %", "\u00a0%", " :", "\u00a0:", " ;", "\u00a0;", " !", "\u00a0!", " ?", "\u00a0?")

// wrap cuts s into lines no wider than w pixels on screen, breaking only at
// ordinary spaces.
func (p painter) wrap(s string, w int) []string {
	var lines []string
	line := ""
	for _, word := range strings.Split(nbsp.Replace(s), " ") {
		if word == "" {
			continue
		}
		try := word
		if line != "" {
			try = line + " " + word
		}
		if p.width(try) > w && line != "" {
			lines = append(lines, line)
			line = word
		} else {
			line = try
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}

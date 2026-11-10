package client

import (
	"bytes"
	_ "embed"
	"image"
	"image/color"
	"math"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// The interface is drawn in pixel art too (GDD 13.1): two pixel fonts and
// flat panels with a one-pixel frame, enlarged by an integer factor like the
// world. Pixelify Sans carries the text, Silkscreen the small labels; both
// fall on their pixel grid at their sizes, so they stay crisp. The fonts are
// under the SIL Open Font License (fonts/OFL-*.txt).

var (
	//go:embed fonts/PixelifySans.ttf
	pixelifyTTF []byte
	//go:embed fonts/Silkscreen-Regular.ttf
	silkscreenTTF []byte

	pixelify   = mustSource(pixelifyTTF)
	silkscreen = mustSource(silkscreenTTF)
)

func mustSource(b []byte) *text.GoTextFaceSource {
	s, err := text.NewGoTextFaceSource(bytes.NewReader(b))
	if err != nil {
		panic(err)
	}
	return s
}

// style is a kind of text of the interface.
type style int

const (
	body  style = iota // Pixelify Sans, 16 px
	bold               // the same, bold
	small              // Silkscreen, 8 px, for labels and figures
	large              // Pixelify Sans bold, 24 px, for titles
	hero               // Pixelify Sans bold, 64 px, the name of the game
)

// sizes and leading of each style, before the interface scale.
var (
	styleSize = [...]float64{body: 16, bold: 16, small: 8, large: 24, hero: 64}
	styleLine = [...]float64{body: 20, bold: 20, small: 12, large: 28, hero: 64}
)

// faces caches the faces of each style at each scale.
var faces = map[[2]int]*text.GoTextFace{}

func faceOf(st style, k float64) *text.GoTextFace {
	key := [2]int{int(st), int(k)}
	if f, ok := faces[key]; ok {
		return f
	}
	src := pixelify
	if st == small {
		src = silkscreen
	}
	f := &text.GoTextFace{Source: src, Size: styleSize[st] * k}
	if st == bold || st == large || st == hero {
		f.SetVariation(text.MustParseTag("wght"), 700)
	}
	faces[key] = f
	return f
}

// lineHeight is the height of a line of body text, before scaling.
const lineHeight = 20

// Interface colours (the design's « tablettes d'Aether »).
var (
	panelFill   = color.NRGBA{0x18, 0x16, 0x34, 0xea}
	panelEdge   = color.RGBA{0x4b, 0x47, 0x87, 0xff}
	panelDim    = color.RGBA{0x34, 0x31, 0x5e, 0xff}
	inkColor    = color.RGBA{0x14, 0x12, 0x2b, 0xff}
	textColor   = color.RGBA{0xf2, 0xee, 0xe3, 0xff}
	dimColor    = color.RGBA{0xb4, 0xb8, 0xd6, 0xff}
	offColor    = color.RGBA{0x8a, 0x8e, 0xb0, 0xff}
	goldColor   = color.RGBA{0xf2, 0xb1, 0x34, 0xff}
	goldShade   = color.RGBA{0xb9, 0x81, 0x1b, 0xff}
	paleGold    = color.RGBA{0xff, 0xd2, 0x7a, 0xff}
	alertColor  = color.RGBA{0xff, 0x7a, 0x5c, 0xff}
	alertFill   = color.NRGBA{0x3c, 0x14, 0x1e, 0xd8}
	alertText   = color.RGBA{0xff, 0xb3, 0xa1, 0xff}
	aetherColor = color.RGBA{0x7a, 0xd8, 0xff, 0xff}
	buttonFill  = color.RGBA{0x2a, 0x27, 0x53, 0xff}
	barBack     = color.RGBA{0x14, 0x12, 0x2b, 0xff}
	familyColor = [...]color.RGBA{
		{0xff, 0x7a, 0x5c, 0xff}, // Armes
		{0xb9, 0xb4, 0xd6, 0xff}, // Murailles
		{0xf2, 0xb1, 0x34, 0xff}, // Industrie
		{0xb4, 0x8c, 0xff, 0xff}, // Arcanes
	}
	resourceColor = [...]color.RGBA{
		{0xc8, 0x93, 0x5a, 0xff}, // Bois
		{0xb9, 0xb4, 0xd6, 0xff}, // Pierre
		{0xec, 0xd0, 0x65, 0xff}, // Vivres
		{0x7a, 0xd8, 0xff, 0xff}, // Aether
	}
)

// button is a clickable area registered while drawing; clicks are handled in
// the next Update. The interface is redrawn on every change, so the areas
// always match what is on screen. A silent button only stops the click from
// reaching the world behind a panel.
type button struct {
	r      image.Rectangle
	action func()
	silent bool
}

// painter draws the interface at an integer scale.
type painter struct {
	dst     *ebiten.Image
	k       float64 // interface scale
	buttons *[]button
}

// px is n interface pixels, on screen.
func (p painter) px(n int) int { return n * int(p.k) }

// styled draws a line of text with its top at y.
func (p painter) styled(s string, x, y int, c color.Color, st style) {
	if st == small {
		s = strings.ToUpper(s)
	}
	f := faceOf(st, p.k)
	var op text.DrawOptions
	// Centre the glyphs in their line, on a whole pixel: they stay crisp.
	m := f.Metrics()
	off := math.Round((styleLine[st]*p.k - (m.HAscent + m.HDescent)) / 2)
	op.GeoM.Translate(float64(x), float64(y)+off)
	op.ColorScale.ScaleWithColor(c)
	text.Draw(p.dst, s, f, &op)
}

func (p painter) text(s string, x, y int, c color.Color) { p.styled(s, x, y, c, body) }

// widthOf returns the width of a string on screen, in a style.
func (p painter) widthOf(s string, st style) int {
	if st == small {
		s = strings.ToUpper(s)
	}
	w, _ := text.Measure(s, faceOf(st, p.k), 0)
	return int(w)
}

func (p painter) width(s string) int { return p.widthOf(s, body) }

// lineOf is the height of a line of a style, on screen.
func (p painter) lineOf(st style) int { return int(styleLine[st] * p.k) }

func (p painter) line() int { return p.lineOf(body) }

func (p painter) fill(r image.Rectangle, c color.Color) {
	vector.FillRect(p.dst, float32(r.Min.X), float32(r.Min.Y), float32(r.Dx()), float32(r.Dy()), c, false)
}

// frame draws a frame of n interface pixels inside a rectangle.
func (p painter) frame(r image.Rectangle, n int, c color.Color) {
	t := p.px(n)
	p.fill(image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+t), c)
	p.fill(image.Rect(r.Min.X, r.Max.Y-t, r.Max.X, r.Max.Y), c)
	p.fill(image.Rect(r.Min.X, r.Min.Y+t, r.Min.X+t, r.Max.Y-t), c)
	p.fill(image.Rect(r.Max.X-t, r.Min.Y+t, r.Max.X, r.Max.Y-t), c)
}

// strokeRect frames a rectangle with a line as thick as one interface pixel.
func (p painter) strokeRect(r image.Rectangle, c color.Color) { p.frame(r, 1, c) }

// block keeps the clicks on a panel from reaching the world behind it.
func (p painter) block(r image.Rectangle) {
	*p.buttons = append(*p.buttons, button{r: r, action: func() {}, silent: true})
}

// panel draws a framed panel and returns its inner area.
func (p painter) panel(r image.Rectangle) image.Rectangle {
	return p.panelEdged(r, panelEdge)
}

// panelEdged draws a panel with a frame of a colour.
func (p painter) panelEdged(r image.Rectangle, edge color.Color) image.Rectangle {
	p.fill(r, panelFill)
	p.frame(r, 2, edge)
	p.block(r)
	return r.Inset(p.px(10))
}

// buttonKind is the look of a button.
type buttonKind int

const (
	plainButton   buttonKind = iota // dark, framed
	primaryButton                   // gold: the action of the moment
	ghostButton                     // only a frame
)

// button draws a plain button and registers its action; a nil action draws
// it disabled.
func (p painter) button(r image.Rectangle, label string, action func()) {
	p.buttonAs(r, label, action, plainButton, panelEdge)
}

// buttonAs draws a button of a kind; edge colours a ghost button.
func (p painter) buttonAs(r image.Rectangle, label string, action func(), kind buttonKind, edge color.Color) {
	col := color.Color(textColor)
	st := body
	switch {
	case action == nil:
		p.frame(r, 1, panelDim)
		col = offColor
	case kind == primaryButton:
		p.fill(r, goldColor)
		p.fill(image.Rect(r.Min.X, r.Max.Y-p.px(2), r.Max.X, r.Max.Y), goldShade)
		col, st = inkColor, bold
	case kind == ghostButton:
		p.frame(r, 2, edge)
		if edge != color.Color(panelEdge) {
			col = edge
		}
	default:
		p.fill(r, buttonFill)
		p.frame(r, 1, panelEdge)
	}
	p.styled(label, r.Min.X+(r.Dx()-p.widthOf(label, st))/2, r.Min.Y+(r.Dy()-p.lineOf(st))/2, col, st)
	if action != nil {
		*p.buttons = append(*p.buttons, button{r: r, action: action})
	}
}

// gauge draws a bar filled to a share, in [0, 1].
func (p painter) gauge(r image.Rectangle, share float64, c color.Color) {
	p.fill(r, barBack)
	w := int(float64(r.Dx()) * min(max(share, 0), 1))
	p.fill(image.Rect(r.Min.X, r.Min.Y, r.Min.X+w, r.Max.Y), c)
}

// dot draws a small square of colour, as the design marks kinds with.
func (p painter) dot(x, y, n int, c color.Color) {
	p.fill(image.Rect(x, y, x+p.px(n), y+p.px(n)), c)
}

// nbsp glues French punctuation to its word, as French typography wants:
// « and », %, : and ; never start or end a line alone.
var nbsp = strings.NewReplacer("« ", "« ", " »", " »", " %", " %", " :", " :", " ;", " ;", " !", " !", " ?", " ?")

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

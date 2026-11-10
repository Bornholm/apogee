package client

import (
	"fmt"
	"image"
)

// The tutorial opens the first game (GDD 13): a few short steps, each
// pointing at the part of the screen it explains. It can be skipped, and
// reopened with the H key.

// tutorialStep is one page of the tutorial; target names the area it frames.
type tutorialStep struct {
	title, text string
	target      string // "pie", "chronicle", "stock", "outer", or ""
	zoom        int    // zoom level the step shows, or -1 to keep the current one
}

var tutorialSteps = []tutorialStep{
	{"Bienvenue, dieu d'Apogée", "Ton île flotte dans un ciel partagé. Tu ne commandes pas ton peuple : tu lui souffles des intentions, et il décide lui-même comment les réaliser.", "", 2},
	{"Les souffles", "Répartis ton attention entre trois intentions au plus, avec les boutons + et −, puis clique sur Souffler ; la touche S les affiche ou les masque. Prospérer bâtit fermes et maisons, Se fortifier élève des tours, S'armer forme des guerriers, Étudier fait progresser le savoir, S'étendre agrandit l'île.", "pie", -1},
	{"La conviction", "La barre dorée sous chaque intention montre ce que ton peuple en a vraiment retenu. Elle suit tes souffles lentement : un peuple ne change pas d'avis d'un coup.", "pie", -1},
	{"Un peuple qui vit", "Regarde ton île : les villageois vont et viennent entre leurs maisons, les chantiers, les champs et les bosquets. Une île se bâtit en une journée environ. La molette rapproche ou éloigne la vue.", "", -1},
	{"La chronique", "Ton peuple raconte ici ce qu'il fait, et pourquoi. En revenant de réunion, un coup d'œil suffit pour savoir ce qui s'est passé.", "chronicle", -1},
	{"Les suppliques", "De temps en temps, ton peuple s'adresse à toi : une découverte à choisir, un manque de place, une famine. Si tu ne réponds pas, il décide seul, sans pénalité.", "", -1},
	{"Ressources et peuple", "Bois, pierre, vivres et Aether, avec leur stock maximal, puis ton peuple et ses soldats. Le savoir et la culture s'ouvrent avec la touche C, et la touche M coupe le son.", "stock", -1},
	{"Le ciel", "Toutes les îles tournent autour du portail, d'où surgissent les vagues de monstres. Avec l'âge et la puissance, ton île descend vers lui, et les vagues se font plus violentes. Ton île porte un repère doré.", "", 0},
	{"Les autres îles", "Clique sur une île pour la convoiter, la punir, la secourir ou l'observer de près. Seule une île en déclin peut être anéantie.", "outer", 0},
	{"Grandeur et chute", "Toute civilisation finit par tomber dans le tourbillon. Le talent d'un dieu, c'est de porter la sienne le plus haut et le plus longtemps possible. Ton peuple renaîtra à l'aube suivante, et les Annales (touche A) garderont son souvenir. Bonne partie !", "", 2},
}

// drawTutorial draws the current step over the game, framing its target.
func (g *Game) drawTutorial(p painter) {
	if !g.tutorial || g.tutStep >= len(tutorialSteps) {
		return
	}
	step := tutorialSteps[g.tutStep]
	if step.zoom >= 0 && g.level != step.zoom && !g.tutZoomed {
		g.SetZoom(step.zoom)
		g.tutZoomed = true
	}
	k := int(p.k)
	if r, ok := g.areas[step.target]; ok {
		for i := 0; i < 3; i++ {
			p.strokeRect(r.Inset(-(2+i)*k), goldColor)
		}
	}
	w := 520 * k
	lines := p.wrap(step.text, w-12*k)
	bh := p.line() + 8*k
	h := p.line()*(len(lines)+1) + bh + 30*k
	// At the bottom of the screen, clear of the island and of the panels.
	bottom := g.outH - p.line() - 70*k
	r := image.Rect((g.outW-w)/2, bottom-h, (g.outW+w)/2, bottom)
	in := p.panel(r)
	p.text(fmt.Sprintf("%s  (%d/%d)", step.title, g.tutStep+1, len(tutorialSteps)), in.Min.X, in.Min.Y, goldColor)
	y := in.Min.Y + p.line() + 4*k
	for _, l := range lines {
		p.text(l, in.Min.X, y, textColor)
		y += p.line()
	}
	y = in.Max.Y - bh
	third := (in.Dx() - 8*k) / 3
	var back func()
	if g.tutStep > 0 {
		back = func() { g.tutorialGo(g.tutStep - 1) }
	}
	p.button(image.Rect(in.Min.X, y, in.Min.X+third, y+bh), "Précédent", back)
	p.button(image.Rect(in.Min.X+third+4*k, y, in.Max.X-third-4*k, y+bh), "Passer", g.tutorialEnd)
	next := "Suivant"
	if g.tutStep == len(tutorialSteps)-1 {
		next = "Jouer"
	}
	p.button(image.Rect(in.Max.X-third, y, in.Max.X, y+bh), next, func() { g.tutorialGo(g.tutStep + 1) })
}

func (g *Game) tutorialGo(step int) {
	if step >= len(tutorialSteps) {
		g.tutorialEnd()
		return
	}
	g.tutStep, g.tutZoomed = step, false
}

func (g *Game) tutorialEnd() {
	g.tutorial, g.tutStep = false, 0
	if g.onTutorialDone != nil {
		g.onTutorialDone()
	}
}

// TutorialStep opens the tutorial at a given step, for screenshots.
func (g *Game) TutorialStep(step int) {
	g.OpenTutorial()
	g.tutorialGo(min(max(step, 0), len(tutorialSteps)-1))
}

// CloseTutorial hides the tutorial without marking it as seen.
func (g *Game) CloseTutorial() {
	g.tutorial = false
	g.dirty = true
}

// OpenTutorial shows the tutorial from its first step.
func (g *Game) OpenTutorial() {
	g.tutorial, g.tutStep, g.tutZoomed = true, 0, false
	g.dirty = true
}

package client

import (
	"testing"

	"apogee/internal/art"
)

// The view goes past the scene by the width of the side panels, no more:
// an island under the chronicle can be brought out, the sky never lost.
func TestPanStaysWithinTheSceneAndItsMargins(t *testing.T) {
	g := &Game{cam: art.DefaultCamera, scale: 2, outW: 400, outH: 300}
	b := g.sceneBounds()
	halfW, halfH := 400/2/2.0, 300/2/2.0
	sideX := float64(chronicleWidth+2*hudMargin) / 2
	sideY := float64(panelTop+50) / 2

	x, y := g.clampPan(1e6, 1e6)
	if got := float64(b.Min.X) + x; got != -halfW+sideX {
		t.Errorf("scrolled right too far: left edge at %v, want %v", got, -halfW+sideX)
	}
	if got := float64(b.Min.Y) + y; got != -halfH+sideY {
		t.Errorf("scrolled down too far: top edge at %v, want %v", got, -halfH+sideY)
	}
	x, y = g.clampPan(-1e6, -1e6)
	if got := float64(b.Max.X) + x; got != halfW-sideX {
		t.Errorf("scrolled left too far: right edge at %v, want %v", got, halfW-sideX)
	}
	if got := float64(b.Max.Y) + y; got != halfH-sideY {
		t.Errorf("scrolled up too far: bottom edge at %v, want %v", got, halfH-sideY)
	}
}

// Even when the whole sky fits the window, the view moves by a panel's
// width: what the chronicle hides is reached.
func TestAPanelNeverHidesAnIslandForGood(t *testing.T) {
	g := &Game{cam: art.DefaultCamera, scale: 2, outW: 4000, outH: 3000}
	sideX := float64(chronicleWidth+2*hudMargin) / 2
	x, _ := g.clampPan(1e6, 0)
	if x <= 0 || x > sideX {
		t.Errorf("pan %v on a window larger than the sky, want up to %v", x, sideX)
	}
}

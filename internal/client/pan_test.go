package client

import (
	"testing"

	"apogee/internal/art"
)

func TestPanStaysWithinTheScene(t *testing.T) {
	g := &Game{cam: art.DefaultCamera, scale: 2, outW: 400, outH: 300}
	b := g.sceneBounds()
	halfW, halfH := 400/2/2.0, 300/2/2.0

	x, y := g.clampPan(1e6, 1e6)
	if got := float64(b.Min.X) + x; got != -halfW {
		t.Errorf("scrolled right too far: left edge at %v, want %v", got, -halfW)
	}
	if got := float64(b.Min.Y) + y; got != -halfH {
		t.Errorf("scrolled down too far: top edge at %v, want %v", got, -halfH)
	}
	x, y = g.clampPan(-1e6, -1e6)
	if got := float64(b.Max.X) + x; got != halfW {
		t.Errorf("scrolled left too far: right edge at %v, want %v", got, halfW)
	}
	if got := float64(b.Max.Y) + y; got != halfH {
		t.Errorf("scrolled up too far: bottom edge at %v, want %v", got, halfH)
	}
}

func TestNoPanWhenTheSceneFits(t *testing.T) {
	g := &Game{cam: art.DefaultCamera, scale: 2, outW: 4000, outH: 3000}
	if x, y := g.clampPan(50, -50); x != 0 || y != 0 {
		t.Errorf("pan (%v, %v) on a window larger than the scene, want (0, 0)", x, y)
	}
}

package client

import "testing"

func TestCloudCrossingZeroIsRedrawn(t *testing.T) {
	// int() truncates towards zero while drawing uses Floor: a cloud moving
	// from -0.01 to +0.01 changes drawn column (-1 → 0) and must be redrawn.
	c := cloud{x: -0.01, speed: 0.02}
	c.lastDraw = c.pixel()
	if !c.drift(1000) {
		t.Fatal("the cloud moved by a pixel without asking for a redraw")
	}
}

func TestCloudStillWithinPixelIsNotRedrawn(t *testing.T) {
	c := cloud{x: 3.1, speed: 0.02}
	c.lastDraw = c.pixel()
	if c.drift(1000) {
		t.Fatal("a sub-pixel move must not cost a redraw")
	}
}

func TestCloudWrapsBeyondLimit(t *testing.T) {
	c := cloud{x: 99.99, speed: 0.02}
	c.drift(100)
	if c.x != -100 {
		t.Fatalf("x = %v after crossing the limit, want -100", c.x)
	}
}

func TestZoomStaysWithinLevels(t *testing.T) {
	g := &Game{scale: float64(zoomLevels[0])}
	g.zoomBy(-1)
	if g.level != 0 {
		t.Fatalf("level %d after zooming out of the first level", g.level)
	}
	for range 10 {
		g.zoomBy(1)
	}
	if g.level != len(zoomLevels)-1 {
		t.Fatalf("level %d after zooming in past the last level", g.level)
	}
}

func TestLeavingTheSkyViewDoesNotAnimate(t *testing.T) {
	g := &Game{scale: float64(zoomLevels[0])}
	g.zoomBy(1) // sky → island: a different scene, no scale animation
	if g.anim != 0 || g.scale != float64(zoomLevels[1]) {
		t.Fatalf("anim %d, scale %v", g.anim, g.scale)
	}
	g.zoomBy(1) // island → island: animated
	if g.anim != transitionTicks {
		t.Fatalf("anim %d, want %d", g.anim, transitionTicks)
	}
}

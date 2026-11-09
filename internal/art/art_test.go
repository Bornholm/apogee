package art

import (
	"bytes"
	"testing"
)

func TestFillPolygonSharedEdgeHasNoGapNorOverlap(t *testing.T) {
	// Two triangles splitting a 10×10 square along its diagonal must cover
	// each pixel of the square exactly once.
	a := []pt{{0, 0}, {10, 0}, {10, 10}}
	b := []pt{{0, 0}, {10, 10}, {0, 10}}
	var hits [10][10]int
	count := func(x, y int) {
		if x < 0 || y < 0 || x >= 10 || y >= 10 {
			t.Fatalf("pixel (%d,%d) outside the square", x, y)
		}
		hits[y][x]++
	}
	fillPolygon(a, count)
	fillPolygon(b, count)
	for y := range 10 {
		for x := range 10 {
			if hits[y][x] != 1 {
				t.Fatalf("pixel (%d,%d) covered %d times, want 1", x, y, hits[y][x])
			}
		}
	}
}

func TestFillPolygonIgnoresDegenerateInput(t *testing.T) {
	fillPolygon([]pt{{0, 0}, {5, 5}}, func(x, y int) { t.Fatal("a segment must not plot") })
}

func TestTemplatesAreDeterministic(t *testing.T) {
	cam := DefaultCamera
	builders := map[string]func(uint64) Mesh{
		"pine": Pine, "rock": Rock, "house": House, "crystal": Crystal,
		"core": Core, "cloud": CloudPuff, "villager": Villager,
		"island": func(s uint64) Mesh { m, _ := Island(s, 40); return m },
	}
	for name, build := range builders {
		a, b := cam.Render(build(42)), cam.Render(build(42))
		if !bytes.Equal(a.Img.Pix, b.Img.Pix) || a.Anchor != b.Anchor {
			t.Errorf("%s: same seed gave different sprites", name)
		}
		if a.Img.Rect.Empty() {
			t.Errorf("%s: empty sprite", name)
		}
	}
}

func TestToneFollowsLight(t *testing.T) {
	cam := DefaultCamera
	cases := []struct {
		name   string
		normal Vec3
		want   Tone
	}{
		{"top", V(0, 0, 1), Lit},
		{"west", V(-1, 0, 0), Lit},
		{"south", V(0, 1, 0), Mid},
		{"east", V(1, 0, 0), Dark},
	}
	for _, c := range cases {
		if got := cam.ToneOf(c.normal); got != c.want {
			t.Errorf("%s face: tone %d, want %d", c.name, got, c.want)
		}
	}
}

func TestOnlyPaletteColoursAreUsed(t *testing.T) {
	allowed := map[[3]uint8]bool{}
	for _, r := range Palette {
		for _, c := range r {
			allowed[[3]uint8{c.R, c.G, c.B}] = true
		}
	}
	sp := DefaultCamera.Render(House(3))
	for i := 0; i < len(sp.Img.Pix); i += 4 {
		p := sp.Img.Pix[i : i+4]
		if p[3] != 0 && !allowed[[3]uint8{p[0], p[1], p[2]}] {
			t.Fatalf("colour %v is not in the palette", p[:3])
		}
	}
}

func TestShadowFallsAwayFromLight(t *testing.T) {
	sp := DefaultCamera.Render(Pine(1))
	if sp.Shadow == nil {
		t.Fatal("a pine must cast a shadow")
	}
	// The light comes from the upper left, so the shadow extends to the right
	// of the object's anchor.
	right := sp.Shadow.Rect.Dx() - sp.ShadowAnchor.X
	if right <= sp.ShadowAnchor.X {
		t.Errorf("shadow extends %d px right and %d px left of the anchor", right, sp.ShadowAnchor.X)
	}
}

func TestIslandTopIsGround(t *testing.T) {
	sp := DefaultCamera.Render(func() Mesh { m, _ := Island(5, 40); return m }())
	n := 0
	for _, a := range sp.Ground.Pix {
		if a != 0 {
			n++
		}
	}
	if n == 0 {
		t.Fatal("the island top must be marked as ground for shadows")
	}
}

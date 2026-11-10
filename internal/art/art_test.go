package art

import (
	"bytes"
	"image"
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

func TestComposeOnOffsetDestination(t *testing.T) {
	// A destination whose origin is not (0, 0), like an atlas cell, must get
	// the same pixels as a fresh image.
	cam := DefaultCamera
	scene := DemoIsland(cam, 3, 30)
	plain := image.NewRGBA(image.Rect(0, 0, 120, 100))
	scene.Compose(cam, plain, 60, 40)

	atlas := image.NewRGBA(image.Rect(0, 0, 300, 300))
	cell := atlas.SubImage(image.Rect(100, 150, 220, 250)).(*image.RGBA)
	scene.Compose(cam, cell, 160, 190)
	for y := range 100 {
		for x := range 120 {
			if plain.RGBAAt(x, y) != cell.RGBAAt(100+x, 150+y) {
				t.Fatalf("pixel (%d,%d) differs on an offset destination", x, y)
			}
		}
	}
}

func TestUpscaleTreatsSmallFactorsAsOne(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 3, 2))
	for _, k := range []int{0, -2} {
		if got := Upscale(src, k).Rect.Size(); got != image.Pt(3, 2) {
			t.Errorf("Upscale(k=%d) size %v, want 3×2", k, got)
		}
	}
}

func TestFlattenKeepsTheTallestObject(t *testing.T) {
	cam := DefaultCamera
	scene := DemoIsland(cam, 11, 16)
	// A tall object near the back edge rises far above the island sprite.
	var tower Mesh
	box(&tower, Stone, -1, -1, 0, 1, 1, 90)
	scene.Objects = append(scene.Objects, Placed{Sprite: cam.Render(tower), Pos: V(0, -12, 0)})
	opaque := func(img *image.RGBA) (n int) {
		for i := 3; i < len(img.Pix); i += 4 {
			if img.Pix[i] != 0 {
				n++
			}
		}
		return n
	}
	// Reference: the same scene on a canvas with ample room on every side.
	ref := image.NewRGBA(image.Rect(0, 0, 400, 400))
	scene.Compose(cam, ref, 200, 250)
	if got, want := opaque(scene.Flatten(cam).Img), opaque(ref); got != want {
		t.Fatalf("flattened sprite has %d opaque pixels, want %d: something was cropped", got, want)
	}
}

func TestFlattenKeepsObjectsOverflowingSideways(t *testing.T) {
	cam := DefaultCamera
	scene := DemoIsland(cam, 11, 16)
	var wall Mesh
	box(&wall, Stone, -2, -2, 0, 30, 2, 4) // juts far out to the east
	scene.Objects = append(scene.Objects, Placed{Sprite: cam.Render(wall), Pos: V(10, 0, 0)})
	ref := image.NewRGBA(image.Rect(0, 0, 400, 400))
	scene.Compose(cam, ref, 200, 250)
	n := func(img *image.RGBA) (k int) {
		for i := 3; i < len(img.Pix); i += 4 {
			if img.Pix[i] != 0 {
				k++
			}
		}
		return k
	}
	if got, want := n(scene.Flatten(cam).Img), n(ref); got != want {
		t.Fatalf("flattened sprite has %d opaque pixels, want %d", got, want)
	}
}

func TestWindowsFaceTheViewer(t *testing.T) {
	for seed := range uint64(8) {
		ws := Windows(House(seed), 0.45, Plaster)
		if len(ws) < 1 || len(ws) > 3 {
			t.Fatalf("seed %d: %d windows on a house", seed, len(ws))
		}
		for _, w := range ws {
			if w.Y <= 0 || w.Z < 1 || w.Z > 4 {
				t.Fatalf("seed %d: window at %+v, not on the front wall", seed, w)
			}
		}
	}
	if ws := Windows(Field(1), 0.45, Plaster); len(ws) != 0 {
		t.Fatalf("a field has %d windows", len(ws))
	}
}

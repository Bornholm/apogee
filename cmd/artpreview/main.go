// Command artpreview renders a demo view to a PNG file, without opening a
// window. It is the quickest way to review the art direction.
package main

import (
	"flag"
	"image"
	"image/png"
	"log"
	"math"
	"os"
	"sort"

	"apogee/internal/art"
)

func main() {
	out := flag.String("o", "preview.png", "output PNG file")
	view := flag.String("view", "island", "view to render: island, sky or icon")
	seed := flag.Uint64("seed", 7, "seed")
	scale := flag.Int("scale", 0, "integer upscale factor (default: 4 for island, 2 for sky)")
	flag.Parse()
	if *scale < 0 {
		log.Fatal("-scale must be positive")
	}

	var canvas *image.RGBA
	switch *view {
	case "island":
		canvas = islandView(*seed)
		if *scale == 0 {
			*scale = 4
		}
	case "sky":
		canvas = skyView(*seed)
		if *scale == 0 {
			*scale = 2
		}
	case "icon":
		canvas = art.Icon()
		if *scale == 0 {
			*scale = 4
		}
	default:
		log.Fatalf("unknown view %q", *view)
	}

	f, err := os.Create(*out)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, art.Upscale(canvas, *scale)); err != nil {
		log.Fatal(err)
	}
}

func islandView(seed uint64) *image.RGBA {
	const w, h = 320, 240
	cam := art.DefaultCamera
	canvas := image.NewRGBA(image.Rect(0, 0, w, h))
	art.FillSky(canvas)
	clouds(canvas, cam, seed, w, h)
	scene := art.DemoIsland(cam, seed, 60)
	scene.Compose(cam, canvas, w/2, h*2/5)
	return canvas
}

// skyView draws the "Ciel" view: islands orbiting the portal (GDD 3.1).
func skyView(seed uint64) *image.RGBA {
	const w, h = 640, 400
	const rim = 270.0 // orbit radius of the rim, in virtual pixels
	cam := art.DefaultCamera
	canvas := image.NewRGBA(image.Rect(0, 0, w, h))
	art.FillSky(canvas)
	clouds(canvas, cam, seed, w, h)

	cx, cy := w/2, h/2+10
	portal := art.PortalFrame(cam, 70, 3, 0)
	art.Blit(canvas, portal.Img, cx-portal.Anchor.X, cy-portal.Anchor.Y)

	type isl struct {
		sp   art.Sprite
		x, y int
	}
	var islands []isl
	r := art.Rng(seed)
	for i := range 8 {
		dist := rim * (0.42 + 0.58*r.Float64()) // islands at various stages of their descent
		angle := r.Float64() * 2 * math.Pi
		x, y := cam.Project(art.V(dist*math.Cos(angle), dist*math.Sin(angle), 0))
		radius := 14 + 6*r.Float64()
		sp := art.DemoIsland(cam, seed*1000+uint64(i), radius).Flatten(cam)
		// Atmospheric depth: islands further back are hazier.
		far := (rim - float64(y)/math.Sin(cam.Pitch)) / (2 * rim)
		sp = art.Haze(sp, art.SkyTop, 0.35*far)
		islands = append(islands, isl{sp, cx + int(math.Round(x)), cy + int(math.Round(y))})
	}
	sort.Slice(islands, func(i, j int) bool { return islands[i].y < islands[j].y })
	for _, is := range islands {
		art.Blit(canvas, is.sp.Img, is.x-is.sp.Anchor.X, is.y-is.sp.Anchor.Y)
	}
	return canvas
}

func clouds(canvas *image.RGBA, cam art.Camera, seed uint64, w, h int) {
	r := art.Rng(seed + 1000)
	for i := range 7 * w / 320 {
		c := cam.Render(art.CloudPuff(seed*101 + uint64(i)))
		art.Blit(canvas, c.Img, r.IntN(w)-c.Anchor.X, h/2+r.IntN(h/2)-c.Anchor.Y)
	}
}

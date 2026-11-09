// Command artpreview renders a demo island to a PNG file, without opening a
// window. It is the quickest way to review the art direction.
package main

import (
	"flag"
	"image"
	"image/png"
	"log"
	"os"

	"apogee/internal/art"
)

func main() {
	out := flag.String("o", "preview.png", "output PNG file")
	seed := flag.Uint64("seed", 7, "island seed")
	width := flag.Int("w", 320, "virtual width in pixels")
	height := flag.Int("h", 200, "virtual height in pixels")
	radius := flag.Float64("r", 60, "island radius in world units")
	scale := flag.Int("scale", 4, "integer upscale factor")
	flag.Parse()

	cam := art.DefaultCamera
	canvas := image.NewRGBA(image.Rect(0, 0, *width, *height))
	art.FillSky(canvas)
	r := art.Rng(*seed + 1000)
	for i := range 7 {
		c := cam.Render(art.CloudPuff(*seed*101 + uint64(i)))
		x := r.IntN(*width)
		y := *height/2 + r.IntN(*height/2)
		art.Blit(canvas, c.Img, x-c.Anchor.X, y-c.Anchor.Y)
	}
	scene := art.DemoIsland(cam, *seed, *radius)
	scene.Compose(cam, canvas, *width/2, *height*2/5)

	f, err := os.Create(*out)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, art.Upscale(canvas, *scale)); err != nil {
		log.Fatal(err)
	}
}

package art

import "image"

// iconSeed picks the island of the application icon.
const iconSeed = 11

// Icon draws the application icon: a small island on a transparent square
// of 64 pixels, to upscale by integer factors. The window, the desktop menu
// and the packages all show it.
func Icon() *image.RGBA {
	const size = 64
	cam := DefaultCamera
	sp := DemoIsland(cam, iconSeed, 22).Flatten(cam)
	canvas := image.NewRGBA(image.Rect(0, 0, size, size))
	b := sp.Img.Bounds()
	Blit(canvas, sp.Img, (size-b.Dx())/2, (size-b.Dy())/2)
	return canvas
}

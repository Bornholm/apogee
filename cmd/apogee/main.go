// Command apogee is the desktop client.
package main

import (
	"errors"
	"flag"
	"image"
	"image/png"
	"log"
	"os"
	"syscall"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"apogee/internal/client"
)

var errDone = errors.New("done")

func main() {
	seed := flag.Uint64("seed", 7, "island seed")
	shot := flag.String("shot", "", "save a screenshot to this PNG file after -frames redraws, then quit")
	frames := flag.Int("frames", 30, "redraws before the screenshot")
	cpu := flag.Duration("cpulog", 0, "log the process CPU usage at this interval (e.g. 10s)")
	quit := flag.Duration("quit", 0, "quit after this duration")
	flag.Parse()

	ebiten.SetWindowTitle("Apogée")
	ebiten.SetWindowSize(1280, 800)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	// The game lives on a second screen: keep running without focus, but only
	// redraw when the picture changes (GDD 13.3).
	ebiten.SetRunnableOnUnfocused(true)
	ebiten.SetScreenClearedEveryFrame(false)

	g := client.NewGame(*seed)
	var done error
	if *shot != "" {
		n := 0
		g.OnFrame = func(screen *ebiten.Image) {
			n++
			if n == *frames {
				if err := savePNG(*shot, screen); err != nil {
					log.Print(err)
				}
				done = errDone
			}
		}
	}
	if *cpu > 0 {
		go logCPU(*cpu)
	}
	start := time.Now()
	wrapped := &quitter{Game: g, stop: func() error {
		if *quit > 0 && time.Since(start) > *quit {
			return errDone
		}
		return done
	}}
	if err := ebiten.RunGame(wrapped); err != nil && !errors.Is(err, errDone) {
		log.Fatal(err)
	}
}

// quitter ends the game loop once stop returns an error.
type quitter struct {
	*client.Game
	stop func() error
}

func (q *quitter) Update() error {
	if err := q.stop(); err != nil {
		return err
	}
	return q.Game.Update()
}

func savePNG(path string, screen *ebiten.Image) error {
	b := screen.Bounds()
	img := image.NewRGBA(b)
	screen.ReadPixels(img.Pix)
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

// logCPU prints the share of one core used by the process over each interval.
func logCPU(every time.Duration) {
	last, lastAt := cpuTime(), time.Now()
	for range time.Tick(every) {
		now, at := cpuTime(), time.Now()
		log.Printf("cpu: %.1f%% of one core, %.0f fps", 100*(now-last).Seconds()/at.Sub(lastAt).Seconds(), ebiten.ActualFPS())
		last, lastAt = now, at
	}
}

func cpuTime() time.Duration {
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		return 0
	}
	return time.Duration(ru.Utime.Nano() + ru.Stime.Nano())
}

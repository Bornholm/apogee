// Command apogee is the desktop client.
package main

import (
	"context"
	"errors"
	"flag"
	"image"
	"image/png"
	"io"
	"log"
	"log/slog"
	"net"
	"os"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"apogee/internal/client"
	"apogee/internal/discovery"
	"apogee/internal/netclient"
	"apogee/internal/server"
)

var errDone = errors.New("done")

func main() {
	seed := flag.Uint64("seed", 7, "seed of the embedded sky and of the clouds")
	connect := flag.String("connect", "", "join the sky at host:port instead of an embedded one")
	discover := flag.Bool("discover", false, "join the first sky found on the local network")
	name := flag.String("name", "Dieu", "player name")
	rivals := flag.Int("rivals", 7, "demo rival islands in the embedded sky")
	shot := flag.String("shot", "", "save a screenshot to this PNG file after -frames redraws, then quit")
	frames := flag.Int("frames", 30, "redraws before the screenshot")
	cpu := flag.Duration("cpulog", 0, "log the process CPU usage at this interval (e.g. 10s)")
	quit := flag.Duration("quit", 0, "quit after this duration")
	width := flag.Int("width", 1280, "initial window width")
	height := flag.Int("height", 800, "initial window height")
	flag.Parse()
	if *shot != "" && *frames < 1 {
		log.Fatal("-frames must be at least 1 with -shot")
	}

	ebiten.SetWindowTitle("Apogée")
	ebiten.SetWindowSize(*width, *height)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	// The game lives on a second screen: keep running without focus, but only
	// redraw when the picture changes (GDD 13.3).
	ebiten.SetRunnableOnUnfocused(true)
	ebiten.SetScreenClearedEveryFrame(false)

	ctx := context.Background()
	addr := *connect
	if *discover {
		servers, err := discovery.Browse(ctx, 3*time.Second)
		if err != nil || len(servers) == 0 {
			log.Fatalf("no sky found on the network (%v): use -connect host:port", err)
		}
		addr = servers[0].Addr
	}
	if addr == "" {
		// Solo play runs an embedded server on the loopback (GDD 10.3).
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			log.Fatal(err)
		}
		srv := server.New(server.Config{
			Name: "Ciel solo", DemoRivals: *rivals, Seed: *seed,
			Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		})
		go srv.Run(ctx)
		go srv.Serve(ctx, ln)
		addr = ln.Addr().String()
	}
	key, err := netclient.PlayerKey()
	if err != nil {
		log.Fatal(err)
	}
	conn, err := netclient.Dial(ctx, addr, *name, key)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	g := client.NewGame(client.Listen(conn), *seed)
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

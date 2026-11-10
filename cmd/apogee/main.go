// Command apogee is the desktop client.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/png"
	"log"
	"os"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"

	"apogee/internal/client"
	"apogee/internal/discovery"
	"apogee/internal/notify"
	"apogee/internal/proto"
	"apogee/internal/sim"
	"apogee/internal/sound"
)

var errDone = errors.New("done")

// version is set at release time, by the linker.
var version = "dev"

func main() {
	seed := flag.Uint64("seed", 7, "seed of a new embedded sky and of the clouds")
	solo := flag.Bool("solo", false, "skip the home screen: continue the solo game")
	fresh := flag.Bool("fresh", false, "skip the home screen: start a new solo game")
	host := flag.Bool("host", false, "skip the home screen: host a sky on the local network")
	connect := flag.String("connect", "", "skip the home screen: join the sky at host:port")
	discover := flag.Bool("discover", false, "skip the home screen: join the first sky found on the network")
	rivals := flag.Int("rivals", 7, "rival AI gods in a new embedded sky")
	speed := flag.Int("speed", 1, "simulation steps per tick of an embedded sky (1 is real time)")
	tutorial := flag.Bool("tutorial", true, "show the tutorial when it has not been seen yet")
	tutoStep := flag.Int("tutostep", 0, "open the tutorial at this step (1-based), for screenshots")
	shot := flag.String("shot", "", "save a screenshot to this PNG file after -frames redraws, then quit")
	frames := flag.Int("frames", 30, "redraws before the screenshot")
	after := flag.Duration("after", 0, "with -shot, wait at least this long before the screenshot")
	onWave := flag.Bool("onwave", false, "with -shot, wait for a wave to fly across the sky")
	onPhase := flag.String("onphase", "", "with -shot, wait for this time of day (dawn, day, dusk, night)")
	zoom := flag.Int("zoom", 0, "initial zoom level: 0 is the sky, higher levels show your island")
	culture := flag.Bool("culture", false, "open the culture and knowledge panel at start")
	annals := flag.Bool("annals", false, "open the Annals panel at start")
	selectID := flag.Uint("select", 0, "select this island at start, as a click would")
	observe := flag.Uint("observe", 0, "observe this island at start")
	notifyFlag := flag.String("notify", "raid,wave,decline,fall", "desktop notifications while the window is hidden: raid, wave, decline, supplique, fall, or none")
	cpu := flag.Duration("cpulog", 0, "log the process CPU usage at this interval (e.g. 10s)")
	quit := flag.Duration("quit", 0, "quit after this duration")
	width := flag.Int("width", 1280, "initial window width")
	height := flag.Int("height", 800, "initial window height")
	soundOn := flag.Bool("sound", true, "play the sound effects and the music (never during a screenshot)")
	showVersion := flag.Bool("version", false, "print the version and quit")
	flag.Parse()
	if *showVersion {
		fmt.Println(version)
		return
	}
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

	start := time.Now()
	var done error
	notifier := notify.New()
	var engine *sound.Engine
	if *soundOn && *shot == "" {
		engine = startSound(*seed)
	}
	app := client.NewApp(client.AppOptions{Seed: *seed, Rivals: *rivals, Speed: *speed, Sound: engine})
	defer app.Close() // the embedded sky is saved before the process ends

	// A screenshot of the game needs a game: options about the view launch
	// the solo game when nothing else is asked; then only the game's frames
	// count. Without them, the screenshot shows the home screen.
	launched := *fresh || *solo || *host || *connect != "" || *discover
	viewed := *onWave || *onPhase != "" || *zoom > 0 || *observe > 0 || *culture || *annals || *selectID > 0 || *tutoStep > 0
	if viewed && !launched {
		*solo, launched = true, true
	}
	n := 0
	onFrame := func(screen *ebiten.Image) {
		g := app.Game()
		if launched && g == nil {
			return
		}
		n++
		ready := n >= *frames && time.Since(start) >= *after && done == nil
		if g != nil {
			ready = ready && (!*onWave || g.WaveVisible()) && (*onPhase == "" || g.Phase() == *onPhase)
		}
		if *shot != "" && ready {
			if err := savePNG(*shot, screen); err != nil {
				log.Print(err)
			}
			done = errDone
		}
	}
	app.OnFrame = onFrame
	app.Options.Configure = func(g *client.Game) {
		g.SetZoom(*zoom)
		g.ShowPanels(*culture, *annals)
		g.Select(proto.IslandID(*selectID))
		if *observe != 0 {
			g.Observe(proto.IslandID(*observe))
		}
		if !*tutorial {
			g.CloseTutorial()
		}
		if *tutoStep > 0 {
			g.TutorialStep(*tutoStep - 1)
		}
		g.NotifyKinds = notifyKinds(*notifyFlag)
		g.Notify = func(text string) { notifier.Send("Apogée", text) }
		g.OnFrame = onFrame
	}

	switch {
	case *fresh:
		app.Start("fresh")
	case *solo:
		app.Start("solo")
	case *host:
		app.Start("host")
	case *connect != "":
		app.Start(*connect)
	case *discover:
		servers, err := discovery.Browse(context.Background(), 3*time.Second)
		if err != nil || len(servers) == 0 {
			log.Fatalf("no sky found on the network (%v): use -connect host:port", err)
		}
		app.Start(servers[0].Addr)
	}
	if *cpu > 0 {
		go logCPU(*cpu)
	}
	wrapped := &quitter{App: app, stop: func() error {
		if *quit > 0 && time.Since(start) > *quit {
			return errDone
		}
		if err := app.LaunchError(); err != nil && *shot != "" {
			// A screenshot of a game that could not start would never come.
			return fmt.Errorf("no screenshot, the game did not start: %w", err)
		}
		return done
	}}
	if err := ebiten.RunGame(wrapped); err != nil && !errors.Is(err, errDone) {
		if engine != nil {
			log.Fatalf("%v (if the sound device is the cause, play with -sound=false)", err)
		}
		log.Fatal(err)
	}
}

// startSound opens the audio output and plays the engine into it.
func startSound(seed uint64) *sound.Engine {
	engine := sound.New(seed)
	player, err := audio.NewContext(sound.SampleRate).NewPlayerF32(engine)
	if err != nil {
		log.Printf("no sound: %v", err)
		return nil
	}
	// A short buffer, so that an effect follows its click closely.
	player.SetBufferSize(80 * time.Millisecond)
	player.Play()
	return engine
}

// notifyKinds parses the -notify flag.
func notifyKinds(s string) map[sim.EntryKind]bool {
	names := map[string]sim.EntryKind{"raid": sim.RaidNews, "wave": sim.WaveNews,
		"decline": sim.DeclineNews, "supplique": sim.SuppliqueNews, "fall": sim.FallNews}
	kinds := map[sim.EntryKind]bool{}
	for _, part := range strings.Split(s, ",") {
		if k, ok := names[strings.TrimSpace(part)]; ok {
			kinds[k] = true
		}
	}
	return kinds
}

// quitter ends the game loop once stop returns an error.
type quitter struct {
	*client.App
	stop func() error
}

func (q *quitter) Update() error {
	if err := q.stop(); err != nil {
		return err
	}
	return q.App.Update()
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

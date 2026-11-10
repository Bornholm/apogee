// Command apogee is the desktop client.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/png"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"

	"apogee/internal/art"
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
	gifFrames := flag.Int("gifframes", 90, "with a -shot ending in .gif, the redraws it records (about 15 a second; each is kept in memory until the end, so pair a long one with -crop)")
	crop := flag.String("crop", "", "with -shot, keep only a centred area of this size, e.g. 1200x800")
	after := flag.Duration("after", 0, "with -shot, wait at least this long before the screenshot")
	onWave := flag.Bool("onwave", false, "with -shot, wait for a wave to fly across the sky")
	onPhase := flag.String("onphase", "", "with -shot, wait for this time of day (dawn, day, dusk, night)")
	zoom := flag.Int("zoom", 0, "initial zoom level: 0 is the sky, higher levels show your island")
	culture := flag.Bool("culture", false, "open the culture and knowledge panel at start")
	annals := flag.Bool("annals", false, "open the Annals panel at start")
	civ := flag.Bool("civ", false, "open the panel of the civilization at start")
	selectID := flag.Uint("select", 0, "select this island at start, as a click would")
	observe := flag.Uint("observe", 0, "observe this island at start")
	notifyFlag := flag.String("notify", "raid,wave,decline,fall", "desktop notifications while the window is hidden: raid, wave, decline, supplique, fall, or none")
	cpu := flag.Duration("cpulog", 0, "log the process CPU usage at this interval (e.g. 10s)")
	quit := flag.Duration("quit", 0, "quit after this duration")
	width := flag.Int("width", 1280, "initial window width")
	height := flag.Int("height", 800, "initial window height")
	battleFlag := flag.String("battle", "", "play out an attack on your island at start: wave, swarm, devourers, breakers, shades, colossus or raid, with ,lost when the island does not hold")
	signFlag := flag.String("sign", "", "show a sign on your island, for screenshots: finished, grown, discovery, whisper, birth, cradle, funeral, feast, relic, wave, wound, decline, supplique or fall")
	weatherFlag := flag.String("weather", "", "force the weather, for screenshots: clear, rain, wind or fog")
	phaseFlag := flag.String("phase", "", "force the time of day, for screenshots: dawn, day, dusk or night")
	soundOn := flag.Bool("sound", true, "play the sound effects and the music (never during a screenshot)")
	showVersion := flag.Bool("version", false, "print the version and quit")
	checkUpdate := flag.Bool("checkupdate", true, "ask GitHub at launch whether a newer version was published (release builds only)")
	flag.Parse()
	if *showVersion {
		fmt.Println(version)
		return
	}
	if *shot != "" && *frames < 1 {
		log.Fatal("-frames must be at least 1 with -shot")
	}

	ebiten.SetWindowTitle("Apogée")
	icon := art.Icon()
	ebiten.SetWindowIcon([]image.Image{icon, art.Upscale(icon, 2), art.Upscale(icon, 4)})
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
	app := client.NewApp(client.AppOptions{Seed: *seed, Rivals: *rivals, Speed: *speed, Sound: engine,
		Version: version, CheckUpdate: *checkUpdate})
	defer app.Close() // the embedded sky is saved before the process ends

	// A screenshot of the game needs a game: options about the view launch
	// the solo game when nothing else is asked; then only the game's frames
	// count. Without them, the screenshot shows the home screen.
	launched := *fresh || *solo || *host || *connect != "" || *discover
	viewed := *signFlag != "" || *phaseFlag != "" || *weatherFlag != "" || *battleFlag != "" || *onWave || *onPhase != "" || *zoom > 0 || *observe > 0 || *civ || *culture || *annals || *selectID > 0 || *tutoStep > 0
	if viewed && !launched {
		*solo, launched = true, true
	}
	n := 0
	var movie []*image.RGBA
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
			img := grab(screen, *crop)
			if strings.HasSuffix(*shot, ".gif") {
				if movie = append(movie, img); len(movie) < *gifFrames {
					return
				}
				if err := saveGIF(*shot, movie); err != nil {
					log.Print(err)
				}
			} else if err := savePNG(*shot, img); err != nil {
				log.Print(err)
			}
			done = errDone
		}
	}
	app.OnFrame = onFrame
	app.Options.Configure = func(g *client.Game) {
		g.SetZoom(*zoom)
		g.ShowPanels(*civ, *culture, *annals)
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
		if *signFlag != "" {
			g.ShowSign(*signFlag)
		}
		g.ForcePhase = *phaseFlag
		g.ForceWeather = *weatherFlag
		if *battleFlag != "" {
			by, lost, _ := strings.Cut(*battleFlag, ",")
			foe := map[string]sim.Foe{"colossus": sim.ColossusFoe, "raid": sim.RaiderFoe}[by]
			species := map[string]sim.Species{"swarm": sim.Swarm, "devourers": sim.Devourers, "breakers": sim.Breakers, "shades": sim.Shades}[by]
			foes := 6
			if species == sim.Swarm {
				foes = 12
			}
			g.ShowBattle(foe, species, foes, lost != "lost")
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
	// The window class matches the desktop entry (StartupWMClass): docks and
	// task bars show it with the entry's icon, Wayland included.
	opts := &ebiten.RunGameOptions{X11ClassName: "apogee", X11InstanceName: "apogee"}
	if err := ebiten.RunGameWithOptions(wrapped, opts); err != nil && !errors.Is(err, errDone) {
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

// grab copies the screen, or a centred area of it ("WxH").
func grab(screen *ebiten.Image, area string) *image.RGBA {
	b := screen.Bounds()
	full := image.NewRGBA(b)
	screen.ReadPixels(full.Pix)
	var w, h int
	if _, err := fmt.Sscanf(area, "%dx%d", &w, &h); err != nil || w <= 0 || h <= 0 {
		return full
	}
	r := image.Rect(0, 0, w, h).Add(image.Pt((b.Dx()-w)/2, (b.Dy()-h)/2)).Intersect(b)
	out := image.NewRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	draw.Draw(out, out.Bounds(), full, r.Min, draw.Src)
	return out
}

func savePNG(path string, img *image.RGBA) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

// saveGIF writes redraws as an animated GIF, at about fifteen a second, in
// the 256 colours they use most.
func saveGIF(path string, frames []*image.RGBA) error {
	count := map[color.RGBA]int{}
	for _, fr := range frames {
		for i := 0; i < len(fr.Pix); i += 4 {
			count[color.RGBA{fr.Pix[i], fr.Pix[i+1], fr.Pix[i+2], 255}]++
		}
	}
	cols := make([]color.RGBA, 0, len(count))
	for c := range count {
		cols = append(cols, c)
	}
	sort.Slice(cols, func(i, j int) bool { return count[cols[i]] > count[cols[j]] })
	pal := make(color.Palette, 0, 256)
	for _, c := range cols[:min(len(cols), 256)] {
		pal = append(pal, c)
	}
	anim := &gif.GIF{}
	for _, fr := range frames {
		p := image.NewPaletted(fr.Bounds(), pal)
		draw.Draw(p, p.Bounds(), fr, image.Point{}, draw.Src)
		anim.Image = append(anim.Image, p)
		anim.Delay = append(anim.Delay, 7)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return gif.EncodeAll(f, anim)
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

package client

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"io"
	"log/slog"
	"math"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"apogee/internal/art"
	"apogee/internal/discovery"
	"apogee/internal/netclient"
	"apogee/internal/proto"
	"apogee/internal/server"
	"apogee/internal/sound"
	"apogee/internal/update"
)

// App is the whole client: the home screen, the connection, then the game.
// It is what Ebitengine runs.
type App struct {
	Options AppOptions

	settings Settings
	screen   appScreen
	game     *Game
	conn     *netclient.Conn

	// The embedded server of a solo or hosted sky, if any.
	stopServer func()

	// Home screen state.
	name     field
	address  field
	servers  []discovery.Server
	browsing bool
	status   string
	pending  chan launchResult
	buttons  []button
	backdrop backdrop
	outW     int
	outH     int
	dsf      float64
	mu       sync.Mutex // guards servers, browsing and newer, filled by goroutines
	// newer is a version published since this one, once GitHub told.
	newer *update.Release
	quit  bool
	// failed is the error of the last launch, if it failed.
	failed error
	// target is the address of the sky in play; lost follows the attempts
	// to reconnect to it after the connection dropped (connection.go).
	target string
	lost   reconnection

	// OnFrame, when set, is called after each redraw of the home screen
	// with the screen image. The screenshot mode uses it.
	OnFrame func(screen *ebiten.Image)
}

// LaunchError returns the error of the last launch, if it failed: the
// command line then gives up instead of waiting for a game that will not
// come.
func (a *App) LaunchError() error { return a.failed }

// Game returns the game in progress, or nil on the home screen.
func (a *App) Game() *Game { return a.game }

// AppOptions tune how the client starts.
type AppOptions struct {
	Seed   uint64
	Rivals int
	Speed  int
	// Configure is called on every new game, to apply command-line options.
	Configure func(*Game)
	// Sound, when set, plays the effects and the music.
	Sound *sound.Engine
	// Version is the game's; with CheckUpdate, the app asks GitHub at
	// launch whether a newer one was published.
	Version     string
	CheckUpdate bool
}

type appScreen int

const (
	homeScreen appScreen = iota
	joinScreen
	connectingScreen
	playingScreen
)

type launchResult struct {
	conn *netclient.Conn
	stop func()
	err  error
	// target is the address dialed, to reconnect to; remember is set for
	// a sky joined over the network, to offer it again from home.
	target   string
	remember bool
}

// field is a one-line text input.
type field struct {
	value   string
	focused bool
	max     int
}

// NewApp prepares the home screen.
func NewApp(opts AppOptions) *App {
	a := &App{Options: opts, settings: LoadSettings()}
	a.name = field{value: a.settings.Name, max: 24}
	a.address = field{value: fmt.Sprintf("127.0.0.1:%d", proto.DefaultPort), max: 64}
	if a.settings.LastServer != "" {
		a.address.value = a.settings.LastServer
	}
	a.backdrop = newBackdrop(opts.Seed)
	a.applySound()
	if opts.CheckUpdate {
		// At launch, then every few hours for a game left running.
		go update.Watch(context.Background(), opts.Version, update.Every, update.Check, func(r update.Release) {
			a.mu.Lock()
			a.newer = &r
			a.mu.Unlock()
		})
	}
	return a
}

// Start skips the home screen and launches directly, for the command line:
// "solo", "fresh", "host", or an address to join.
func (a *App) Start(how string) {
	switch how {
	case "solo":
		a.launch(func() launchResult { return a.solo(false) })
	case "fresh":
		a.launch(func() launchResult { return a.solo(true) })
	case "host":
		a.launch(a.host)
	default:
		addr := how
		a.launch(func() launchResult { return a.join(addr) })
	}
}

// Close ends the session and stops the embedded server after its last save.
func (a *App) Close() {
	if a.conn != nil {
		a.conn.Close()
	}
	if a.stopServer != nil {
		a.stopServer()
		a.stopServer = nil
	}
}

// launch connects in the background; the result arrives in Update.
func (a *App) launch(f func() launchResult) {
	a.settings.Name = cleanName(a.name.value)
	a.settings.Save()
	a.screen = connectingScreen
	a.status = ""
	a.pending = make(chan launchResult, 1)
	go func() { a.pending <- f() }()
}

func cleanName(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "Dieu"
	}
	return s
}

// embedded starts a server in this process and returns how to stop it.
func (a *App) embedded(cfg server.Config, ln net.Listener) (func(), error) {
	cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg.Seed, cfg.Speed = a.Options.Seed, a.Options.Speed
	if cfg.Rivals == 0 {
		cfg.Rivals = a.Options.Rivals
	}
	srv, err := server.New(cfg)
	if err != nil {
		ln.Close()
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	ran := make(chan struct{})
	go func() {
		defer close(ran)
		srv.Run(ctx)
	}()
	go srv.Serve(ctx, ln)
	return func() {
		cancel()
		<-ran // the last save happens when Run ends
	}, nil
}

func (a *App) dial(addr string, stop func()) launchResult {
	key, err := netclient.PlayerKey()
	if err == nil {
		var conn *netclient.Conn
		conn, err = netclient.Dial(context.Background(), addr, a.settings.Name, key)
		if err == nil {
			return launchResult{conn: conn, stop: stop, target: addr}
		}
	}
	if stop != nil {
		stop()
	}
	return launchResult{err: err}
}

// solo plays alone on an embedded server, on the loopback (GDD 10.3).
func (a *App) solo(fresh bool) launchResult {
	path, exists := SoloSave()
	if fresh && exists {
		os.Remove(path)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return launchResult{err: err}
	}
	stop, err := a.embedded(server.Config{Name: "Ciel solo", DataFile: path}, ln)
	if err != nil {
		return launchResult{err: err}
	}
	return a.dial(ln.Addr().String(), stop)
}

// host opens a sky on the local network, announced with mDNS, and joins it.
func (a *App) host() launchResult {
	dir, err := Dir()
	if err != nil {
		return launchResult{err: err}
	}
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", proto.DefaultPort))
	if err != nil {
		return launchResult{err: fmt.Errorf("le port %d est déjà pris : %w", proto.DefaultPort, err)}
	}
	name := "Ciel de " + a.settings.Name
	stop, err := a.embedded(server.Config{Name: name, DataFile: filepath.Join(dir, "hosted.json")}, ln)
	if err != nil {
		return launchResult{err: err}
	}
	if ad, err := discovery.Advertise(name, proto.DefaultPort); err == nil {
		inner := stop
		stop = func() {
			ad.Stop()
			inner()
		}
	}
	return a.dial(fmt.Sprintf("127.0.0.1:%d", proto.DefaultPort), stop)
}

func (a *App) join(addr string) launchResult {
	r := a.dial(addr, nil)
	r.remember = true
	return r
}

// browse looks for skies on the network, in the background.
func (a *App) browse() {
	a.mu.Lock()
	if a.browsing {
		a.mu.Unlock()
		return
	}
	a.browsing = true
	a.mu.Unlock()
	go func() {
		found, _ := discovery.Browse(context.Background(), 3*time.Second)
		a.mu.Lock()
		a.servers, a.browsing = found, false
		a.mu.Unlock()
	}()
}

// Update runs the current screen.
func (a *App) Update() error {
	if a.quit {
		return ebiten.Termination
	}
	switch a.screen {
	case playingScreen:
		a.watchConnection()
		if a.game == nil {
			return nil // the player gave up
		}
		if r := a.Newer(); r != nil && (a.game.Newer == nil || a.game.Newer.Version != r.Version) {
			a.game.Newer = r
			a.game.dirty = true
			if a.game.Notify != nil && !a.game.isFocused() {
				a.game.Notify("Apogée " + r.Version + " est disponible.")
			}
		}
		if err := a.game.Update(); err != nil {
			// The sky is lost: back to the home screen, with the reason.
			a.Close()
			a.game, a.conn = nil, nil
			a.screen = homeScreen
			a.status = "La connexion au ciel est perdue."
			a.failed = fmt.Errorf("the connection to the sky was lost: %w", err)
		}
		return nil
	case connectingScreen:
		select {
		case r := <-a.pending:
			if r.err != nil {
				a.screen = homeScreen
				a.status = "Impossible de rejoindre le ciel : " + r.err.Error()
				a.failed = r.err
				return nil
			}
			a.conn, a.stopServer = r.conn, r.stop
			a.game = NewGame(Listen(r.conn), a.Options.Seed)
			a.target, a.lost = r.target, reconnection{}
			if r.remember {
				// Remember the sky, to come back to it from the home screen.
				a.settings.LastServer, a.settings.LastSky = r.target, r.conn.Welcome.Sky
				a.address.value = r.target
				a.settings.Save()
			}
			a.game.OnGiveUp = a.giveUp
			a.game.Sound = a.Options.Sound
			a.game.OnMute = func(m bool) {
				a.settings.Muted = m
				a.settings.Save()
			}
			a.game.tutorial = !a.settings.TutorialDone
			a.game.onTutorialDone = func() {
				a.settings.TutorialDone = true
				a.settings.Save()
			}
			if a.Options.Configure != nil {
				a.Options.Configure(a.game)
			}
			a.screen = playingScreen
		default:
		}
	}
	if s := a.Options.Sound; s != nil && a.screen != playingScreen {
		s.SetMood(sound.Mood{}) // the calm of the home screen
	}
	a.backdrop.update()
	a.editFields()
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		x, y := ebiten.CursorPosition()
		a.name.focused, a.address.focused = false, false
		for i := len(a.buttons) - 1; i >= 0; i-- {
			if b := a.buttons[i]; image.Pt(x, y).In(b.r) {
				if s := a.Options.Sound; s != nil {
					s.Play(sound.Click)
				}
				b.action()
				break
			}
		}
	}
	return nil
}

// editFields types into the focused text field.
func (a *App) editFields() {
	for _, f := range []*field{&a.name, &a.address} {
		if !f.focused {
			continue
		}
		for _, r := range ebiten.AppendInputChars(nil) {
			if utf8.RuneCountInString(f.value) < f.max && r >= ' ' {
				f.value += string(r)
			}
		}
		if repeat(ebiten.KeyBackspace) && f.value != "" {
			_, size := utf8.DecodeLastRuneInString(f.value)
			f.value = f.value[:len(f.value)-size]
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyTab) {
			f.focused = false
		}
	}
}

// repeat reports a key press, repeated while the key is held down.
func repeat(k ebiten.Key) bool {
	d := inpututil.KeyPressDuration(k)
	return d == 1 || (d > 30 && d%4 == 0)
}

// Draw draws the current screen: the sky, and on its left the column of
// the menu (the design's home screen).
func (a *App) Draw(screen *ebiten.Image) {
	if a.screen == playingScreen {
		a.game.Draw(screen)
		return
	}
	screen.Clear()
	a.backdrop.draw(screen, a.outW, a.outH, a.uiScale())
	a.buttons = a.buttons[:0]
	p := painter{dst: screen, k: a.uiScale(), buttons: &a.buttons}
	x := p.px(72)
	if a.outW < p.px(640) {
		x = p.px(24)
	}
	w := min(p.px(440), a.outW-x-p.px(24))
	y := p.px(56)
	p.styled("Un dieu · un peuple · une chute", x, y, dimColor, small)
	y += p.lineOf(small) + p.px(8)
	p.styled("Apogée", x+p.px(4), y+p.px(4), inkColor, hero) // its shadow
	p.styled("Apogée", x, y, goldColor, hero)
	y += p.lineOf(hero) + p.px(8)
	for _, l := range p.wrap("Souffle tes intentions à ton peuple. Porte-le au plus haut, avant sa chute.", w-p.px(40)) {
		p.text(l, x, y, dimColor)
		y += p.line()
	}
	y += p.px(24)
	col := image.Rect(x, y, x+w, y)
	switch a.screen {
	case connectingScreen:
		p.styled("Ouverture du ciel…", x, y, goldColor, large)
		y += p.lineOf(large)
	case homeScreen:
		y = a.drawHome(p, col)
	case joinScreen:
		y = a.drawJoin(p, col)
	}
	if r := a.Newer(); r != nil {
		a.drawNewer(p, r)
	}
	if a.status != "" {
		y += p.px(16)
		for _, l := range p.wrap(a.status, w) {
			p.text(l, x, y, alertText)
			y += p.line()
		}
	}
	if a.OnFrame != nil {
		a.OnFrame(screen)
	}
}

// Newer returns the version published since this one, if any.
func (a *App) Newer() *update.Release {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.newer
}

// drawNewer tells, in the top right corner of the home screen, that a newer
// version is out, with its page.
func (a *App) drawNewer(p painter, r *update.Release) {
	w := min(p.px(380), a.outW-p.px(48))
	title := "Apogée " + r.Version + " est disponible"
	sub := "Tu joues à la version " + a.Options.Version + "."
	link := strings.TrimPrefix(r.URL, "https://")
	bh := p.px(34)
	h := p.px(20) + 2*p.line() + p.lineOf(small) + p.px(12) + bh
	box := image.Rect(a.outW-p.px(24)-w, p.px(24), a.outW-p.px(24), p.px(24)+h)
	in := p.panelEdged(box, aetherColor)
	p.styled(title, in.Min.X, in.Min.Y, aetherColor, bold)
	p.text(sub, in.Min.X, in.Min.Y+p.line(), dimColor)
	p.styled(link, in.Min.X, in.Min.Y+2*p.line()+p.px(2), dimColor, small)
	p.buttonAs(image.Rect(in.Min.X, in.Max.Y-bh, in.Max.X, in.Max.Y), "Ouvrir la page de téléchargement", func() {
		a.openRelease(r)
	}, primaryButton, panelEdge)
}

// openRelease opens the page of a release in the browser, or says where it
// is when no browser opens.
func (a *App) openRelease(r *update.Release) {
	if err := update.Open(r.URL); err != nil {
		a.status = "Ouvre " + r.URL + " pour mettre Apogée à jour."
	}
}

// heading draws the small title of a group of the menu.
func heading(p painter, s string, x, y int) int {
	p.styled(s, x, y, dimColor, small)
	return y + p.lineOf(small) + p.px(6)
}

// drawField draws a text field and makes it clickable: a line of text over
// a gold rule.
func (a *App) drawField(p painter, r image.Rectangle, f *field) {
	edge := goldColor
	if !f.focused {
		edge = panelEdge
	}
	p.fill(image.Rect(r.Min.X, r.Max.Y-p.px(2), r.Max.X, r.Max.Y), edge)
	v := f.value
	if f.focused && (time.Now().UnixMilli()/500)%2 == 0 {
		v += "_"
	}
	p.styled(v, r.Min.X, r.Min.Y+(r.Dy()-p.px(2)-p.line())/2, textColor, bold)
	*p.buttons = append(*p.buttons, button{r: r, action: func() { f.focused = true }})
}

// drawHome draws the menu and returns where it ends.
func (a *App) drawHome(p painter, r image.Rectangle) int {
	x, w := r.Min.X, r.Dx()
	y := r.Min.Y
	// The god's name.
	nameBox := image.Rect(x, y, x+p.px(260), y+p.px(40))
	p.fill(nameBox, buttonFill)
	p.frame(nameBox, 2, panelEdge)
	p.styled("Dieu", x+p.px(12), y+(p.px(40)-p.lineOf(small))/2, dimColor, small)
	a.drawField(p, image.Rect(x+p.px(56), y+p.px(6), nameBox.Max.X-p.px(12), nameBox.Max.Y-p.px(6)), &a.name)
	y += p.px(40) + p.px(24)

	big, bh := p.px(48), p.px(40)
	_, hasSolo := SoloSave()
	y = heading(p, "Ton ciel", x, y)
	solo := func(fresh bool) func() {
		return func() { a.launch(func() launchResult { return a.solo(fresh) }) }
	}
	if hasSolo {
		p.buttonAs(image.Rect(x, y, x+w, y+big), "Continuer ma partie", solo(false), primaryButton, panelEdge)
		y += big + p.px(8)
		p.button(image.Rect(x, y, x+w, y+bh), "Nouvelle partie solo", solo(true))
	} else {
		p.buttonAs(image.Rect(x, y, x+w, y+big), "Jouer en solo", solo(true), primaryButton, panelEdge)
		y += big - bh
	}
	y += bh + p.px(24)

	y = heading(p, "Ciels partagés", x, y)
	if last := a.settings.LastServer; last != "" {
		label := a.settings.LastSky
		if label == "" {
			label = last
		}
		if r := []rune(label); len(r) > 24 {
			label = string(r[:23]) + "…"
		}
		p.buttonAs(image.Rect(x, y, x+w, y+bh), "Revenir sur « "+label+" »", func() {
			a.launch(func() launchResult { return a.join(last) })
		}, ghostButton, aetherColor)
		y += bh + p.px(8)
	}
	half := (w - p.px(8)) / 2
	p.button(image.Rect(x, y, x+half, y+bh), "Héberger", func() { a.launch(a.host) })
	p.button(image.Rect(x+w-half, y, x+w, y+bh), "Rejoindre…", func() {
		a.screen, a.status = joinScreen, ""
		a.browse()
	})
	y += bh + p.px(32)

	// The settings, on one line.
	level, tune := "Son coupé", "Musique non"
	if !a.settings.Muted {
		level = "Son " + volumeNames[a.settings.Volume]
	}
	if a.settings.Music {
		tune = "Musique oui"
	}
	var cycle, music func()
	if a.Options.Sound != nil {
		cycle, music = a.cycleVolume, a.toggleMusic
	} else {
		// No sound device, or -sound=false: say so rather than a level.
		level, tune = "Son indisponible", "Musique indisponible"
	}
	sx := x
	small := func(label string, action func(), kind buttonKind) {
		bw := p.width(label) + p.px(24)
		p.buttonAs(image.Rect(sx, y, sx+bw, y+p.px(34)), label, action, kind, panelEdge)
		sx += bw + p.px(6)
	}
	small(level, cycle, plainButton)
	small(tune, music, plainButton)
	small("Revoir le tutoriel", a.resetTutorial(), ghostButton)
	quit := "Quitter"
	qw := p.width(quit) + p.px(24)
	p.buttonAs(image.Rect(max(sx, x+w-qw), y, max(sx, x+w-qw)+qw, y+p.px(34)), quit, func() { a.quit = true }, ghostButton, dimColor)
	return y + p.px(34)
}

// applySound gives the sound engine the player's settings.
func (a *App) applySound() {
	s := a.Options.Sound
	if s == nil {
		return
	}
	s.SetVolume(volumes[a.settings.Volume])
	s.SetMuted(a.settings.Muted)
	s.SetMusic(a.settings.Music)
}

// cycleVolume goes from low to loud, then silent, then low again.
func (a *App) cycleVolume() {
	switch {
	case a.settings.Muted:
		a.settings.Muted, a.settings.Volume = false, 1
	case a.settings.Volume == len(volumes)-1:
		a.settings.Muted = true
	default:
		a.settings.Volume++
	}
	a.settings.Save()
	a.applySound()
}

func (a *App) toggleMusic() {
	a.settings.Music = !a.settings.Music
	a.settings.Save()
	a.applySound()
}

func (a *App) resetTutorial() func() {
	if !a.settings.TutorialDone {
		return nil // it will show anyway
	}
	return func() {
		a.settings.TutorialDone = false
		a.settings.Save()
		a.status = "Le tutoriel s'affichera au début de ta prochaine partie."
	}
}

// drawJoin lists the skies found on the network, and an address to join;
// it returns where it ends.
func (a *App) drawJoin(p painter, r image.Rectangle) int {
	x, w := r.Min.X, r.Dx()
	bh := p.px(40)
	gap := p.px(8)
	a.mu.Lock()
	servers, browsing := append([]discovery.Server(nil), a.servers...), a.browsing
	found := len(servers)
	a.mu.Unlock()
	y := heading(p, "Ciels découverts sur le réseau", x, r.Min.Y)
	// Only as many skies as the window holds: the address field and the
	// buttons below must stay reachable.
	fixed := 2*bh + 3*gap + p.lineOf(small) + p.px(90)
	room := max(1, (a.outH-y-fixed)/(bh+gap))
	hidden := 0
	if len(servers) > room {
		hidden = len(servers) - (room - 1)
		servers = servers[:room-1]
	}
	switch {
	case browsing:
		p.text("Recherche en cours…", x, y, dimColor)
		y += p.line() + gap
	case found == 0:
		for _, l := range p.wrap("Aucun ciel trouvé : le réseau bloque peut-être la découverte.", w) {
			p.text(l, x, y, dimColor)
			y += p.line()
		}
		y += gap
	}
	for _, s := range servers {
		addr := s.Addr
		p.buttonAs(image.Rect(x, y, x+w, y+bh), s.Name, func() {
			a.launch(func() launchResult { return a.join(addr) })
		}, ghostButton, aetherColor)
		y += bh + gap
	}
	if hidden > 0 {
		msg := fmt.Sprintf("… et %d autres ciels : agrandis la fenêtre pour les voir.", hidden)
		if len(servers) == 0 {
			msg = fmt.Sprintf("%d ciels trouvés : agrandis la fenêtre pour les voir.", hidden)
		}
		p.text(msg, x, y, dimColor)
		y += p.line() + gap
	}
	var refresh func()
	if !browsing {
		refresh = a.browse
	}
	p.button(image.Rect(x, y, x+w, y+bh), "Chercher à nouveau", refresh)
	y += bh + p.px(24)
	y = heading(p, "Ou rejoindre une adresse", x, y)
	a.drawField(p, image.Rect(x, y, x+w, y+p.px(32)), &a.address)
	y += p.px(32) + p.px(16)
	half := (w - gap) / 2
	p.buttonAs(image.Rect(x, y, x+half, y+bh), "Rejoindre", func() {
		addr := strings.TrimSpace(a.address.value)
		a.launch(func() launchResult { return a.join(addr) })
	}, primaryButton, panelEdge)
	p.buttonAs(image.Rect(x+w-half, y, x+w, y+bh), "Retour", func() { a.screen = homeScreen }, ghostButton, panelEdge)
	return y + bh
}

// Layout follows the game's: physical pixels.
func (a *App) Layout(w, h int) (int, int) { return w, h }

// LayoutF converts to physical pixels, as the game does.
func (a *App) LayoutF(w, h float64) (float64, float64) {
	if a.screen == playingScreen {
		return a.game.LayoutF(w, h)
	}
	f := 1.0
	if m := ebiten.Monitor(); m != nil {
		f = m.DeviceScaleFactor()
	}
	a.dsf = f
	a.outW, a.outH = int(math.Round(w*f)), int(math.Round(h*f))
	return float64(a.outW), float64(a.outH)
}

func (a *App) uiScale() float64 { return math.Max(1, math.Round(max(a.dsf, 1))) }

// backdrop is the home screen's sky: the portal turning on the right, and
// a shade on the left under the menu.
type backdrop struct {
	sky    *ebiten.Image
	shade  *ebiten.Image
	portal []*ebiten.Image
	frame  int
}

func newBackdrop(seed uint64) backdrop {
	grad := image.NewRGBA(image.Rect(0, 0, 1, 256))
	art.FillSky(grad)
	b := backdrop{sky: ebiten.NewImageFromImage(grad)}
	// The shade fades from the menu's ink to nothing.
	shade := image.NewNRGBA(image.Rect(0, 0, 256, 1))
	for x := range 256 {
		t := float64(x) / 255
		alpha := 0.92
		if t > 0.55 {
			alpha = 0.92 * (1 - (t-0.55)/0.45)
		}
		shade.SetNRGBA(x, 0, color.NRGBA{0x14, 0x12, 0x2b, uint8(alpha * 255)})
	}
	b.shade = ebiten.NewImageFromImage(shade)
	for i := range portalFrames {
		sp := art.PortalFrame(art.DefaultCamera, 90, 3, float64(i)/portalFrames)
		b.portal = append(b.portal, ebiten.NewImageFromImage(sp.Img))
	}
	return b
}

func (b *backdrop) update() { b.frame = portalFrame(time.Now().UnixMilli()) }

func (b *backdrop) draw(screen *ebiten.Image, w, h int, k float64) {
	var op ebiten.DrawImageOptions
	op.GeoM.Scale(float64(w), float64(h)/256)
	screen.DrawImage(b.sky, &op)
	img := b.portal[b.frame]
	s := 2.5 * k
	var po ebiten.DrawImageOptions
	po.GeoM.Translate(-float64(img.Bounds().Dx())/2, -float64(img.Bounds().Dy())/2)
	po.GeoM.Scale(s, s)
	po.GeoM.Translate(math.Round(float64(w)*0.7), math.Round(float64(h)*0.45))
	screen.DrawImage(img, &po)
	var so ebiten.DrawImageOptions
	so.GeoM.Scale(float64(w)*0.62/256, float64(h))
	screen.DrawImage(b.shade, &so)
}

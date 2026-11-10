package client

import (
	"context"
	"fmt"
	"image"
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
	"github.com/hajimehoshi/ebiten/v2/text/v2"

	"apogee/internal/art"
	"apogee/internal/discovery"
	"apogee/internal/netclient"
	"apogee/internal/proto"
	"apogee/internal/server"
	"apogee/internal/sound"
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
	mu       sync.Mutex // guards servers and browsing, filled by a goroutine
	quit     bool
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

// Draw draws the current screen.
func (a *App) Draw(screen *ebiten.Image) {
	if a.screen == playingScreen {
		a.game.Draw(screen)
		return
	}
	screen.Clear()
	a.backdrop.draw(screen, a.outW, a.outH, a.uiScale())
	a.buttons = a.buttons[:0]
	p := painter{dst: screen, k: a.uiScale(), buttons: &a.buttons}
	k := int(p.k)
	title := "APOGÉE"
	tk := p.k * 4
	tw := textWidth(title) * tk
	var op text.DrawOptions
	op.GeoM.Scale(tk, tk)
	op.GeoM.Translate(math.Round((float64(a.outW)-tw)/2), float64(40*k))
	op.ColorScale.ScaleWithColor(goldColor)
	text.Draw(screen, title, face, &op)
	tag := "Souffle tes intentions à ton peuple. Porte-le au plus haut, avant sa chute."
	p.text(tag, (a.outW-p.width(tag))/2, 40*k+int(lineHeight*tk)+8*k, textColor)

	w := 380 * k
	top := 40*k + int(lineHeight*tk) + 40*k
	switch a.screen {
	case connectingScreen:
		msg := "Ouverture du ciel…"
		p.text(msg, (a.outW-p.width(msg))/2, a.outH/2, goldColor)
	case homeScreen:
		a.drawHome(p, image.Rect((a.outW-w)/2, top, (a.outW+w)/2, top), k)
	case joinScreen:
		a.drawJoin(p, image.Rect((a.outW-w)/2, top, (a.outW+w)/2, top), k)
	}
	if a.status != "" {
		for i, l := range p.wrap(a.status, w*3/2) {
			p.text(l, (a.outW-p.width(l))/2, a.outH-(3-i)*p.line()-20*k, alertColor)
		}
	}
	if a.OnFrame != nil {
		a.OnFrame(screen)
	}
}

func textWidth(s string) float64 {
	w, _ := text.Measure(s, face, lineHeight)
	return w
}

// drawField draws a text field and makes it clickable.
func (a *App) drawField(p painter, r image.Rectangle, f *field) {
	p.fill(r, barBack)
	edge := panelEdge
	if f.focused {
		edge = goldColor
	}
	p.strokeRect(r, edge)
	v := f.value
	if f.focused && (time.Now().UnixMilli()/500)%2 == 0 {
		v += "_"
	}
	p.text(v, r.Min.X+int(6*p.k), r.Min.Y+(r.Dy()-p.line())/2+int(p.k), textColor)
	*p.buttons = append(*p.buttons, button{r, func() { f.focused = true }})
}

func (a *App) drawHome(p painter, r image.Rectangle, k int) {
	bh := p.line() + 10*k
	gap := 8 * k
	_, hasSolo := SoloSave()
	rows := 7
	if hasSolo {
		rows++
	}
	last := a.settings.LastServer
	if last != "" {
		rows++
	}
	r.Max.Y = r.Min.Y + p.line() + gap + bh + gap*3 + rows*(bh+gap) + 12*k
	in := p.panel(r)
	p.text("Ton nom de dieu", in.Min.X, in.Min.Y, goldColor)
	y := in.Min.Y + p.line() + gap/2
	a.drawField(p, image.Rect(in.Min.X, y, in.Max.X, y+bh), &a.name)
	y += bh + gap*3
	btn := func(label string, action func()) {
		p.button(image.Rect(in.Min.X, y, in.Max.X, y+bh), label, action)
		y += bh + gap
	}
	if hasSolo {
		btn("Continuer ma partie solo", func() { a.launch(func() launchResult { return a.solo(false) }) })
		btn("Nouvelle partie solo", func() { a.launch(func() launchResult { return a.solo(true) }) })
	} else {
		btn("Jouer en solo", func() { a.launch(func() launchResult { return a.solo(true) }) })
	}
	if last != "" {
		label := a.settings.LastSky
		if label == "" {
			label = last
		}
		if r := []rune(label); len(r) > 24 {
			label = string(r[:23]) + "…"
		}
		btn("Revenir sur « "+label+" »", func() { a.launch(func() launchResult { return a.join(last) }) })
	}
	btn("Héberger un ciel sur le réseau", func() { a.launch(a.host) })
	btn("Rejoindre un ciel", func() {
		a.screen, a.status = joinScreen, ""
		a.browse()
	})
	btn("Revoir le tutoriel au prochain lancement", a.resetTutorial())
	level, tune := "coupé", "non"
	if !a.settings.Muted {
		level = volumeNames[a.settings.Volume]
	}
	if a.settings.Music {
		tune = "oui"
	}
	var cycle, music func()
	if a.Options.Sound != nil {
		cycle, music = a.cycleVolume, a.toggleMusic
	} else {
		// No sound device, or -sound=false: say so rather than a level.
		level, tune = "indisponible", "indisponible"
	}
	btn("Son : "+level, cycle)
	btn("Musique : "+tune, music)
	btn("Quitter", func() { a.quit = true })
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

func (a *App) drawJoin(p painter, r image.Rectangle, k int) {
	bh := p.line() + 10*k
	gap := 8 * k
	a.mu.Lock()
	servers, browsing := append([]discovery.Server(nil), a.servers...), a.browsing
	found := len(servers)
	a.mu.Unlock()
	// Only as many skies as the window holds: the address field and the
	// buttons below must stay reachable.
	fixed := 3*p.line() + 3*(bh+gap) + bh + 30*k
	// The rows the window holds; in a tiny one, a single line says how many
	// skies were found.
	room := max(1, (a.outH-r.Min.Y-fixed-2*p.line())/(bh+gap))
	hidden := 0
	if len(servers) > room {
		hidden = len(servers) - (room - 1)
		servers = servers[:room-1]
	}
	rows := max(1, len(servers))
	if hidden > 0 {
		rows++
	}
	r.Max.Y = r.Min.Y + fixed + rows*(bh+gap)
	in := p.panel(r)
	p.text("Ciels découverts sur le réseau", in.Min.X, in.Min.Y, goldColor)
	y := in.Min.Y + p.line() + gap/2
	switch {
	case browsing:
		p.text("Recherche en cours…", in.Min.X, y, dimColor)
		y += bh + gap
	case found == 0:
		p.text("Aucun ciel trouvé : le réseau bloque peut-être la découverte.", in.Min.X, y, dimColor)
		y += bh + gap
	}
	for _, s := range servers {
		addr := s.Addr
		p.button(image.Rect(in.Min.X, y, in.Max.X, y+bh), s.Name, func() {
			a.launch(func() launchResult { return a.join(addr) })
		})
		y += bh + gap
	}
	if hidden > 0 {
		msg := fmt.Sprintf("… et %d autres ciels : agrandis la fenêtre pour les voir.", hidden)
		if len(servers) == 0 {
			msg = fmt.Sprintf("%d ciels trouvés : agrandis la fenêtre pour les voir.", hidden)
		}
		p.text(msg, in.Min.X, y, dimColor)
		y += bh + gap
	}
	var refresh func()
	if !browsing {
		refresh = a.browse
	}
	p.button(image.Rect(in.Min.X, y, in.Max.X, y+bh), "Chercher à nouveau", refresh)
	y += bh + gap*2
	p.text("Ou rejoindre une adresse", in.Min.X, y, goldColor)
	y += p.line() + gap/2
	a.drawField(p, image.Rect(in.Min.X, y, in.Max.X, y+bh), &a.address)
	y += bh + gap
	half := (in.Dx() - gap) / 2
	p.button(image.Rect(in.Min.X, y, in.Min.X+half, y+bh), "Rejoindre", func() {
		addr := strings.TrimSpace(a.address.value)
		a.launch(func() launchResult { return a.join(addr) })
	})
	p.button(image.Rect(in.Max.X-half, y, in.Max.X, y+bh), "Retour", func() { a.screen = homeScreen })
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

// backdrop is the home screen's sky: the portal turning among the clouds.
type backdrop struct {
	sky    *ebiten.Image
	portal []*ebiten.Image
	frame  int
}

func newBackdrop(seed uint64) backdrop {
	grad := image.NewRGBA(image.Rect(0, 0, 1, 256))
	art.FillSky(grad)
	b := backdrop{sky: ebiten.NewImageFromImage(grad)}
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
	s := 2 * k
	var po ebiten.DrawImageOptions
	po.GeoM.Translate(-float64(img.Bounds().Dx())/2, -float64(img.Bounds().Dy())/2)
	po.GeoM.Scale(s, s)
	po.GeoM.Translate(math.Round(float64(w)/2), math.Round(float64(h)*0.8))
	po.ColorScale.ScaleAlpha(0.55) // behind the menu, discreet
	screen.DrawImage(img, &po)
}

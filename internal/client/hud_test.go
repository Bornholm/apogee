package client

import (
	"testing"
	"time"

	"apogee/internal/proto"
	"apogee/internal/sim"
	"apogee/internal/sound"
)

func TestAdjustKeepsThePieWhole(t *testing.T) {
	g := &Game{draft: sim.DefaultPie} // Prosper 60, Fortify 40
	g.adjust(sim.Study, +10)()
	if g.draft[sim.Study] != 10 || g.draft[sim.Prosper] != 50 || !g.drafting {
		t.Fatalf("after +10 Study: %v", g.draft)
	}
	if err := g.draft.Validate(); err != nil {
		t.Fatal(err)
	}
	g.adjust(sim.Fortify, -10)()
	if g.draft[sim.Fortify] != 30 || g.draft.Validate() != nil {
		t.Fatalf("after -10 Fortify: %v", g.draft)
	}
}

func TestAdjustRefusesImpossibleMoves(t *testing.T) {
	g := &Game{draft: sim.Pie{sim.Prosper: 40, sim.Fortify: 30, sim.Study: 30}}
	if g.adjust(sim.Arm, +10) != nil {
		t.Error("a fourth slice was allowed")
	}
	if g.adjust(sim.Arm, -10) != nil {
		t.Error("an empty slice went below zero")
	}
	g = &Game{draft: sim.Pie{sim.Study: 100}}
	if g.adjust(sim.Study, -10) != nil {
		t.Error("a single slice gave 10 % to nobody")
	}
	// Room is made from the other side: + on another intention takes it.
	g.adjust(sim.Prosper, +10)()
	if g.draft != (sim.Pie{sim.Study: 90, sim.Prosper: 10}) {
		t.Errorf("after +10 Prosper: %v", g.draft)
	}
}

func TestWrapKeepsFrenchPunctuationWithItsWord(t *testing.T) {
	p := painter{k: 1}
	text := "Sans signe de son dieu, ton peuple a choisi : « Charrue : Vivres +30 % »."
	for w := 60; w < 400; w += 7 {
		for _, l := range p.wrap(text, w) {
			if l == "«" || l == "»." || l == "%" || l == ":" || l[0] == ':' {
				t.Fatalf("width %d: a line holds punctuation alone: %q", w, l)
			}
		}
	}
}

func TestNotificationsOnlyForNewChosenNewsWhileAway(t *testing.T) {
	var got []string
	focused := false
	g := &Game{
		Notify:      func(s string) { got = append(got, s) },
		NotifyKinds: map[sim.EntryKind]bool{sim.RaidNews: true},
		focused:     func() bool { return focused },
	}
	state := func(entries ...sim.Entry) *proto.State {
		return &proto.State{Own: &proto.Own{Chronicle: entries}}
	}
	old := sim.Entry{Tick: 5, Text: "un vieux raid", Kind: sim.RaidNews}
	g.notifyNews(state(old))
	if len(got) != 0 {
		t.Fatalf("notified history at start: %v", got)
	}
	raid := sim.Entry{Tick: 9, Text: "un raid", Kind: sim.RaidNews}
	info := sim.Entry{Tick: 9, Text: "une maison", Kind: sim.Info}
	g.notifyNews(state(old, raid, info))
	if len(got) != 1 || got[0] != "un raid" {
		t.Fatalf("notifications %v, want only the new raid", got)
	}
	g.notifyNews(state(old, raid, info))
	if len(got) != 1 {
		t.Fatalf("the same entry was notified twice: %v", got)
	}
	focused = true
	g.notifyNews(state(old, raid, info, sim.Entry{Tick: 12, Text: "encore", Kind: sim.RaidNews}))
	if len(got) != 1 {
		t.Fatalf("notified while the player looks at the game: %v", got)
	}
}

func TestNewsMakeOneSoundTheWeightiest(t *testing.T) {
	var heard []sound.Effect
	g := &Game{onPlay: func(fx sound.Effect) { heard = append(heard, fx) }}
	state := func(entries ...sim.Entry) *proto.State {
		return &proto.State{Own: &proto.Own{Chronicle: entries}}
	}
	old := sim.Entry{Tick: 5, Text: "un vieux raid", Kind: sim.RaidNews}
	g.notifyNews(state(old))
	if len(heard) != 0 {
		t.Fatalf("history played %v at start", heard)
	}
	built := sim.Entry{Tick: 9, Text: "une maison", Kind: sim.BuiltNews}
	struck := sim.Entry{Tick: 9, Text: "la vague frappe", Kind: sim.WaveNews}
	info := sim.Entry{Tick: 9, Text: "rien", Kind: sim.Info}
	g.notifyNews(state(old, built, struck, info))
	if len(heard) != 1 || heard[0] != sound.Struck {
		t.Fatalf("played %v, want only the blow of the wave", heard)
	}
	g.notifyNews(state(old, built, struck, info, sim.Entry{Tick: 11, Text: "rien", Kind: sim.Info}))
	if len(heard) != 1 {
		t.Fatalf("an entry without sound played %v", heard[1:])
	}
}

// Shortcuts follow the letter typed, whatever the keyboard layout.
func TestShortcutsFollowTheLetterTyped(t *testing.T) {
	g := &Game{}
	g.shortcut('a')
	if g.book != annalsTab {
		t.Fatal("a should open the Annals")
	}
	g.shortcut('q') // where A sits on an AZERTY keyboard's QWERTY code
	if g.book != annalsTab {
		t.Fatal("q closed the Annals")
	}
	g.shortcut('C') // the culture takes the shared panel
	if g.book != cultureTab {
		t.Fatalf("C should turn the panel to the culture: %v", g.book)
	}
	g.shortcut('i')
	if g.book != civTab {
		t.Fatalf("i should turn the panel to the civilization: %v", g.book)
	}
	g.shortcut('i')
	if g.book != noBook {
		t.Fatal("i again should close the panel")
	}
}

func TestEventSoundsAreSpacedOut(t *testing.T) {
	var g Game
	now := time.Unix(1000, 0)
	if !g.mayPlay(sound.Built, now) {
		t.Fatal("the first effect is silent")
	}
	if g.mayPlay(sound.Victory, now.Add(time.Second)) {
		t.Fatal("two events a second apart both sounded")
	}
	if g.mayPlay(sound.Built, now.Add(10*time.Second)) {
		t.Fatal("the same effect again after ten seconds")
	}
	if !g.mayPlay(sound.Victory, now.Add(10*time.Second)) {
		t.Fatal("another event after ten seconds is silent")
	}
	if !g.mayPlay(sound.Click, now.Add(10*time.Second)) {
		t.Fatal("a click waited")
	}
}

func TestThePieShowsOnDemand(t *testing.T) {
	g := &Game{}
	if !g.pieShown() {
		t.Fatal("the pie is hidden in the sky view")
	}
	g.shortcut('s')
	if g.pieShown() {
		t.Fatal("s did not put the pie away")
	}
	g.level = 2
	if g.pieShown() {
		t.Fatal("the pie shows in the island view")
	}
	g.togglePanel(cultureTab)
	g.shortcut('S')
	if !g.pieShown() || g.book != noBook {
		t.Fatalf("S should open the pie over the culture: %v %v", g.pieShown(), g.book)
	}
}

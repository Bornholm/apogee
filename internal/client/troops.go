package client

import (
	"math"

	"apogee/internal/geo"
	"apogee/internal/proto"
	"apogee/internal/sim"
)

// Troops show on the player's island the groups that cross the sky (GDD 7):
// its warriors leaving for a raid and coming home, the relief a neighbour
// sends and that stays on guard, the raiders closing in, and the banners of
// the islands that covet it.

// troopTime is how long a departure or a homecoming takes on the island.
const troopTime = 7.0

// march is a departure or a homecoming of the player's warriors.
type march struct {
	home    bool // coming home; otherwise leaving
	n       int
	shore   geo.Vec2 // where they take off, or land
	t       float64
	bearing float64
}

// allies is a relief force on guard on the island.
type allies struct {
	raid uint32
	n    int
	t    float64 // since they landed
}

// figure is one person of a troop, as the island draws it.
type figure struct {
	x, y, z float64
	kind    int // ownFigure, allyFigure or raiderFigure
}

const (
	ownFigure = iota
	allyFigure
	raiderFigure
)

type troops struct {
	primed  bool
	out     map[uint32]float64 // our raids in the sky: their warriors, by id
	marches []march
	allies  []allies
	// raiders are the hostile groups about to land, by progress.
	raiders []proto.RaidView
	// banners are the islands that covet ours, by the kind of their intent.
	banners []bannerAt
	heart   geo.Vec2
	sh      shore
}

type bannerAt struct {
	kind sim.Outer
	at   geo.Vec2
	// lineage is the coveting island's: its colour and emblem.
	lineage sim.Lineage
}

// observe follows the raids of the sky that concern the player's island.
func (tr *troops) observe(st *proto.State, sum proto.IslandSummary, own *proto.Own) {
	tr.sh = shore{geo.Outline(sum.Seed), sum.Size}
	for _, b := range own.Buildings {
		if b.Kind == sim.Core {
			tr.heart = geo.Vec2{X: b.X, Y: b.Y}
		}
	}
	if tr.out == nil {
		tr.out = map[uint32]float64{}
	}
	present := map[uint32]bool{}
	tr.raiders = tr.raiders[:0]
	var stationed []proto.RaidView
	for _, r := range st.Raids {
		switch {
		case r.From == st.You && r.Kind != sim.Relief:
			present[r.ID] = true
			if _, seen := tr.out[r.ID]; !seen && tr.primed {
				tr.start(false, r)
			}
			tr.out[r.ID] = r.Units
		case r.To == st.You && r.Kind == sim.Relief && r.Progress >= 1 && !r.Returning:
			stationed = append(stationed, r)
		case r.To == st.You && r.Kind != sim.Relief && !r.Returning && r.Progress > 0.75:
			tr.raiders = append(tr.raiders, r)
		}
	}
	// A raid of ours gone from the sky has come home.
	ids := make([]uint32, 0, len(tr.out))
	for id := range tr.out {
		ids = append(ids, id)
	}
	for _, id := range sortIDs(ids) {
		if !present[id] {
			if units := tr.out[id]; units >= 1 {
				tr.start(true, proto.RaidView{ID: id, Units: units})
			}
			delete(tr.out, id)
		}
	}
	// Relief forces on guard, each landing when it arrives.
	kept := tr.allies[:0]
	for _, r := range stationed {
		// A new force lands; one already there when the god first looked
		// is simply on guard.
		a := allies{raid: r.ID, n: min(max(int(r.Units), 1), 4)}
		if !tr.primed {
			a.t = troopTime
		}
		for _, old := range tr.allies {
			if old.raid == r.ID {
				a.t = old.t
			}
		}
		kept = append(kept, a)
	}
	tr.allies = kept
	tr.banners = tr.banners[:0]
	for _, is := range st.Islands {
		if is.BannerTarget == st.You && is.Banner > 0 && is.ID != st.You && len(tr.banners) < 4 {
			a := float64(is.ID) * 2.39996
			edge := tr.edgeAt(a)
			tr.banners = append(tr.banners, bannerAt{is.Banner, edge.Scale(0.9), is.Lineage})
		}
	}
	tr.primed = true
}

func (tr *troops) start(home bool, r proto.RaidView) {
	if len(tr.marches) >= 4 {
		return
	}
	b := float64(r.ID) * 2.39996
	tr.marches = append(tr.marches, march{home: home, n: min(max(int(r.Units), 2), 6),
		shore: tr.edgeAt(b).Scale(0.85), bearing: b})
}

// edgeAt returns the island's shore in a direction.
func (tr *troops) edgeAt(a float64) geo.Vec2 {
	if len(tr.sh.outline) == 0 {
		return geo.Vec2{}
	}
	i := int(math.Mod(a/(2*math.Pi), 1)*float64(len(tr.sh.outline))) % len(tr.sh.outline)
	return tr.sh.outline[i].Scale(tr.sh.size)
}

func (tr *troops) step(dt float64) {
	kept := tr.marches[:0]
	for _, m := range tr.marches {
		if m.t += dt; m.t < troopTime {
			kept = append(kept, m)
		}
	}
	tr.marches = kept
	for i := range tr.allies {
		tr.allies[i].t += dt
	}
}

// active reports whether something moves, for redraws.
func (tr *troops) active() bool {
	return len(tr.marches) > 0 || len(tr.allies) > 0 || len(tr.raiders) > 0
}

// figures returns the people of the troops at this moment.
func (tr *troops) figures(clock float64) []figure {
	var out []figure
	for _, m := range tr.marches {
		k := m.t / troopTime
		for i := range m.n {
			// Each walks between the heart and the shore, then flies.
			off := geo.Vec2{X: float64(i%3-1) * 2, Y: float64(i/3) * 2}
			from := geo.Vec2{X: tr.heart.X + off.X, Y: tr.heart.Y + 3 + off.Y}
			to := geo.Vec2{X: m.shore.X + off.X, Y: m.shore.Y + off.Y}
			walk, fly := ease(k/0.55), ease((k-0.55)/0.45)
			if m.home {
				walk, fly = ease((1-k)/0.55), ease((0.45-k)/0.45)
			}
			p := geo.Vec2{X: from.X + (to.X-from.X)*walk, Y: from.Y + (to.Y-from.Y)*walk}
			out = append(out, figure{x: p.X + math.Cos(m.bearing)*fly*20, y: p.Y + math.Sin(m.bearing)*fly*20,
				z: fly * 35, kind: ownFigure})
		}
	}
	for _, a := range tr.allies {
		for i := range a.n {
			ang := clock*0.12 + float64(a.raid) + float64(i)*2*math.Pi/float64(a.n)
			r := 10 + float64(i%2)*3
			z := 30 * (1 - ease(a.t/3))
			out = append(out, figure{x: tr.heart.X + r*math.Cos(ang), y: tr.heart.Y + r*0.8*math.Sin(ang), z: z, kind: allyFigure})
		}
	}
	for _, r := range tr.raiders {
		// The raiders close in from the sky, until the battle.
		k := ease((r.Progress - 0.75) / 0.25)
		b := float64(r.From) * 2.39996
		edge := tr.edgeAt(b)
		far := edge.Scale(1.9)
		for i := range min(max(int(r.Units), 1), 6) {
			off := float64(i%3-1) * 2.5
			out = append(out, figure{x: far.X + (edge.X-far.X)*k + off, y: far.Y + (edge.Y-far.Y)*k + float64(i/3)*2,
				z: 30 - 18*k, kind: raiderFigure})
		}
	}
	return out
}

// rehearse forces a troop on the island, for screenshots: depart,
// homecoming, relief, threat or raiders.
func (tr *troops) rehearse(name string) {
	switch name {
	case "depart", "homecoming":
		if len(tr.marches) == 0 {
			tr.start(name == "homecoming", proto.RaidView{ID: 7, Units: 5})
		}
	case "relief":
		if len(tr.allies) == 0 {
			tr.allies = []allies{{raid: 9, n: 3}}
		}
	case "threat":
		tr.banners = append(tr.banners[:0], bannerAt{sim.Covet, tr.edgeAt(1).Scale(0.9), sim.Lineage{Colour: 5, Emblem: 3}},
			bannerAt{sim.Annihilate, tr.edgeAt(4).Scale(0.9), sim.Lineage{Colour: 2, Emblem: 1}})
	case "raiders":
		tr.raiders = append(tr.raiders[:0], proto.RaidView{From: 5, Units: 5, Progress: 0.92})
	}
}

func sortIDs(ids []uint32) []uint32 {
	for i := 1; i < len(ids); i++ {
		for j := i; j > 0 && ids[j] < ids[j-1]; j-- {
			ids[j], ids[j-1] = ids[j-1], ids[j]
		}
	}
	return ids
}

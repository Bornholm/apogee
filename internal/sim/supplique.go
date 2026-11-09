package sim

import (
	"errors"
	"fmt"
)

// Suppliques: the people turn to their god for a decision (GDD 6.5). Only one
// waits at a time; unanswered, the people decide by their culture, with no
// penalty.

// Supplique tunables (GDD 🔧).
const (
	suppliqueTimeout = 5 * 60.0  // seconds before the people decide alone
	suppliqueGap     = 10 * 60.0 // minimum time between two ordinary suppliques
	roomCooldown     = 2 * 3600.0
)

// SuppliqueKind tells what a supplique is about.
type SuppliqueKind int

const (
	AskDiscovery SuppliqueKind = iota // the scholars hesitate (GDD 5.3)
	AskRoom                           // the island is too small
	AskFamine                         // the granaries are empty
)

// Supplique is a question awaiting an answer.
type Supplique struct {
	ID       uint32
	Kind     SuppliqueKind
	Question string
	Options  []string
	// Left is the time in seconds before the people decide alone.
	Left float64
	// Axis and Tier locate the discovery at stake for AskDiscovery.
	Axis Family
	Tier int
}

// ErrNoSupplique is returned when answering a supplique that is not pending.
var ErrNoSupplique = errors.New("no such supplique is waiting")

func (c *Civ) ask(s Supplique, tick uint64) {
	if c.Pending != nil {
		// A new supplique replaces the old one: the people settle the old
		// one themselves (GDD 6.5).
		c.resolve(c.leaning(*c.Pending), false, tick)
	}
	c.NextSupplique++
	s.ID = c.NextSupplique
	s.Left = suppliqueTimeout
	c.Pending = &s
	if s.Kind != AskDiscovery {
		c.SinceSupplique = 0
	}
}

// Answer is the god's reply to the pending supplique.
func (c *Civ) Answer(id uint32, option int, tick uint64) error {
	if c.Pending == nil || c.Pending.ID != id {
		return ErrNoSupplique
	}
	if option < 0 || option >= len(c.Pending.Options) {
		return fmt.Errorf("option %d out of range", option)
	}
	c.resolve(option, true, tick)
	return nil
}

// leaning is what the people would choose on their own: what their culture
// and conviction favour.
func (c *Civ) leaning(s Supplique) int {
	switch s.Kind {
	case AskDiscovery:
		d := Discoveries[s.Axis][s.Tier]
		if c.Conviction[d[1].Lean] > c.Conviction[d[0].Lean] {
			return 1
		}
		return 0
	case AskRoom:
		// Only a people that wishes to spread out pushes the edges alone.
		if c.Conviction[Expand] >= 0.2 {
			return 0
		}
		return 1
	default:
		return 0
	}
}

func (c *Civ) resolve(option int, byGod bool, tick uint64) {
	s := *c.Pending
	c.Pending = nil
	if byGod {
		c.chronicle(tick, "Ton peuple a entendu ta voix : « %s ».", s.Options[option])
	} else {
		c.chronicle(tick, "Sans signe de son dieu, ton peuple a choisi : « %s ».", s.Options[option])
	}
	switch s.Kind {
	case AskDiscovery:
		c.Tech.Chosen[s.Axis][s.Tier] = int8(option)
		d := Discoveries[s.Axis][s.Tier][option]
		c.chronicle(tick, "Les savants maîtrisent « %s » (%s).", d.Name, d.Text)
	case AskRoom:
		switch {
		case option == 0 && c.Expanding == 0 && c.Size < maxSize && c.Stock.Covers(expandCost):
			c.Stock = c.Stock.Sub(expandCost)
			c.Expanding = 60
		case option == 0 && c.Size >= maxSize:
			c.chronicle(tick, "L'île a atteint sa plus grande étendue : elle ne peut plus grandir.")
		case option == 0 && c.Expanding > 0:
			c.chronicle(tick, "Ton peuple repousse déjà les bords de l'île.")
		case option == 0:
			// Agreed but not affordable yet: the people promise to do it as
			// soon as they can, and stop asking.
			c.ExpandPledge = true
			c.RoomCooldown = roomCooldown
			c.chronicle(tick, "Ton peuple repoussera les bords de l'île dès que le bois et la pierre suffiront.")
		case option == 1:
			// The god said no: ask again later, and each time later still.
			c.RoomRefusals++
			c.RoomCooldown = roomCooldown * float64(int(1)<<min(c.RoomRefusals-1, 4))
		}
	case AskFamine:
		if option == 0 {
			// Rationing: every soldier goes back to the fields.
			c.Population += c.Defenders + c.Assailants
			c.Defenders, c.Assailants = 0, 0
		} else {
			c.FarmUrge = 10 * 60
		}
	}
}

func (c *Civ) offerDiscovery(axis Family, tier int, tick uint64) {
	d := Discoveries[axis][tier]
	c.chronicle(tick, "Les savants ont percé un secret %s.", axis.Of())
	c.ask(Supplique{
		Kind:     AskDiscovery,
		Question: fmt.Sprintf("Nos savants hésitent entre deux découvertes %s.", axis.Of()),
		Options:  []string{d[0].Name + " : " + d[0].Text, d[1].Name + " : " + d[1].Text},
		Axis:     axis,
		Tier:     tier,
	}, tick)
}

// petition raises ordinary suppliques from the people's troubles, at most
// one every suppliqueGap seconds, preferably at dawn (GDD 3.6).
func (c *Civ) petition(dt float64, dawn bool, tick uint64) {
	if c.Pending != nil {
		c.Pending.Left -= dt
		if c.Pending.Left <= 0 {
			c.resolve(c.leaning(*c.Pending), false, tick)
		}
		return
	}
	c.SinceSupplique += dt
	if c.SinceSupplique < suppliqueGap || (!dawn && c.SinceSupplique < 2*suppliqueGap) {
		return
	}
	switch {
	case c.Starving:
		c.ask(Supplique{
			Kind:     AskFamine,
			Question: "Les greniers sont vides. Que devons-nous faire ?",
			Options:  []string{"Rationner : les soldats retournent aux champs", "Bâtir des fermes avant toute chose"},
		}, tick)
	case c.Crowded && c.Size < maxSize && c.RoomCooldown <= 0:
		c.ask(Supplique{
			Kind:     AskRoom,
			Question: "Nous manquons de place. Faut-il repousser les bords de l'île ?",
			Options:  []string{"Repousser les bords de l'île", "Nous ferons avec"},
		}, tick)
	}
}

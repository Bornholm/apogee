package sim

import "testing"

func TestTheDeedsOfTheFallenChooseTheRelics(t *testing.T) {
	c := NewCiv(1)
	c.Repelled = 30 // a fortress
	got := relicsOf(&c, 20)
	if len(got) != 3 || got[0] != Shield {
		t.Fatalf("a fortress leaves %v, want the shield first", got)
	}
	c.Repelled, c.Conquests = 0, 9
	if got := relicsOf(&c, 20); got[0] != WarBanner {
		t.Fatalf("a conqueror leaves %v, want the banner first", got)
	}
}

func TestARelicIsChosenAtRebirthAndActs(t *testing.T) {
	c := NewCiv(2)
	c.Pending = nil
	c.offerRelics([]Relic{Crown, Shield, Seeds}, 10)
	if c.Pending == nil || c.Pending.Kind != AskRelic || len(c.Pending.Options) != 3 {
		t.Fatalf("no relic asked: %+v", c.Pending)
	}
	wood := c.Stock[Wood]
	if err := c.Answer(c.Pending.ID, 0, 11); err != nil {
		t.Fatal(err)
	}
	if c.Relic != Crown || c.Stock[Wood] != 2*wood {
		t.Fatalf("the crown: relic %v, wood %v from %v", c.Relic, c.Stock[Wood], wood)
	}
	c.Relic = Shield
	if c.Mods().CoreArmor < 0.2+c.Trait.Mods().CoreArmor-1e-9 {
		t.Fatal("the shield does not armour the heart")
	}
}

func TestTheFallenLeaveRelicsToTheirHeirs(t *testing.T) {
	var s Sky
	id := s.Found("lignée", "Lignée", 4, 0)
	is := s.Island(id)
	is.Civ.Repelled = 12
	is.Civ.CoreHealth = 0
	is.Falling = 0.01
	s.Step()
	if len(s.Waiting) != 1 || len(s.Waiting[0].Relics) != 3 {
		t.Fatalf("waiting: %+v", s.Waiting)
	}
	s.rebirths()
	for _, n := range s.Islands {
		if n.Owner == "lignée" {
			if n.Civ.Pending == nil || n.Civ.Pending.Kind != AskRelic {
				t.Fatal("the heirs are not asked for their relic")
			}
			return
		}
	}
	t.Fatal("no heir")
}

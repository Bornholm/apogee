package sim

import (
	"hash/fnv"
	"strings"
	"unicode/utf8"
)

// Lineage is how a god is known in the sky (GDD 11.7): the name of their
// line, the colour of their roofs and cloth, their emblem and the style of
// their buildings. It weighs on nothing; the sky keeps it by owner, so it
// lives on through rebirths.
type Lineage struct {
	Name   string `json:",omitempty"`
	Colour int8   `json:",omitempty"`
	Emblem int8   `json:",omitempty"`
	Style  int8   `json:",omitempty"`
}

// Bounds of a lineage: they match the art's colours, emblems and styles.
const (
	NumColours = 8
	NumEmblems = 12
	NumStyles  = 3
	// MaxName is the longest name of a lineage or a civilisation, in runes.
	MaxName = 28
)

// Clean bounds a lineage received from a client.
func (l Lineage) Clean() Lineage {
	l.Name = CleanName(l.Name)
	if l.Colour < 0 || l.Colour >= NumColours {
		l.Colour = 0
	}
	if l.Emblem < 0 || l.Emblem >= NumEmblems {
		l.Emblem = 0
	}
	if l.Style < 0 || l.Style >= NumStyles {
		l.Style = 0
	}
	return l
}

// CleanName trims a name and cuts it to MaxName runes, without control
// characters.
func CleanName(s string) string {
	s = strings.Map(func(r rune) rune {
		if r < ' ' || r == 0x7f {
			return -1
		}
		return r
	}, strings.TrimSpace(s))
	for utf8.RuneCountInString(s) > MaxName {
		_, size := utf8.DecodeLastRuneInString(s)
		s = s[:len(s)-size]
	}
	return strings.TrimSpace(s)
}

// SetLineage records the lineage of an owner.
func (s *Sky) SetLineage(owner string, l Lineage) {
	if s.Lineages == nil {
		s.Lineages = map[string]Lineage{}
	}
	s.Lineages[owner] = l.Clean()
}

// LineageOf returns the lineage of an owner: the one they chose, or for a
// rival AI and a god who chose none, one drawn from their key.
func (s *Sky) LineageOf(owner string) Lineage {
	if l, ok := s.Lineages[owner]; ok {
		return l
	}
	h := fnv.New64a()
	h.Write([]byte(owner))
	v := h.Sum64()
	l := Lineage{Colour: int8(v % NumColours), Emblem: int8(v / 8 % NumEmblems), Style: int8(v / 96 % NumStyles)}
	if strings.HasPrefix(owner, AIPrefix) {
		l.Name = "Lignée " + strings.TrimPrefix(owner, AIPrefix)
	}
	return l
}

// Peoples and places make the names of civilisations: a people, then a
// place that the trait of the island suggests.
var (
	peoples = []string{"Les Enfants", "Les Gardiens", "Les Veilleurs", "Les Héritiers", "Les Fils", "Les Pèlerins", "Les Bâtisseurs", "Les Rêveurs"}
	places  = [NumTraits][]string{
		NoTrait:     {"des Nuées", "du Ciel", "de l'Aube"},
		CrystalIsle: {"du Cristal", "de l'Aether", "des Éclats"},
		ForestIsle:  {"des Forêts", "des Pins", "des Sous-bois"},
		RockIsle:    {"du Roc", "des Pierres", "de la Falaise"},
		HotSpring:   {"des Sources", "des Brumes", "des Eaux chaudes"},
		HighIsle:    {"des Cimes", "des Hauteurs", "du Vent"},
		FertileIsle: {"des Prés", "des Moissons", "des Vergers"},
	}
)

// CivName draws the name of a civilisation from the trait of its island:
// n picks another one.
func CivName(t Trait, seed uint64, n int) string {
	if t < 0 || t >= NumTraits {
		t = NoTrait
	}
	v := seed + uint64(n)*0x9e3779b97f4a7c15
	h := fnv.New64a()
	var b [8]byte
	for i := range b {
		b[i] = byte(v >> (8 * i))
	}
	h.Write(b[:])
	x := h.Sum64()
	ps := places[t]
	return peoples[x%uint64(len(peoples))] + " " + ps[x/8%uint64(len(ps))]
}

// Rename names a living civilisation of an owner.
func (s *Sky) Rename(owner, name string) bool {
	name = CleanName(name)
	if name == "" {
		return false
	}
	for i := range s.Islands {
		if s.Islands[i].Owner == owner && s.Islands[i].Falling == 0 {
			s.Islands[i].CivName = name
			return true
		}
	}
	return false
}

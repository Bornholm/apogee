package sim

import "fmt"

// Resource is one of the four basic resources (GDD 4.1).
type Resource int

const (
	Wood Resource = iota
	Stone
	Food
	Aether
	NumResources
)

// Name returns the resource's name, as the chronicle writes it.
func (r Resource) Name() string {
	return [...]string{"bois", "pierre", "vivres", "aether"}[r]
}

// Stock is an amount of each resource.
type Stock [NumResources]float64

// Covers reports whether s holds at least c of every resource.
func (s Stock) Covers(c Stock) bool {
	for i := range s {
		if s[i] < c[i] {
			return false
		}
	}
	return true
}

// Sub returns s minus c.
func (s Stock) Sub(c Stock) Stock {
	for i := range s {
		s[i] -= c[i]
	}
	return s
}

// Intention is a wish turned towards the island, one slice of the god's pie
// (GDD 6.3).
type Intention int

const (
	Prosper Intention = iota
	Expand
	Fortify
	Arm
	Study
	NumIntentions
)

// Name returns the intention's verb, as shown to the player.
func (i Intention) Name() string {
	return [...]string{"Prospérer", "S'étendre", "Se fortifier", "S'armer", "Étudier"}[i]
}

// motive is how the chronicle explains a deed done for this intention.
func (i Intention) motive() string {
	return [...]string{"Voulant prospérer", "Voulant s'étendre", "Voulant se fortifier",
		"Voulant s'armer", "Avide de savoir"}[i]
}

// Family groups intentions; each family is also a technology axis (GDD 5.2).
type Family int

const (
	Weapons  Family = iota // Armes
	Walls                  // Murailles
	Industry               // Industrie
	Arcana                 // Arcanes
	NumFamilies
)

// Name returns the family's name, which is also its technology axis.
func (f Family) Name() string {
	return [...]string{"Armes", "Murailles", "Industrie", "Arcanes"}[f]
}

// Of returns the family's name with its article, as in « un secret des
// Armes » or « de l'Industrie ».
func (f Family) Of() string {
	return [...]string{"des Armes", "des Murailles", "de l'Industrie", "des Arcanes"}[f]
}

// Family returns the family the intention belongs to.
func (i Intention) Family() Family {
	return [...]Family{Industry, Industry, Walls, Weapons, Arcana}[i]
}

// weight is how much an intention feeds the culture of its family: two
// intentions feed the Industry, so the second only counts half, and the
// Industry does not outweigh the other families for having two (GDD 6.6).
func (i Intention) weight() float64 {
	if i == Expand {
		return 0.5
	}
	return 1
}

// Pie is the god's split of attention between inner intentions, in percent
// (GDD 6.2).
type Pie [NumIntentions]int

// DefaultPie is the pie of a newborn civilisation: Prosper 60, Fortify 40.
var DefaultPie = Pie{Prosper: 60, Fortify: 40}

// MaxSlices is the number of intentions a pie may hold at once.
const MaxSlices = 3

// Validate checks the pie rules: slices in steps of 10 %, at most three of
// them, adding up to 100 %.
func (p Pie) Validate() error {
	sum, n := 0, 0
	for i, v := range p {
		if v < 0 || v > 100 || v%10 != 0 {
			return fmt.Errorf("%s: %d %% is not a multiple of 10 between 0 and 100", Intention(i).Name(), v)
		}
		if v > 0 {
			n++
		}
		sum += v
	}
	if n > MaxSlices {
		return fmt.Errorf("%d intentions, at most %d", n, MaxSlices)
	}
	if sum != 100 {
		return fmt.Errorf("the slices add up to %d %%, not 100 %%", sum)
	}
	return nil
}

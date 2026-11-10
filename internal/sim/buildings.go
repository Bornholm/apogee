package sim

// Kind is a type of building.
type Kind int

const (
	Core Kind = iota
	House
	Farm
	Woodcutter
	Quarry
	Extractor
	Warehouse
	Tower
	Barracks
	Library
	NumKinds
)

// Spec describes a kind of building. Values are starting points to balance
// (GDD 🔧).
type Spec struct {
	Name string // with its article, as the chronicle writes it
	Cost Stock
	// BuildTime is the construction time in seconds of simulated time.
	BuildTime float64
	// Radius is the footprint on the ground (GDD 3.5).
	Radius  float64
	Housing int
	// Jobs and Output describe a producer: each worker yields Output per
	// second of the resource Produces, or knowledge for a library.
	Jobs     int
	Produces Resource
	Output   float64
	// Near names the natural feature a producer works best next to.
	Near FeatureKind
	// Storage raises the cap of every resource.
	Storage float64
	// Defense is the strength the building adds against attacks.
	Defense float64
	// Affinity weighs how much each intention calls for this building.
	Affinity [NumIntentions]float64
}

// Specs lists every kind of building.
var Specs = [NumKinds]Spec{
	Core: {Name: "le cœur", Radius: 7, Housing: 6, Defense: 4},
	House: {Name: "une maison", Cost: Stock{Wood: 20, Stone: 5}, BuildTime: 30, Radius: 4,
		Housing: 5, Affinity: [NumIntentions]float64{Prosper: 0.6, Expand: 0.3}},
	Farm: {Name: "une ferme", Cost: Stock{Wood: 15}, BuildTime: 25, Radius: 5,
		Jobs: 3, Produces: Food, Output: 0.08, Near: NoFeature,
		Affinity: [NumIntentions]float64{Prosper: 0.8, Arm: 0.3}},
	Woodcutter: {Name: "une cabane de bûcheron", Cost: Stock{Wood: 10, Stone: 5}, BuildTime: 25, Radius: 3.5,
		Jobs: 3, Produces: Wood, Output: 0.06, Near: Grove,
		Affinity: [NumIntentions]float64{Prosper: 0.5, Expand: 0.4, Fortify: 0.2, Arm: 0.2}},
	Quarry: {Name: "une carrière", Cost: Stock{Wood: 15}, BuildTime: 30, Radius: 3.5,
		Jobs: 3, Produces: Stone, Output: 0.05, Near: Rock,
		Affinity: [NumIntentions]float64{Prosper: 0.3, Expand: 0.4, Fortify: 0.6}},
	Extractor: {Name: "un extracteur d'aether", Cost: Stock{Wood: 15, Stone: 15}, BuildTime: 40, Radius: 3,
		Jobs: 2, Produces: Aether, Output: 0.03, Near: Crystal,
		Affinity: [NumIntentions]float64{Study: 0.7, Expand: 0.4}},
	Warehouse: {Name: "un entrepôt", Cost: Stock{Wood: 25, Stone: 15}, BuildTime: 35, Radius: 4.5,
		Storage: 300, Affinity: [NumIntentions]float64{Prosper: 0.4}},
	Tower: {Name: "une tour", Cost: Stock{Wood: 10, Stone: 30}, BuildTime: 45, Radius: 3,
		Defense: 4.5, Affinity: [NumIntentions]float64{Fortify: 1}},
	Barracks: {Name: "une caserne", Cost: Stock{Wood: 30, Stone: 20}, BuildTime: 50, Radius: 5,
		Affinity: [NumIntentions]float64{Arm: 1}},
	Library: {Name: "une bibliothèque", Cost: Stock{Wood: 20, Stone: 25, Aether: 5}, BuildTime: 50, Radius: 4.5,
		Jobs: 2, Output: 0.0015, Defense: 2, // the scholars' wards
		Affinity: [NumIntentions]float64{Study: 1}},
}

// Building stands on an island. Positions are in island units, relative to
// the island centre.
type Building struct {
	ID   uint32
	Kind Kind
	X, Y float64
	// Progress goes from 0 to 1 while the building is under construction.
	Progress float64
}

// Done reports whether the building is complete.
func (b Building) Done() bool { return b.Progress >= 1 }

// FeatureKind is a natural site on an island.
type FeatureKind int

const (
	NoFeature FeatureKind = iota
	Grove                 // pines: wood
	Rock                  // boulders: stone
	Crystal               // aether crystals
	Spring                // a hot spring: steam, nothing to work
)

// Feature is a natural site, generated from the island's seed.
type Feature struct {
	Kind   FeatureKind
	X, Y   float64
	Radius float64
}

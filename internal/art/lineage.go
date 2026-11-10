package art

import "math"

// The identity of a lineage (GDD 11.7): the colour of its roofs and cloth,
// its emblem, the style of its buildings. None of it weighs on the game; it
// lets a god be recognised in the sky.

// Style is the architecture of a lineage.
type Style int

const (
	Gabled   Style = iota // pitched roofs
	Domed                 // domes
	Terraced              // flat roofs behind parapets
	NumStyles
)

// StyleNames name the styles, for the interface.
var StyleNames = [NumStyles]string{"Toits pentus", "Dômes", "Terrasses"}

// LineageColours are the colours a lineage may take, for its roofs and its
// cloth: the first is the roofs' own terracotta.
var LineageColours = [...]Ramp{
	{hex(0xe27c48), hex(0xb95b37), hex(0x8a412b)}, // terracotta
	{hex(0x7f95c0), hex(0x5b6f99), hex(0x3f4f73)}, // slate
	{hex(0x8fb35a), hex(0x6b8f42), hex(0x4c6a30)}, // moss
	{hex(0xc0607a), hex(0x964459), hex(0x6a2f40)}, // wine
	{hex(0xe6b54a), hex(0xb98a33), hex(0x86621f)}, // ochre
	{hex(0xa37ad6), hex(0x7d58ae), hex(0x584083)}, // amethyst
	{hex(0x4fb3a8), hex(0x368a81), hex(0x23625c)}, // teal
	{hex(0x8a8790), hex(0x66636c), hex(0x45434b)}, // charcoal
}

// ColourNames name the colours, for the interface.
var ColourNames = [len(LineageColours)]string{"Terre cuite", "Ardoise", "Mousse", "Lie-de-vin", "Ocre", "Améthyste", "Sarcelle", "Charbon"}

// Livery is the material of a lineage's colour.
func Livery(colour int) Material {
	if colour < 0 || colour >= len(LineageColours) {
		colour = 0
	}
	return lineage0 + Material(colour)
}

// Dress paints a mesh in a lineage's colour: its roofs and its cloth.
func Dress(m Mesh, colour int) Mesh {
	to := Livery(colour)
	out := Mesh{Faces: make([]Face, len(m.Faces))}
	for i, f := range m.Faces {
		if f.Mat == Roof || f.Mat == Team {
			f.Mat = to
		}
		out.Faces[i] = f
	}
	return out
}

// roofOf covers a box of half sizes w × d and wall height h in a style,
// with its ridge along X when pitched.
func roofOf(m *Mesh, style Style, roof, wall Material, w, d, h, rise, overhang float64) {
	switch style {
	case Domed:
		// A low drum, then a dome of three rings closing on a finial.
		var dome Mesh
		rad := math.Min(w, d) * 1.05
		n := 8
		base := ringAt(n, rad, h, math.Pi/8)
		mid := ringAt(n, rad*0.8, h+rise*0.55, math.Pi/8)
		top := ringAt(n, rad*0.42, h+rise*0.95, math.Pi/8)
		loft(&dome, roof, base, mid)
		loft(&dome, roof, mid, top)
		fan(&dome, roof, top, V(0, 0, h+rise*1.15))
		dome.OrientOutward(V(0, 0, h))
		m.Merge(dome)
		// The roof between the walls and the drum.
		m.Add(wall, V(-w, -d, h), V(w, -d, h), V(w, d, h), V(-w, d, h))
	case Terraced:
		// A flat roof behind a parapet of the lineage's colour, and a
		// small stair hut.
		box(m, wall, -w-0.2, -d-0.2, h, w+0.2, d+0.2, h+0.3)
		t := 0.35
		box(m, roof, -w-0.2, -d-0.2, h+0.3, w+0.2, -d-0.2+t, h+1.1)
		box(m, roof, -w-0.2, d+0.2-t, h+0.3, w+0.2, d+0.2, h+1.1)
		box(m, roof, -w-0.2, -d-0.2+t, h+0.3, -w-0.2+t, d+0.2-t, h+1.1)
		box(m, roof, w+0.2-t, -d-0.2+t, h+0.3, w+0.2, d+0.2-t, h+1.1)
		box(m, wall, w*0.2, -d*0.5, h+0.3, w*0.7, d*0.3, h+2)
	default:
		gable(m, roof, wall, w, d, h, rise, overhang)
	}
}

// ringAt is a regular ring, without jitter.
func ringAt(n int, radius, z, phase float64) []Vec3 {
	pts := make([]Vec3, n)
	for i := range pts {
		a := phase + 2*math.Pi*float64(i)/float64(n)
		pts[i] = V(radius*math.Cos(a), radius*math.Sin(a), z)
	}
	return pts
}

// Emblems are the sigils a lineage may raise, seven pixels square: an ear
// of wheat, a tower, an eye, a flame, a wing, a star, a moon, a sun, a
// crown, a tree, an anchor, a key.
var Emblems = [...][7]string{
	{"...#...", "..#.#..", "...#...", "..#.#..", "...#...", "...#...", "...#..."},
	{"#.#.#.#", "#######", ".#####.", ".##.##.", ".#####.", ".#####.", "#######"},
	{".......", "..###..", ".#...#.", "#..#..#", ".#...#.", "..###..", "......."},
	{"...#...", "..##...", "..###..", ".####..", ".#####.", "##.###.", ".#####."},
	{"#......", "##.....", "###....", "####...", "#####..", ".#####.", "..####."},
	{"...#...", "...#...", "#######", ".#####.", "..###..", ".##.##.", "#.....#"},
	{"..###..", ".##....", "##.....", "##.....", "##.....", ".##....", "..###.."},
	{"#..#..#", ".#.#.#.", "..###..", "###.###", "..###..", ".#.#.#.", "#..#..#"},
	{".......", "#..#..#", "##.#.##", "#######", "#######", "#######", "......."},
	{"..###..", ".#####.", "#######", ".#####.", "...#...", "...#...", "..###.."},
	{"...#...", "..###..", "...#...", "...#...", "#..#..#", ".#.#.#.", "..###.."},
	{".###...", "#...#..", "#...#..", ".###...", "..#....", "..##...", "..#.#.."},
}

// EmblemNames name the emblems, for the interface.
var EmblemNames = [len(Emblems)]string{"Épi", "Tour", "Œil", "Flamme", "Aile", "Étoile", "Lune", "Soleil", "Couronne", "Arbre", "Ancre", "Clé"}

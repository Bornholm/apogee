// Package art generates Apogée's faceted pixel-art sprites procedurally.
//
// Every visual is a small low-poly mesh: a list of planar faces, each made of
// one material. The mesh is projected with an oblique 3/4 orthographic camera,
// each face receives one of the three tones of its material ramp depending on
// its orientation towards the single fixed light, and the result is rasterized
// without anti-aliasing. See GDD sections 12.1 and 12.2.
package art

import "math"

// Vec3 is a point or direction in world space: X points east (screen right),
// Y points south (towards the viewer) and Z points up.
type Vec3 struct{ X, Y, Z float64 }

// V is a shorthand constructor for Vec3.
func V(x, y, z float64) Vec3 { return Vec3{x, y, z} }

func (a Vec3) Add(b Vec3) Vec3      { return Vec3{a.X + b.X, a.Y + b.Y, a.Z + b.Z} }
func (a Vec3) Sub(b Vec3) Vec3      { return Vec3{a.X - b.X, a.Y - b.Y, a.Z - b.Z} }
func (a Vec3) Scale(k float64) Vec3 { return Vec3{a.X * k, a.Y * k, a.Z * k} }
func (a Vec3) Dot(b Vec3) float64   { return a.X*b.X + a.Y*b.Y + a.Z*b.Z }
func (a Vec3) Len() float64         { return math.Sqrt(a.Dot(a)) }
func (a Vec3) Cross(b Vec3) Vec3 {
	return Vec3{a.Y*b.Z - a.Z*b.Y, a.Z*b.X - a.X*b.Z, a.X*b.Y - a.Y*b.X}
}

// Norm returns a unit vector with the same direction, or the zero vector.
func (a Vec3) Norm() Vec3 {
	l := a.Len()
	if l == 0 {
		return Vec3{}
	}
	return a.Scale(1 / l)
}

// RotZ rotates the vector around the vertical axis by angle radians.
func (a Vec3) RotZ(angle float64) Vec3 {
	s, c := math.Sincos(angle)
	return Vec3{a.X*c - a.Y*s, a.X*s + a.Y*c, a.Z}
}

// Face is a planar polygon made of a single material.
type Face struct {
	V   []Vec3
	Mat Material
}

// Normal returns the unit normal of the face, computed with Newell's method so
// that slightly non-planar polygons still get a sensible orientation. The
// normal follows the counter-clockwise order of the vertices.
func (f Face) Normal() Vec3 {
	var n Vec3
	for i, a := range f.V {
		b := f.V[(i+1)%len(f.V)]
		n.X += (a.Y - b.Y) * (a.Z + b.Z)
		n.Y += (a.Z - b.Z) * (a.X + b.X)
		n.Z += (a.X - b.X) * (a.Y + b.Y)
	}
	return n.Norm()
}

// Centroid returns the average of the face vertices.
func (f Face) Centroid() Vec3 {
	var c Vec3
	for _, v := range f.V {
		c = c.Add(v)
	}
	return c.Scale(1 / float64(len(f.V)))
}

// Mesh is a collection of faces forming one object.
type Mesh struct {
	Faces []Face
}

// Add appends a face to the mesh.
func (m *Mesh) Add(mat Material, v ...Vec3) {
	m.Faces = append(m.Faces, Face{V: v, Mat: mat})
}

// Merge appends all faces of other to the mesh.
func (m *Mesh) Merge(other Mesh) {
	m.Faces = append(m.Faces, other.Faces...)
}

// Translate moves every vertex of the mesh by d.
func (m *Mesh) Translate(d Vec3) {
	for i := range m.Faces {
		for j := range m.Faces[i].V {
			m.Faces[i].V[j] = m.Faces[i].V[j].Add(d)
		}
	}
}

// RotZ rotates every vertex of the mesh around the vertical axis through the
// origin. Lighting stays correct because tones are computed after rotation.
func (m *Mesh) RotZ(angle float64) {
	for i := range m.Faces {
		for j := range m.Faces[i].V {
			m.Faces[i].V[j] = m.Faces[i].V[j].RotZ(angle)
		}
	}
}

// OrientOutward flips the faces whose normal points towards center, so that
// every face of a star-shaped solid faces outwards. Builders can then list
// vertices in any order.
func (m *Mesh) OrientOutward(center Vec3) {
	for i := range m.Faces {
		f := &m.Faces[i]
		if f.Normal().Dot(f.Centroid().Sub(center)) < 0 {
			for a, b := 0, len(f.V)-1; a < b; a, b = a+1, b-1 {
				f.V[a], f.V[b] = f.V[b], f.V[a]
			}
		}
	}
}

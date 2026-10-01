package dxf

import (
	"reflect"
	"testing"
)

func meshCodePairs() []CodePair {
	return []CodePair{
		NewStringCodePair(100, "AcDbEntity"),
		NewStringCodePair(8, "MESHES"),
		NewIntCodePair(92, 0), // proxy graphics, not the vertex count
		NewStringCodePair(100, "AcDbSubDMesh"),
		NewShortCodePair(71, 2),
		NewShortCodePair(72, 1),
		NewIntCodePair(91, 3),
		NewIntCodePair(92, 4),
		NewDoubleCodePair(10, 0), NewDoubleCodePair(20, 0), NewDoubleCodePair(30, 0),
		NewDoubleCodePair(10, 1), NewDoubleCodePair(20, 0), NewDoubleCodePair(30, 0),
		NewDoubleCodePair(10, 1), NewDoubleCodePair(20, 1), NewDoubleCodePair(30, 0),
		NewDoubleCodePair(10, 0), NewDoubleCodePair(20, 1), NewDoubleCodePair(30, 1),
		// a quad and a triangle: 1 + 4 + 1 + 3 values
		NewIntCodePair(93, 9),
		NewIntCodePair(90, 4), NewIntCodePair(90, 0), NewIntCodePair(90, 1), NewIntCodePair(90, 2), NewIntCodePair(90, 3),
		NewIntCodePair(90, 3), NewIntCodePair(90, 0), NewIntCodePair(90, 2), NewIntCodePair(90, 3),
		NewIntCodePair(94, 2),
		NewIntCodePair(90, 0), NewIntCodePair(90, 1),
		NewIntCodePair(90, 1), NewIntCodePair(90, 2),
		NewIntCodePair(95, 2),
		NewDoubleCodePair(140, 0.5),
		NewDoubleCodePair(140, -1),
		NewIntCodePair(90, 1),
		NewIntCodePair(91, 7),
		NewIntCodePair(92, 1),
		NewIntCodePair(90, 0),
		NewShortCodePair(63, 3),
	}
}

func TestReadMesh(t *testing.T) {
	mesh := parseEntity(t, "MESH", meshCodePairs()...).(*Mesh)
	assertEqString(t, "MESHES", mesh.Layer())
	assertEqInt(t, 2, int(mesh.Version))
	assertEqBool(t, true, mesh.BlendCrease)
	assertEqInt(t, 3, mesh.SubdivisionLevels)
	assertEqInt(t, 4, len(mesh.Vertices))
	assertEqPoint(t, Point{0, 1, 1}, mesh.Vertices[3])
	assert(t, reflect.DeepEqual([][]int{{0, 1, 2, 3}, {0, 2, 3}}, mesh.Faces), "unexpected faces")
	assert(t, reflect.DeepEqual([][2]int{{0, 1}, {1, 2}}, mesh.Edges), "unexpected edges")
	assert(t, reflect.DeepEqual([]float64{0.5, -1}, mesh.Creases), "unexpected creases")
	assertEqInt(t, 5, len(mesh.overrideData))
}

func TestWriteMesh(t *testing.T) {
	mesh := NewMesh()
	mesh.Vertices = []Point{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}}
	mesh.Faces = [][]int{{0, 1, 2}}
	mesh.Edges = [][2]int{{0, 1}, {1, 2}, {2, 0}}
	mesh.Creases = []float64{0.25}
	written := allCodePairs(mesh, R2013)
	assertContainsCodePairs(t, []CodePair{
		NewStringCodePair(100, "AcDbSubDMesh"),
		NewShortCodePair(71, 2),
		NewShortCodePair(72, 0),
		NewIntCodePair(91, 0),
		NewIntCodePair(92, 3),
	}, written)
	assertContainsCodePairs(t, []CodePair{
		NewIntCodePair(93, 4),
		NewIntCodePair(90, 3), NewIntCodePair(90, 0), NewIntCodePair(90, 1), NewIntCodePair(90, 2),
		NewIntCodePair(94, 3),
	}, written)
	// one crease per edge
	assertContainsCodePairs(t, []CodePair{
		NewIntCodePair(95, 3),
		NewDoubleCodePair(140, 0.25), NewDoubleCodePair(140, 0), NewDoubleCodePair(140, 0),
		NewIntCodePair(90, 0),
	}, written)

	// MESH exists since AutoCAD 2010
	assertNotContainsCodePairs(t, []CodePair{NewStringCodePair(0, "MESH")}, drawingCodePairsFromEntity(t, mesh, R2007))
}

func TestRoundTripMesh(t *testing.T) {
	original := parseEntity(t, "MESH", meshCodePairs()...).(*Mesh)
	drawing := *NewDrawing()
	drawing.Header.Version = R2018
	drawing.Entities = append(drawing.Entities, original)
	actual := roundTripDrawing(t, &drawing).Entities[0].(*Mesh)
	assert(t, reflect.DeepEqual(original.Vertices, actual.Vertices), "vertices differ")
	assert(t, reflect.DeepEqual(original.Faces, actual.Faces), "faces differ")
	assert(t, reflect.DeepEqual(original.Edges, actual.Edges), "edges differ")
	assert(t, reflect.DeepEqual(original.Creases, actual.Creases), "creases differ")
	assert(t, reflect.DeepEqual(original.overrideData, actual.overrideData), "overrides differ")
}

func TestTransformMesh(t *testing.T) {
	mesh := parseEntity(t, "MESH", meshCodePairs()...).(*Mesh)
	m := similarityTransforms["mirrored tilted"]
	transformed := transformSingle(t, mesh, m).(*Mesh)
	assertNearPointWithin(t, "mesh vertex", m.TransformPoint(mesh.Vertices[3]), transformed.Vertices[3])
	assert(t, reflect.DeepEqual(mesh.Faces, transformed.Faces), "faces changed")
	assertEqPoint(t, Point{0, 1, 1}, mesh.Vertices[3])
}

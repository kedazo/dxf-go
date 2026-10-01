package dxf

import (
	"testing"
)

func polyfaceLocation(x, y, z float64) []CodePair {
	return []CodePair{
		NewStringCodePair(0, "VERTEX"),
		NewDoubleCodePair(10, x),
		NewDoubleCodePair(20, y),
		NewDoubleCodePair(30, z),
		NewShortCodePair(70, 192),
	}
}

func polyfaceFace(indices ...int16) []CodePair {
	pairs := []CodePair{
		NewStringCodePair(0, "VERTEX"),
		NewDoubleCodePair(10, 0.0),
		NewDoubleCodePair(20, 0.0),
		NewDoubleCodePair(30, 0.0),
		NewShortCodePair(70, 128),
	}
	for i, index := range indices {
		pairs = append(pairs, NewShortCodePair(71+i, index))
	}
	return pairs
}

func TestPolyfaceFaces(t *testing.T) {
	pairs := []CodePair{
		NewStringCodePair(0, "POLYLINE"),
		NewShortCodePair(66, 1),
		NewShortCodePair(70, 64),
		NewShortCodePair(71, 4),
		NewShortCodePair(72, 2),
	}
	pairs = append(pairs, polyfaceLocation(0, 0, 0)...)
	pairs = append(pairs, polyfaceLocation(1, 0, 0)...)
	pairs = append(pairs, polyfaceLocation(1, 1, 0)...)
	pairs = append(pairs, polyfaceLocation(0, 1, 1)...)
	pairs = append(pairs, polyfaceFace(1, 2, -3)...)
	pairs = append(pairs, polyfaceFace(1, 3, 4, 9)...) // 9 is out of range
	pairs = append(pairs, NewStringCodePair(0, "SEQEND"))
	polyline := parseEntities(t, pairs...)[0].(*Polyline)

	vertices := polyline.PolyfaceVertices()
	assertEqInt(t, 4, len(vertices))
	assertEqPoint(t, Point{0.0, 1.0, 1.0}, vertices[3])

	faces := polyline.PolyfaceFaces()
	assertEqInt(t, 2, len(faces))
	assertEqInt(t, 3, len(faces[0].Points))
	assertEqInt(t, 2, faces[0].Indices[2])
	assertEqPoint(t, Point{1.0, 1.0, 0.0}, faces[0].Points[2])
	assertEqBool(t, true, faces[0].EdgeVisible[1])
	assertEqBool(t, false, faces[0].EdgeVisible[2])
	assertEqInt(t, 3, len(faces[1].Points))
	assertEqPoint(t, Point{0.0, 1.0, 1.0}, faces[1].Points[2])
}

func TestPolyfaceFacesOfOrdinaryPolyline(t *testing.T) {
	polyline := NewPolyline()
	polyline.Vertices = append(polyline.Vertices, *NewVertex())
	assertEqInt(t, 0, len(polyline.PolyfaceVertices()))
	assertEqInt(t, 0, len(polyline.PolyfaceFaces()))
}

func TestPolygonMeshGrid(t *testing.T) {
	polyline := NewPolyline()
	polyline.SetIs3DPolygonMesh(true)
	polyline.PolygonMeshMVertexCount = 2
	polyline.PolygonMeshNVertexCount = 3
	for i := 0; i < 6; i++ {
		v := NewVertex()
		v.Location = Point{float64(i % 3), float64(i / 3), 0.0}
		polyline.Vertices = append(polyline.Vertices, *v)
	}
	grid := polyline.PolygonMeshGrid()
	assertEqInt(t, 2, len(grid))
	assertEqInt(t, 3, len(grid[1]))
	assertEqPoint(t, Point{2.0, 1.0, 0.0}, grid[1][2])

	polyline.PolygonMeshNVertexCount = 4
	assert(t, polyline.PolygonMeshGrid() == nil, "expected nil grid for mismatched vertex count")
}

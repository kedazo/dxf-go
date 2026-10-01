package dxf

// PolyfaceFace is one face of a polyface mesh.
type PolyfaceFace struct {
	// Indices are 0-based indices into PolyfaceVertices().
	Indices []int
	// Points are the face corners, resolved from Indices.
	Points []Point
	// EdgeVisible tells whether the edge starting at the corner with the same index is visible.
	EdgeVisible []bool
}

// VertexWidths returns the start and end width of vertex i: its own 40/41 where it has them, else the POLYLINE's
// DefaultStartingWidth/DefaultEndingWidth. An explicit 0 on the vertex stays 0.
func (p *Polyline) VertexWidths(i int) (start, end float64) {
	start, end = p.DefaultStartingWidth, p.DefaultEndingWidth
	if i < 0 || i >= len(p.Vertices) {
		return
	}
	v := &p.Vertices[i]
	if v.HasStartingWidth || v.StartingWidth != 0 {
		start = v.StartingWidth
	}
	if v.HasEndingWidth || v.EndingWidth != 0 {
		end = v.EndingWidth
	}
	return
}

// PolyfaceVertices returns the vertex locations of a polyface mesh, i.e. the vertices that faces index into.
func (p *Polyline) PolyfaceVertices() (points []Point) {
	if !p.IsPolyfaceMesh() {
		return nil
	}
	for i := range p.Vertices {
		v := &p.Vertices[i]
		if isPolyfaceLocationVertex(v) {
			points = append(points, v.Location)
		}
	}
	return
}

// PolyfaceFaces returns the faces of a polyface mesh. Face records are VERTEX entities without a location of their
// own; their 1-based, sign-encoded indices are resolved against PolyfaceVertices(). Indices that are out of range are
// skipped.
func (p *Polyline) PolyfaceFaces() (faces []PolyfaceFace) {
	vertices := p.PolyfaceVertices()
	if vertices == nil {
		return nil
	}
	for i := range p.Vertices {
		v := &p.Vertices[i]
		if !v.IsPolyfaceMeshVertex() || isPolyfaceLocationVertex(v) {
			continue
		}
		var face PolyfaceFace
		for _, index := range []int{v.PolyfaceMeshVertexIndex1, v.PolyfaceMeshVertexIndex2, v.PolyfaceMeshVertexIndex3, v.PolyfaceMeshVertexIndex4} {
			position := index
			if position < 0 {
				position = -position
			}
			position--
			if position < 0 || position >= len(vertices) {
				continue
			}
			face.Indices = append(face.Indices, position)
			face.Points = append(face.Points, vertices[position])
			face.EdgeVisible = append(face.EdgeVisible, index > 0)
		}
		if len(face.Indices) > 0 {
			faces = append(faces, face)
		}
	}
	return
}

// PolygonMeshGrid returns the vertices of a 3D polygon mesh as M rows of N points. If the vertex count does not match
// the declared M x N size, nil is returned.
func (p *Polyline) PolygonMeshGrid() [][]Point {
	m, n := p.PolygonMeshMVertexCount, p.PolygonMeshNVertexCount
	if !p.Is3DPolygonMesh() || m <= 0 || n <= 0 || len(p.Vertices) != m*n {
		return nil
	}
	grid := make([][]Point, m)
	for row := range grid {
		grid[row] = make([]Point, n)
		for column := range grid[row] {
			grid[row][column] = p.Vertices[row*n+column].Location
		}
	}
	return grid
}

// isPolyfaceLocationVertex reports whether a polyface mesh vertex carries a location (flags 128 + 64) rather than
// being a face record (flag 128 only).
func isPolyfaceLocationVertex(v *Vertex) bool {
	return v.IsPolyfaceMeshVertex() && v.Is3DPolygonMesh()
}

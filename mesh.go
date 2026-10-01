package dxf

// Mesh is a subdivision surface MESH (AutoCAD 2010+). Vertices are in world coordinates; faces and edges index into
// Vertices.
type Mesh struct {
	entityCommon

	Version           int16 // code 71
	BlendCrease       bool  // code 72
	SubdivisionLevels int   // code 91, 0 = no smoothing
	Vertices          []Point
	Faces             [][]int
	Edges             [][2]int
	// Creases has one value per edge.
	Creases []float64

	// sub-entity property overrides, kept as read (starting with their count, code 90)
	overrideData []CodePair

	isReadingMeshData bool
	meshData          []CodePair
}

// NewMesh creates an empty MESH.
func NewMesh() *Mesh {
	return &Mesh{
		entityCommon: newEntityCommon(),
		Version:      2,
	}
}

func (e *Mesh) typeString() string      { return "MESH" }
func (e *Mesh) minVersion() AcadVersion { return R2010 }
func (e *Mesh) maxVersion() AcadVersion { return R2018 }

func (e *Mesh) tryApplyCodePair(codePair CodePair) {
	if e.isReadingMeshData {
		e.meshData = append(e.meshData, codePair)
		return
	}
	if codePair.Code == 100 {
		e.isReadingMeshData = stringValue(codePair) == "AcDbSubDMesh"
		return
	}
	// codes like 92 mean something else in the mesh data
	tryApplyCodePairForEntity(e, codePair)
}

// parseMeshData interprets the collected mesh data once the entity is complete.
func (e *Mesh) parseMeshData() {
	r := &hatchDataReader{pairs: e.meshData}
	e.meshData = nil
	e.isReadingMeshData = false

	for !r.done() {
		pair := r.next()
		switch pair.Code {
		case 71:
			e.Version = shortValue(pair)
		case 72:
			e.BlendCrease = shortValue(pair) != 0
		case 91:
			e.SubdivisionLevels = intValue(pair)
		case 92:
			count := intValue(pair)
			for i := 0; i < count; i++ {
				x, ok := r.takeDouble(10)
				if !ok {
					break
				}
				y, _ := r.takeDouble(20)
				z, _ := r.takeDouble(30)
				e.Vertices = append(e.Vertices, Point{x, y, z})
			}
		case 93:
			// the size of the face list counts all values: every face is its vertex count and the indices
			remaining := intValue(pair)
			for remaining > 0 {
				size, ok := r.takeInt(90)
				if !ok {
					break
				}
				remaining--
				face := make([]int, 0, size)
				for i := 0; i < size && remaining > 0; i++ {
					index, ok := r.takeInt(90)
					if !ok {
						break
					}
					face = append(face, index)
					remaining--
				}
				e.Faces = append(e.Faces, face)
			}
		case 94:
			count := intValue(pair)
			for i := 0; i < count; i++ {
				start, ok := r.takeInt(90)
				if !ok {
					break
				}
				end, _ := r.takeInt(90)
				e.Edges = append(e.Edges, [2]int{start, end})
			}
		case 95:
			count := intValue(pair)
			for i := 0; i < count; i++ {
				crease, ok := r.takeDouble(140)
				if !ok {
					break
				}
				e.Creases = append(e.Creases, crease)
			}
		case 90:
			// the sub-entity overrides close the mesh data
			e.overrideData = append([]CodePair{pair}, r.pairs[r.position:]...)
			r.position = len(r.pairs)
		}
	}
}

func (e *Mesh) codePairs(version AcadVersion) (pairs []CodePair) {
	pairs = append(pairs, NewStringCodePair(0, "MESH"))
	pairs = append(pairs, codePairsForEntity(e, version)...)
	pairs = append(pairs, NewStringCodePair(100, "AcDbSubDMesh"))
	pairs = append(pairs, NewShortCodePair(71, e.Version))
	pairs = append(pairs, NewShortCodePair(72, shortFromBool(e.BlendCrease)))
	pairs = append(pairs, NewIntCodePair(91, e.SubdivisionLevels))

	pairs = append(pairs, NewIntCodePair(92, len(e.Vertices)))
	for _, vertex := range e.Vertices {
		pairs = append(pairs, NewDoubleCodePair(10, vertex.X), NewDoubleCodePair(20, vertex.Y), NewDoubleCodePair(30, vertex.Z))
	}

	faceListSize := 0
	for _, face := range e.Faces {
		faceListSize += 1 + len(face)
	}
	pairs = append(pairs, NewIntCodePair(93, faceListSize))
	for _, face := range e.Faces {
		pairs = append(pairs, NewIntCodePair(90, len(face)))
		for _, index := range face {
			pairs = append(pairs, NewIntCodePair(90, index))
		}
	}

	pairs = append(pairs, NewIntCodePair(94, len(e.Edges)))
	for _, edge := range e.Edges {
		pairs = append(pairs, NewIntCodePair(90, edge[0]), NewIntCodePair(90, edge[1]))
	}

	// AutoCAD requires one crease per edge
	pairs = append(pairs, NewIntCodePair(95, len(e.Edges)))
	for i := range e.Edges {
		crease := 0.0
		if i < len(e.Creases) {
			crease = e.Creases[i]
		}
		pairs = append(pairs, NewDoubleCodePair(140, crease))
	}

	if len(e.overrideData) > 0 {
		pairs = append(pairs, e.overrideData...)
	} else {
		pairs = append(pairs, NewIntCodePair(90, 0))
	}
	return
}

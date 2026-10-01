package dxf

// MLineElementParameters is the parametrization of one MLINE element along one segment (codes 74/41 and 75/42).
// Distances are along the segment direction unless noted.
type MLineElementParameters struct {
	// Line is the miter offset (from the vertex along the miter direction to the element), then the distance from
	// there to where the element starts, then alternating dash and gap lengths. An element broken with MLEDIT has
	// more values than its neighbours.
	Line []float64
	// AreaFill is the same for the fill area; usually empty.
	AreaFill []float64
}

// buildElementParameters splits the 41/42 values read into ElementParameters by the 74/75 counts. The elements of
// a vertex are StyleElementCount consecutive 74/75 groups.
func (e *MLine) buildElementParameters() {
	elementCount := len(e.parameterCounts)
	vertexCount := len(e.Vertices)
	if elementCount == 0 || vertexCount == 0 {
		return
	}
	perVertex := e.StyleElementCount
	if perVertex <= 0 || perVertex*vertexCount != elementCount {
		if elementCount%vertexCount != 0 {
			return
		}
		perVertex = elementCount / vertexCount
	}

	line, fill := e.Parameters, e.AreaFillParameters
	take := func(values *[]float64, count int) []float64 {
		count = max(0, min(count, len(*values)))
		taken := append([]float64{}, (*values)[:count]...)
		*values = (*values)[count:]
		return taken
	}
	e.ElementParameters = make([][]MLineElementParameters, 0, vertexCount)
	for v := 0; v < vertexCount; v++ {
		elements := make([]MLineElementParameters, perVertex)
		for j := range elements {
			k := v*perVertex + j
			elements[j].Line = take(&line, e.parameterCounts[k])
			if k < len(e.areaFillParameterCounts) {
				elements[j].AreaFill = take(&fill, e.areaFillParameterCounts[k])
			} else {
				elements[j].AreaFill = []float64{}
			}
		}
		e.ElementParameters = append(e.ElementParameters, elements)
	}
}

func (e *MLine) codePairs(version AcadVersion) (pairs []CodePair) {
	pairs = append(pairs, NewStringCodePair(0, "MLINE"))
	pairs = append(pairs, codePairsForEntity(e, version)...)
	pairs = append(pairs, NewStringCodePair(100, "AcDbMline"))
	pairs = append(pairs, NewStringCodePair(2, e.StyleName))
	if e.styleHandle != "" {
		pairs = append(pairs, NewStringCodePair(340, e.styleHandle))
	}
	pairs = append(pairs, NewDoubleCodePair(40, e.ScaleFactor))
	pairs = append(pairs, NewShortCodePair(70, int16(e.Justification)))
	pairs = append(pairs, NewShortCodePair(71, int16(e.Flags)))
	pairs = append(pairs, NewShortCodePair(72, int16(len(e.Vertices))))
	pairs = append(pairs, NewShortCodePair(73, int16(e.StyleElementCount)))
	pairs = append(pairs, NewDoubleCodePair(10, e.StartPoint.X))
	pairs = append(pairs, NewDoubleCodePair(20, e.StartPoint.Y))
	pairs = append(pairs, NewDoubleCodePair(30, e.StartPoint.Z))
	if e.Normal != *NewZAxis() {
		pairs = append(pairs, NewDoubleCodePair(210, e.Normal.X))
		pairs = append(pairs, NewDoubleCodePair(220, e.Normal.Y))
		pairs = append(pairs, NewDoubleCodePair(230, e.Normal.Z))
	}
	// every vertex: location, segment and miter direction, then each element's parametrization
	for i, vertex := range e.Vertices {
		direction, miter := Point{1, 0, 0}, Point{0, 1, 0}
		if i < len(e.SegmentDirections) {
			direction = e.SegmentDirections[i]
		}
		if i < len(e.MiterDirections) {
			miter = e.MiterDirections[i]
		}
		pairs = append(pairs, NewDoubleCodePair(11, vertex.X))
		pairs = append(pairs, NewDoubleCodePair(21, vertex.Y))
		pairs = append(pairs, NewDoubleCodePair(31, vertex.Z))
		pairs = append(pairs, NewDoubleCodePair(12, direction.X))
		pairs = append(pairs, NewDoubleCodePair(22, direction.Y))
		pairs = append(pairs, NewDoubleCodePair(32, direction.Z))
		pairs = append(pairs, NewDoubleCodePair(13, miter.X))
		pairs = append(pairs, NewDoubleCodePair(23, miter.Y))
		pairs = append(pairs, NewDoubleCodePair(33, miter.Z))

		var elements []MLineElementParameters
		if i < len(e.ElementParameters) {
			elements = e.ElementParameters[i]
		} else {
			elements = make([]MLineElementParameters, e.StyleElementCount)
		}
		for _, element := range elements {
			pairs = append(pairs, NewShortCodePair(74, int16(len(element.Line))))
			for _, value := range element.Line {
				pairs = append(pairs, NewDoubleCodePair(41, value))
			}
			pairs = append(pairs, NewShortCodePair(75, int16(len(element.AreaFill))))
			for _, value := range element.AreaFill {
				pairs = append(pairs, NewDoubleCodePair(42, value))
			}
		}
	}
	return
}

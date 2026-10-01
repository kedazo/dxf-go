package dxf

import (
	"reflect"
	"testing"
)

func parseHatch(t *testing.T, hatchData ...CodePair) *Hatch {
	pairs := []CodePair{
		NewStringCodePair(5, "A1"),
		NewStringCodePair(330, "1F"), // owner
		NewStringCodePair(100, "AcDbEntity"),
		NewStringCodePair(8, "FILL"),
		NewStringCodePair(100, "AcDbHatch"),
	}
	pairs = append(pairs, hatchData...)
	return parseEntity(t, "HATCH", pairs...).(*Hatch)
}

func hatchPoint(xCode int, x, y float64) []CodePair {
	return []CodePair{NewDoubleCodePair(xCode, x), NewDoubleCodePair(xCode+10, y)}
}

func pairs(groups ...[]CodePair) (result []CodePair) {
	for _, group := range groups {
		result = append(result, group...)
	}
	return
}

func TestReadHatchPolylinePathWithBulgesAndSourceHandles(t *testing.T) {
	hatch := parseHatch(t, pairs(
		hatchPoint(10, 0.0, 0.0),
		[]CodePair{
			NewDoubleCodePair(30, 5.0),
			NewDoubleCodePair(210, 0.0),
			NewDoubleCodePair(220, 0.0),
			NewDoubleCodePair(230, -1.0),
			NewStringCodePair(2, "ANSI31"),
			NewShortCodePair(70, 0),
			NewShortCodePair(71, 1),
			NewIntCodePair(91, 1),
			NewIntCodePair(92, 3),
			NewShortCodePair(72, 1),
			NewShortCodePair(73, 1),
			NewIntCodePair(93, 3),
		},
		hatchPoint(10, 0.0, 0.0), []CodePair{NewDoubleCodePair(42, 0.0)},
		hatchPoint(10, 10.0, 0.0), []CodePair{NewDoubleCodePair(42, 1.0)},
		hatchPoint(10, 10.0, 10.0), []CodePair{NewDoubleCodePair(42, 0.0)},
		[]CodePair{
			NewIntCodePair(97, 1),
			NewStringCodePair(330, "2B"),
			NewShortCodePair(75, 1),
			NewShortCodePair(76, 1),
		},
	)...)

	assertEqString(t, "FILL", hatch.Layer())
	assertEqUInt64(t, 0x1F, uint64(hatch.getOwnerPointer().handle))
	assertEqFloat64(t, 5.0, hatch.Elevation())
	assertEqVector(t, Vector{0.0, 0.0, -1.0}, hatch.ExtrusionDirection)
	assertEqString(t, "ANSI31", hatch.PatternName)
	assertEqBool(t, false, hatch.SolidFill)
	assertEqBool(t, true, hatch.IsAssociative)
	assertEqInt(t, int(HatchStyleOutermost), int(hatch.Style))
	assertEqInt(t, int(HatchPatternTypePredefined), int(hatch.PatternType))

	assertEqInt(t, 1, len(hatch.Paths))
	path := hatch.Paths[0]
	assertEqBool(t, true, path.IsPolyline())
	assertEqBool(t, true, path.IsClosed)
	assertEqInt(t, 3, len(path.Vertices))
	assertEqFloat64(t, 10.0, path.Vertices[2][1])
	assertEqInt(t, 3, len(path.Bulges))
	assertEqFloat64(t, 1.0, path.Bulges[1])
	assertEqInt(t, 0, len(path.Edges))
	assertEqInt(t, 1, len(path.SourceHandles))
	assertEqUInt64(t, 0x2B, uint64(path.SourceHandles[0]))
}

func TestReadHatchPolylinePathWithoutBulges(t *testing.T) {
	hatch := parseHatch(t, pairs(
		[]CodePair{
			NewIntCodePair(91, 1),
			NewIntCodePair(92, 2),
			NewShortCodePair(72, 0),
			NewShortCodePair(73, 1),
			NewIntCodePair(93, 2),
		},
		hatchPoint(10, 0.0, 0.0),
		hatchPoint(10, 1.0, 0.0),
		[]CodePair{NewIntCodePair(97, 0)},
	)...)
	assertEqInt(t, 2, len(hatch.Paths[0].Vertices))
	assert(t, hatch.Paths[0].Bulges == nil, "expected no bulges")
}

func TestReadHatchBulgeWithoutHasBulgeFlag(t *testing.T) {
	hatch := parseHatch(t, pairs(
		[]CodePair{
			NewIntCodePair(91, 1),
			NewIntCodePair(92, 2),
			NewShortCodePair(72, 0),
			NewIntCodePair(93, 3),
		},
		hatchPoint(10, 0.0, 0.0),
		hatchPoint(10, 1.0, 0.0), []CodePair{NewDoubleCodePair(42, 0.5)},
		hatchPoint(10, 1.0, 1.0),
	)...)
	assertEqInt(t, 3, len(hatch.Paths[0].Bulges))
	assertEqFloat64(t, 0.0, hatch.Paths[0].Bulges[0])
	assertEqFloat64(t, 0.5, hatch.Paths[0].Bulges[1])
	assertEqFloat64(t, 0.0, hatch.Paths[0].Bulges[2])
}

func TestReadHatchEdgePath(t *testing.T) {
	hatch := parseHatch(t, pairs(
		[]CodePair{
			NewIntCodePair(91, 1),
			NewIntCodePair(92, 1),
			NewIntCodePair(93, 4),
			// line
			NewShortCodePair(72, 1),
		},
		hatchPoint(10, -5.0, 0.0),
		hatchPoint(11, 5.0, 0.0),
		// counter-clockwise arc
		[]CodePair{NewShortCodePair(72, 2)},
		hatchPoint(10, 0.0, 0.0),
		[]CodePair{
			NewDoubleCodePair(40, 5.0),
			NewDoubleCodePair(50, 0.0),
			NewDoubleCodePair(51, 180.0),
			NewShortCodePair(73, 1),
			// clockwise arc
			NewShortCodePair(72, 2),
		},
		hatchPoint(10, 1.0, 2.0),
		[]CodePair{
			NewDoubleCodePair(40, 3.0),
			NewDoubleCodePair(50, 270.0),
			NewDoubleCodePair(51, 360.0),
			NewShortCodePair(73, 0),
			// ellipse
			NewShortCodePair(72, 3),
		},
		hatchPoint(10, 0.0, 0.0),
		hatchPoint(11, 4.0, 0.0),
		[]CodePair{
			NewDoubleCodePair(40, 0.5),
			NewDoubleCodePair(50, 0.0),
			NewDoubleCodePair(51, 90.0),
			NewShortCodePair(73, 1),
			NewIntCodePair(97, 0),
			NewShortCodePair(75, 0),
		},
	)...)

	path := hatch.Paths[0]
	assertEqBool(t, false, path.IsPolyline())
	assertEqInt(t, 0, len(path.Vertices))
	assertEqInt(t, 4, len(path.Edges))

	line := path.Edges[0].(*HatchLineEdge)
	assertEqFloat64(t, -5.0, line.Start[0])
	assertEqFloat64(t, 5.0, line.End[0])

	arc := path.Edges[1].(*HatchArcEdge)
	assertEqFloat64(t, 5.0, arc.Radius)
	assertEqFloat64(t, 180.0, arc.EndAngle)
	assertEqBool(t, true, arc.IsCounterClockwise)

	clockwise := path.Edges[2].(*HatchArcEdge)
	assertEqFloat64(t, 2.0, clockwise.Center[1])
	assertEqFloat64(t, 270.0, clockwise.StartAngle)
	assertEqBool(t, false, clockwise.IsCounterClockwise)

	ellipse := path.Edges[3].(*HatchEllipseEdge)
	assertEqFloat64(t, 4.0, ellipse.MajorAxis[0])
	assertEqFloat64(t, 0.5, ellipse.MinorAxisRatio)
	assertEqFloat64(t, 90.0, ellipse.EndAngle)
	assertEqBool(t, true, ellipse.IsCounterClockwise)
}

func splineEdgePairs(withFitData bool) []CodePair {
	result := pairs(
		[]CodePair{
			NewShortCodePair(72, 4),
			NewIntCodePair(94, 2),
			NewShortCodePair(73, 1),
			NewShortCodePair(74, 0),
			NewIntCodePair(95, 6),
			NewIntCodePair(96, 3),
			NewDoubleCodePair(40, 0.0),
			NewDoubleCodePair(40, 0.0),
			NewDoubleCodePair(40, 0.0),
			NewDoubleCodePair(40, 1.0),
			NewDoubleCodePair(40, 1.0),
			NewDoubleCodePair(40, 1.0),
		},
		hatchPoint(10, 1.0, 0.0), []CodePair{NewDoubleCodePair(42, 1.0)},
		hatchPoint(10, 1.0, 1.0), []CodePair{NewDoubleCodePair(42, 0.7071)},
		hatchPoint(10, 0.0, 1.0), []CodePair{NewDoubleCodePair(42, 1.0)},
	)
	if withFitData {
		result = append(result, pairs(
			[]CodePair{NewIntCodePair(97, 2)},
			hatchPoint(11, 1.0, 0.0),
			hatchPoint(11, 0.0, 1.0),
			hatchPoint(12, 0.0, 1.0),
			hatchPoint(13, -1.0, 0.0),
		)...)
	}
	return result
}

func TestReadHatchSplineEdgeWithFitData(t *testing.T) {
	hatch := parseHatch(t, pairs(
		[]CodePair{
			NewIntCodePair(91, 1),
			NewIntCodePair(92, 1),
			NewIntCodePair(93, 1),
		},
		splineEdgePairs(true),
		[]CodePair{
			NewIntCodePair(97, 1),
			NewStringCodePair(330, "2C"),
		},
	)...)

	spline := hatch.Paths[0].Edges[0].(*HatchSplineEdge)
	assertEqInt(t, 2, spline.Degree)
	assertEqBool(t, true, spline.IsRational)
	assertEqBool(t, false, spline.IsPeriodic)
	assertEqInt(t, 6, len(spline.Knots))
	assertEqInt(t, 3, len(spline.ControlPoints))
	assertEqInt(t, 3, len(spline.Weights))
	assertEqFloat64(t, 0.7071, spline.Weights[1])
	assertEqInt(t, 2, len(spline.FitPoints))
	assertEqFloat64(t, -1.0, spline.EndTangent[0])
	assertEqInt(t, 1, len(hatch.Paths[0].SourceHandles))
}

func TestReadHatchSplineEdgeWithoutFitData(t *testing.T) {
	// before R2010 the 97 after the last spline edge is the source boundary count
	hatch := parseHatch(t, pairs(
		[]CodePair{
			NewIntCodePair(91, 1),
			NewIntCodePair(92, 1),
			NewIntCodePair(93, 1),
		},
		splineEdgePairs(false),
		[]CodePair{
			NewIntCodePair(97, 1),
			NewStringCodePair(330, "2C"),
			NewShortCodePair(75, 0),
		},
	)...)

	spline := hatch.Paths[0].Edges[0].(*HatchSplineEdge)
	assertEqInt(t, 3, len(spline.ControlPoints))
	assertEqInt(t, 0, len(spline.FitPoints))
	assertEqInt(t, 1, len(hatch.Paths[0].SourceHandles))
	assertEqUInt64(t, 0x2C, uint64(hatch.Paths[0].SourceHandles[0]))
}

func TestReadSolidHatchWithSeedPointsAndGradient(t *testing.T) {
	hatch := parseHatch(t, pairs(
		[]CodePair{
			NewStringCodePair(2, "SOLID"),
			NewShortCodePair(70, 1),
			NewShortCodePair(71, 0),
			NewIntCodePair(91, 1),
			NewIntCodePair(92, 3),
			NewShortCodePair(72, 0),
			NewShortCodePair(73, 1),
			NewIntCodePair(93, 4),
		},
		hatchPoint(10, 0.0, 0.0),
		hatchPoint(10, 10.0, 0.0),
		hatchPoint(10, 10.0, 10.0),
		hatchPoint(10, 0.0, 10.0),
		[]CodePair{
			NewIntCodePair(97, 0),
			NewShortCodePair(75, 0),
			NewShortCodePair(76, 1),
			NewDoubleCodePair(47, 0.25),
			NewIntCodePair(98, 1),
		},
		hatchPoint(10, 555.0, 555.0),
		[]CodePair{
			NewIntCodePair(450, 1),
			NewIntCodePair(451, 0),
			NewDoubleCodePair(460, 0.5),
			NewDoubleCodePair(461, 0.0),
			NewIntCodePair(452, 0),
			NewDoubleCodePair(462, 1.0),
			NewIntCodePair(453, 2),
			NewDoubleCodePair(463, 0.0),
			NewShortCodePair(63, 5),
			NewIntCodePair(421, 255),
			NewDoubleCodePair(463, 1.0),
			NewIntCodePair(421, 16776960),
			NewStringCodePair(470, "LINEAR"),
		},
	)...)

	assertEqBool(t, true, hatch.SolidFill)
	assertEqInt(t, 4, len(hatch.Paths[0].Vertices))
	assertEqFloat64(t, 10.0, hatch.Paths[0].Vertices[3][1])
	assertEqInt(t, 1, len(hatch.SeedPoints))
	assertEqFloat64(t, 555.0, hatch.SeedPoints[0][0])
	assertEqFloat64(t, 0.25, hatch.PixelSize)

	gradient := hatch.Gradient
	assert(t, gradient != nil, "expected a gradient")
	assertEqBool(t, true, gradient.IsGradient)
	assertEqFloat64(t, 0.5, gradient.Angle)
	assertEqFloat64(t, 1.0, gradient.Tint)
	assertEqString(t, "LINEAR", gradient.Name)
	assertEqInt(t, 2, len(gradient.Colors))
	assertEqInt(t, 5, int(gradient.Colors[0].Color))
	assertEqInt(t, 255, gradient.Colors[0].TrueColor)
	assertEqFloat64(t, 1.0, gradient.Colors[1].Value)
	assertEqInt(t, 16776960, gradient.Colors[1].TrueColor)
}

func TestHatchGradientTrueColorBlack(t *testing.T) {
	// 421 = 0 is black; a color without 421 has no true color
	hatch := NewHatch()
	hatch.Gradient = &HatchGradient{IsGradient: true, Name: "LINEAR", Colors: []HatchGradientColor{
		{Value: 0, Color: 5, TrueColor: 0, HasTrueColor: true},
		{Value: 1, Color: 3},
	}}
	written := hatchDataCodePairs(t, hatch, R2004)
	assertEqCodePairs(t, []CodePair{NewIntCodePair(421, 0)}, codePairsWithCode(421, written))

	reread := parseHatch(t, written[1:]...)
	assertEqBool(t, true, reread.Gradient.Colors[0].HasTrueColor)
	assertEqInt(t, 0, reread.Gradient.Colors[0].TrueColor)
	assertEqBool(t, false, reread.Gradient.Colors[1].HasTrueColor)
}

func TestReadHatchPatternLines(t *testing.T) {
	hatch := parseHatch(t,
		NewStringCodePair(2, "ANSI32"),
		NewShortCodePair(70, 0),
		NewIntCodePair(91, 0),
		NewShortCodePair(75, 0),
		NewShortCodePair(76, 1),
		NewDoubleCodePair(52, 45.0),
		NewDoubleCodePair(41, 2.0),
		NewShortCodePair(77, 1),
		NewShortCodePair(78, 2),
		NewDoubleCodePair(53, 45.0),
		NewDoubleCodePair(43, 0.0),
		NewDoubleCodePair(44, 0.0),
		NewDoubleCodePair(45, -2.0),
		NewDoubleCodePair(46, 2.0),
		NewShortCodePair(79, 0),
		NewDoubleCodePair(53, 45.0),
		NewDoubleCodePair(43, 0.5),
		NewDoubleCodePair(44, 0.0),
		NewDoubleCodePair(45, -2.0),
		NewDoubleCodePair(46, 2.0),
		NewShortCodePair(79, 2),
		NewDoubleCodePair(49, 1.5),
		NewDoubleCodePair(49, -0.5),
		NewIntCodePair(98, 0),
	)
	assertEqFloat64(t, 45.0, hatch.PatternAngle)
	assertEqFloat64(t, 2.0, hatch.PatternScale)
	assertEqBool(t, true, hatch.IsPatternDouble)
	assertEqInt(t, 2, len(hatch.PatternLines))
	assertEqFloat64(t, 0.5, hatch.PatternLines[1].BaseX)
	assertEqInt(t, 0, len(hatch.PatternLines[0].Dashes))
	assertEqInt(t, 2, len(hatch.PatternLines[1].Dashes))
	assertEqFloat64(t, -0.5, hatch.PatternLines[1].Dashes[1])
	assertEqInt(t, 0, len(hatch.SeedPoints))
}

func TestReadHatchImageByteCountBeforeHatchData(t *testing.T) {
	// 92 is the proxy graphics byte count in the common entity data and the path type in the hatch data
	hatch := parseEntity(t, "HATCH",
		NewStringCodePair(100, "AcDbEntity"),
		NewIntCodePair(92, 8),
		NewStringCodePair(100, "AcDbHatch"),
		NewIntCodePair(91, 1),
		NewIntCodePair(92, 2),
		NewIntCodePair(93, 0),
	).(*Hatch)
	assertEqInt(t, 8, hatch.ImageByteCount())
	assertEqInt(t, 2, hatch.Paths[0].PathType)
}

func TestReadHatchWithoutSubclassMarkers(t *testing.T) {
	hatch := parseEntity(t, "HATCH",
		NewStringCodePair(8, "FILL"),
		NewDoubleCodePair(10, 0.0),
		NewDoubleCodePair(20, 0.0),
		NewDoubleCodePair(30, 0.0),
		NewStringCodePair(2, "SOLID"),
		NewShortCodePair(70, 1),
		NewIntCodePair(91, 1),
		NewIntCodePair(92, 2),
		NewIntCodePair(93, 1),
		NewDoubleCodePair(10, 3.0),
		NewDoubleCodePair(20, 4.0),
	).(*Hatch)
	assertEqString(t, "FILL", hatch.Layer())
	assertEqBool(t, true, hatch.SolidFill)
	assertEqFloat64(t, 4.0, hatch.Paths[0].Vertices[0][1])
}

func TestReadHatchWithTruncatedCounts(t *testing.T) {
	// counts larger than the data must not swallow the following values
	hatch := parseHatch(t,
		NewIntCodePair(91, 2),
		NewIntCodePair(92, 2),
		NewIntCodePair(93, 5),
		NewDoubleCodePair(10, 1.0),
		NewDoubleCodePair(20, 2.0),
		NewShortCodePair(75, 1),
		NewShortCodePair(76, 1),
	)
	assertEqInt(t, 1, len(hatch.Paths))
	assertEqInt(t, 1, len(hatch.Paths[0].Vertices))
	assertEqInt(t, int(HatchStyleOutermost), int(hatch.Style))
}

// hatchDataCodePairs returns the written pairs from the AcDbHatch subclass marker on.
func hatchDataCodePairs(t *testing.T, hatch *Hatch, version AcadVersion) []CodePair {
	written := allCodePairs(hatch, version)
	for i, pair := range written {
		if pair.Code == 100 && pair.Value.(StringCodePairValue).Value == "AcDbHatch" {
			return written[i:]
		}
	}
	t.Fatal("missing AcDbHatch subclass marker")
	return nil
}

func TestWritePatternHatchWithPolylinePath(t *testing.T) {
	hatch := NewHatch()
	hatch.SetElevation(2.0)
	hatch.PatternName = "ANSI31"
	hatch.SolidFill = false
	hatch.IsAssociative = true
	hatch.PatternAngle = 45.0
	hatch.PatternScale = 2.0
	hatch.Paths = []HatchBoundaryPath{{
		PathType:      3,
		Vertices:      [][2]float64{{0, 0}, {10, 0}},
		Bulges:        []float64{0.0, 1.0},
		IsClosed:      true,
		SourceHandles: []Handle{0x2B},
	}}
	hatch.PatternLines = []HatchPatternLine{{Angle: 45.0, OffsetX: -2.0, OffsetY: 2.0, Dashes: []float64{1.5, -0.5}}}

	assertEqCodePairs(t, []CodePair{
		NewStringCodePair(100, "AcDbHatch"),
		NewDoubleCodePair(10, 0.0),
		NewDoubleCodePair(20, 0.0),
		NewDoubleCodePair(30, 2.0),
		NewDoubleCodePair(210, 0.0),
		NewDoubleCodePair(220, 0.0),
		NewDoubleCodePair(230, 1.0),
		NewStringCodePair(2, "ANSI31"),
		NewShortCodePair(70, 0),
		NewShortCodePair(71, 1),
		NewIntCodePair(91, 1),
		NewIntCodePair(92, 3),
		NewShortCodePair(72, 1),
		NewShortCodePair(73, 1),
		NewIntCodePair(93, 2),
		NewDoubleCodePair(10, 0.0),
		NewDoubleCodePair(20, 0.0),
		NewDoubleCodePair(42, 0.0),
		NewDoubleCodePair(10, 10.0),
		NewDoubleCodePair(20, 0.0),
		NewDoubleCodePair(42, 1.0),
		NewIntCodePair(97, 1),
		NewStringCodePair(330, "2B"),
		NewShortCodePair(75, 0),
		NewShortCodePair(76, 1),
		NewDoubleCodePair(52, 45.0),
		NewDoubleCodePair(41, 2.0),
		NewShortCodePair(77, 0),
		NewShortCodePair(78, 1),
		NewDoubleCodePair(53, 45.0),
		NewDoubleCodePair(43, 0.0),
		NewDoubleCodePair(44, 0.0),
		NewDoubleCodePair(45, -2.0),
		NewDoubleCodePair(46, 2.0),
		NewShortCodePair(79, 2),
		NewDoubleCodePair(49, 1.5),
		NewDoubleCodePair(49, -0.5),
		NewIntCodePair(98, 0),
	}, hatchDataCodePairs(t, hatch, R2010))
}

func TestWriteSolidHatchGradientOnlyForR2004AndLater(t *testing.T) {
	hatch := NewHatch()
	hatch.SeedPoints = [][2]float64{{1.0, 2.0}}
	hatch.Gradient = &HatchGradient{Name: "LINEAR", Colors: []HatchGradientColor{{Value: 0.0, Color: 5, TrueColor: 255}}}

	written := hatchDataCodePairs(t, hatch, R2004)
	styleIndex := 0
	for written[styleIndex].Code != 75 {
		styleIndex++
	}
	assertEqCodePairs(t, []CodePair{
		NewShortCodePair(75, 0),
		NewShortCodePair(76, 1),
		NewIntCodePair(98, 1),
		NewDoubleCodePair(10, 1.0),
		NewDoubleCodePair(20, 2.0),
		NewIntCodePair(450, 0),
		NewIntCodePair(451, 0),
		NewDoubleCodePair(460, 0.0),
		NewDoubleCodePair(461, 0.0),
		NewIntCodePair(452, 0),
		NewDoubleCodePair(462, 0.0),
		NewIntCodePair(453, 1),
		NewDoubleCodePair(463, 0.0),
		NewShortCodePair(63, 5),
		NewIntCodePair(421, 255),
		NewStringCodePair(470, "LINEAR"),
	}, written[styleIndex:])

	r2000 := hatchDataCodePairs(t, hatch, R2000)
	assertNotContainsCodePairs(t, []CodePair{NewIntCodePair(450, 0)}, r2000)
	// solid fills have no pattern definition
	assertNotContainsCodePairs(t, []CodePair{NewShortCodePair(78, 0)}, r2000)
}

func TestWriteHatchSplineFitDataOnlyForR2010AndLater(t *testing.T) {
	hatch := NewHatch()
	hatch.Paths = []HatchBoundaryPath{{
		PathType: 1,
		Edges: []HatchEdge{&HatchSplineEdge{
			Degree:        1,
			Knots:         []float64{0, 0, 1, 1},
			ControlPoints: [][2]float64{{0, 0}, {1, 1}},
			FitPoints:     [][2]float64{{0, 0}, {1, 1}},
			EndTangent:    [2]float64{1, 1},
		}},
	}}
	fitData := []CodePair{
		NewIntCodePair(97, 2),
		NewDoubleCodePair(11, 0.0),
		NewDoubleCodePair(21, 0.0),
		NewDoubleCodePair(11, 1.0),
		NewDoubleCodePair(21, 1.0),
		NewDoubleCodePair(12, 0.0),
		NewDoubleCodePair(22, 0.0),
		NewDoubleCodePair(13, 1.0),
		NewDoubleCodePair(23, 1.0),
		NewIntCodePair(97, 0),
	}
	assertContainsCodePairs(t, fitData, hatchDataCodePairs(t, hatch, R2010))
	assertNotContainsCodePairs(t, []CodePair{NewIntCodePair(97, 2)}, hatchDataCodePairs(t, hatch, R2007))
}

func TestWriteHatchOnlyForR14AndLater(t *testing.T) {
	hatch := NewHatch()
	assertNotContainsCodePairs(t, []CodePair{NewStringCodePair(0, "HATCH")}, drawingCodePairsFromEntity(t, hatch, R13))
	assertContainsCodePairs(t, []CodePair{NewStringCodePair(0, "HATCH")}, drawingCodePairsFromEntity(t, hatch, R14))
}

func TestRoundTripHatch(t *testing.T) {
	hatch := NewHatch()
	hatch.SetLayer("FILL")
	hatch.SetElevation(3.0)
	hatch.ExtrusionDirection = Vector{0.0, 0.0, -1.0}
	hatch.PatternName = "CUSTOM"
	hatch.SolidFill = false
	hatch.PatternType = HatchPatternTypeCustom
	hatch.Style = HatchStyleEntire
	hatch.PatternAngle = 30.0
	hatch.PatternScale = 0.5
	hatch.IsPatternDouble = true
	hatch.PixelSize = 0.125
	hatch.Paths = []HatchBoundaryPath{
		{
			PathType: 2,
			Vertices: [][2]float64{{0, 0}, {4, 0}, {4, 4}},
			Bulges:   []float64{0, 0.5, 0},
			IsClosed: true,
		},
		{
			PathType: 1,
			Edges: []HatchEdge{
				&HatchLineEdge{Start: [2]float64{0, 0}, End: [2]float64{2, 0}},
				&HatchArcEdge{Center: [2]float64{1, 0}, Radius: 1, StartAngle: 0, EndAngle: 180, IsCounterClockwise: true},
				&HatchEllipseEdge{Center: [2]float64{0, 0}, MajorAxis: [2]float64{3, 0}, MinorAxisRatio: 0.5, StartAngle: 90, EndAngle: 270},
				&HatchSplineEdge{
					Degree:        2,
					IsRational:    true,
					Knots:         []float64{0, 0, 0, 1, 1, 1},
					ControlPoints: [][2]float64{{1, 0}, {1, 1}, {0, 1}},
					Weights:       []float64{1, 0.5, 1},
					FitPoints:     [][2]float64{{1, 0}, {0, 1}},
					StartTangent:  [2]float64{0, 1},
					EndTangent:    [2]float64{-1, 0},
				},
			},
		},
	}
	hatch.PatternLines = []HatchPatternLine{
		{Angle: 30, BaseX: 1, BaseY: 2, OffsetX: 3, OffsetY: 4, Dashes: []float64{1, -1}},
		{Angle: 120, Dashes: nil},
	}
	hatch.SeedPoints = [][2]float64{{1, 1}, {2, 2}}
	hatch.Gradient = &HatchGradient{
		IsGradient: true,
		Angle:      0.25,
		Tint:       0.5,
		Colors:     []HatchGradientColor{{Value: 0, TrueColor: 255, HasTrueColor: true}, {Value: 1, Color: 3, TrueColor: 0, HasTrueColor: true}},
		Name:       "SPHERICAL",
	}

	drawing := *NewDrawing()
	drawing.Header.Version = R2018
	drawing.Entities = append(drawing.Entities, hatch)
	roundTripped := roundTripDrawing(t, &drawing)
	assertEqInt(t, 1, len(roundTripped.Entities))
	actual := roundTripped.Entities[0].(*Hatch)

	assertEqString(t, "FILL", actual.Layer())
	assertEqFloat64(t, 3.0, actual.Elevation())
	assertEqVector(t, hatch.ExtrusionDirection, actual.ExtrusionDirection)
	assertEqString(t, "CUSTOM", actual.PatternName)
	assertEqBool(t, false, actual.SolidFill)
	assertEqInt(t, int(HatchPatternTypeCustom), int(actual.PatternType))
	assertEqInt(t, int(HatchStyleEntire), int(actual.Style))
	assertEqFloat64(t, 30.0, actual.PatternAngle)
	assertEqFloat64(t, 0.5, actual.PatternScale)
	assertEqBool(t, true, actual.IsPatternDouble)
	assertEqFloat64(t, 0.125, actual.PixelSize)
	assert(t, reflect.DeepEqual(hatch.Paths, actual.Paths), "boundary paths differ after round trip")
	assert(t, reflect.DeepEqual(hatch.PatternLines[0], actual.PatternLines[0]), "pattern lines differ after round trip")
	assertEqInt(t, 0, len(actual.PatternLines[1].Dashes))
	assert(t, reflect.DeepEqual(hatch.SeedPoints, actual.SeedPoints), "seed points differ after round trip")
	assert(t, reflect.DeepEqual(hatch.Gradient, actual.Gradient), "gradient differs after round trip")
}

package dxf

import (
	"fmt"
	"math"
	"testing"
)

func assertNearPoint2(t *testing.T, expected, actual [2]float64) {
	t.Helper()
	if math.Hypot(expected[0]-actual[0], expected[1]-actual[1]) > nearEpsilon {
		t.Errorf("Expected: %v\nActual: %v", expected, actual)
	}
}

func TestPolygonOfLineEdges(t *testing.T) {
	path := HatchBoundaryPath{PathType: 1, Edges: []HatchEdge{
		&HatchLineEdge{Start: [2]float64{0, 0}, End: [2]float64{1, 0}},
		&HatchLineEdge{Start: [2]float64{1, 0}, End: [2]float64{1, 1}},
		&HatchLineEdge{Start: [2]float64{1, 1}, End: [2]float64{0, 1}},
		&HatchLineEdge{Start: [2]float64{0, 1}, End: [2]float64{0, 0}},
	}}
	points, exact := path.Polygon(0.01)
	assertEqBool(t, true, exact)
	assertEqInt(t, 4, len(points))
	assertNearPoint2(t, [2]float64{1, 1}, points[2])
}

func TestPolygonOfFullCircleArcEdge(t *testing.T) {
	const radius, tolerance = 10.0, 0.01
	center := [2]float64{3, 4}
	path := HatchBoundaryPath{PathType: 1, Edges: []HatchEdge{
		&HatchArcEdge{Center: center, Radius: radius, StartAngle: 0, EndAngle: 360, IsCounterClockwise: true},
	}}
	points, _ := path.Polygon(tolerance)

	// 2·acos(1 - tolerance/radius) is the largest step: ceil(2π / 0.0894) segments
	assertEqInt(t, 71, len(points))
	for i, p := range points {
		assertNearFloat64(t, radius, math.Hypot(p[0]-center[0], p[1]-center[1]))
		next := points[(i+1)%len(points)]
		middle := [2]float64{(p[0] + next[0]) / 2, (p[1] + next[1]) / 2}
		deviation := radius - math.Hypot(middle[0]-center[0], middle[1]-center[1])
		assert(t, deviation <= tolerance, fmt.Sprintf("chord %d deviates by %v", i, deviation))
	}
}

func TestPolygonOfClockwiseArcEdge(t *testing.T) {
	// clockwise edges store mirrored angles: 270..360 is the arc from 90° back to 0°
	points, _ := flattenHatchEdge(&HatchArcEdge{Radius: 1, StartAngle: 270, EndAngle: 360, IsCounterClockwise: false}, 0.001)
	assertNearPoint2(t, [2]float64{0, 1}, points[0])
	assertNearPoint2(t, [2]float64{1, 0}, points[len(points)-1])
	for _, p := range points {
		assert(t, p[0] >= -nearEpsilon && p[1] >= -nearEpsilon, fmt.Sprintf("%v is not in the first quadrant", p))
	}
}

func TestPolygonOfHalfDiscEdges(t *testing.T) {
	// the shape the old reader turned into [[-5 0] [0 0]]
	path := HatchBoundaryPath{PathType: 1, Edges: []HatchEdge{
		&HatchLineEdge{Start: [2]float64{-5, 0}, End: [2]float64{5, 0}},
		&HatchArcEdge{Radius: 5, StartAngle: 0, EndAngle: 180, IsCounterClockwise: true},
	}}
	points, _ := path.Polygon(0.01)
	assertNearPoint2(t, [2]float64{-5, 0}, points[0])
	assertNearPoint2(t, [2]float64{5, 0}, points[1])
	assert(t, len(points) > 10, "expected the arc to be flattened")
	for _, p := range points {
		assert(t, p[1] >= -nearEpsilon, fmt.Sprintf("%v is below the chord", p))
	}
	// the closing point is not repeated
	assert(t, !nearlyEqual2(points[0], points[len(points)-1]), "expected an implicitly closed polygon")
}

func TestPolygonOfBulgedPolyline(t *testing.T) {
	// a bulge of 1 is a counter-clockwise semicircle: from (0,0) to (2,0) it passes below the chord
	path := HatchBoundaryPath{
		PathType: 2,
		Vertices: [][2]float64{{0, 0}, {2, 0}},
		Bulges:   []float64{1, 0},
		IsClosed: true,
	}
	points, _ := path.Polygon(0.001)
	lowest := 0.0
	for _, p := range points {
		assertNearFloat64(t, 1.0, math.Hypot(p[0]-1, p[1]))
		lowest = math.Min(lowest, p[1])
	}
	assertNearFloat64(t, -1.0, lowest)
	assertNearPoint2(t, [2]float64{0, 0}, points[0])

	// a negative bulge goes the other way
	path.Bulges = []float64{-1, 0}
	points, _ = path.Polygon(0.001)
	highest := 0.0
	for _, p := range points {
		highest = math.Max(highest, p[1])
	}
	assertNearFloat64(t, 1.0, highest)
}

func TestPolygonOfPolylineWithoutBulges(t *testing.T) {
	path := HatchBoundaryPath{PathType: 2, Vertices: [][2]float64{{0, 0}, {1, 0}, {1, 1}}, IsClosed: true}
	points, exact := path.Polygon(0)
	assertEqBool(t, true, exact)
	assertEqInt(t, 3, len(points))
}

func TestPolygonOfEllipseEdge(t *testing.T) {
	edge := &HatchEllipseEdge{MajorAxis: [2]float64{4, 0}, MinorAxisRatio: 0.5, StartAngle: 0, EndAngle: 90, IsCounterClockwise: true}
	points, _ := flattenHatchEdge(edge, 0.001)
	assertNearPoint2(t, [2]float64{4, 0}, points[0])
	assertNearPoint2(t, [2]float64{0, 2}, points[len(points)-1])
	for _, p := range points {
		assertNearFloat64(t, 1.0, (p[0]/4)*(p[0]/4)+(p[1]/2)*(p[1]/2))
	}

	// angles are real angles around the center, not ellipse parameters
	edge.EndAngle = 45
	points, _ = flattenHatchEdge(edge, 0.001)
	end := points[len(points)-1]
	assertNearFloat64(t, end[0], end[1])
}

func TestPolygonOfLinearSplineEdge(t *testing.T) {
	edge := &HatchSplineEdge{
		Degree:        1,
		Knots:         []float64{0, 0, 1, 2, 2},
		ControlPoints: [][2]float64{{0, 0}, {2, 0}, {2, 2}},
	}
	points, exact := flattenHatchEdge(edge, 0.001)
	assertEqBool(t, true, exact)
	assertNearPoint2(t, [2]float64{0, 0}, points[0])
	assertNearPoint2(t, [2]float64{2, 2}, points[len(points)-1])
	for _, p := range points {
		onFirstLeg := math.Abs(p[1]) < nearEpsilon
		onSecondLeg := math.Abs(p[0]-2) < nearEpsilon
		assert(t, onFirstLeg || onSecondLeg, fmt.Sprintf("%v is not on the control polygon", p))
	}
}

func TestPolygonOfRationalQuarterCircleSplineEdge(t *testing.T) {
	edge := &HatchSplineEdge{
		Degree:        2,
		IsRational:    true,
		Knots:         []float64{0, 0, 0, 1, 1, 1},
		ControlPoints: [][2]float64{{1, 0}, {1, 1}, {0, 1}},
		Weights:       []float64{1, math.Sqrt2 / 2, 1},
	}
	points, exact := flattenHatchEdge(edge, 1e-4)
	assertEqBool(t, true, exact)
	assertNearPoint2(t, [2]float64{1, 0}, points[0])
	assertNearPoint2(t, [2]float64{0, 1}, points[len(points)-1])
	assert(t, len(points) > 8, "expected the curve to be refined")
	for _, p := range points {
		assert(t, math.Abs(math.Hypot(p[0], p[1])-1.0) < 1e-12, fmt.Sprintf("%v is not on the unit circle", p))
	}
}

func TestPolygonOfInvalidSplineEdgeFallsBack(t *testing.T) {
	edge := &HatchSplineEdge{
		Degree:        3,
		Knots:         []float64{0, 1},
		ControlPoints: [][2]float64{{0, 0}, {1, 1}, {2, 0}},
		FitPoints:     [][2]float64{{0, 0}, {2, 0}},
	}
	points, exact := flattenHatchEdge(edge, 0.01)
	assertEqBool(t, false, exact)
	assertEqInt(t, 2, len(points))

	edge.FitPoints = nil
	points, exact = flattenHatchEdge(edge, 0.01)
	assertEqBool(t, false, exact)
	assertEqInt(t, 3, len(points))
}

func TestBoundaryPolygonsWCS(t *testing.T) {
	hatch := NewHatch()
	hatch.SetElevation(2.0)
	hatch.ExtrusionDirection = Vector{0, 0, -1}
	hatch.Paths = []HatchBoundaryPath{{PathType: 2, Vertices: [][2]float64{{1, 0}, {1, 1}, {0, 1}}, IsClosed: true}}

	ocs := hatch.BoundaryPolygons(0)
	assertEqInt(t, 1, len(ocs))
	assertNearPoint2(t, [2]float64{1, 0}, ocs[0][0])

	wcs := hatch.BoundaryPolygonsWCS(0)
	assertEqInt(t, 3, len(wcs[0]))
	// a mirrored OCS flips X and Z
	assertNearPoint(t, Point{-1, 0, -2}, wcs[0][0])
	assertNearPoint(t, Point{0, 1, -2}, wcs[0][2])
}

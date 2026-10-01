package dxf

import (
	"math"
)

const (
	// relativeFlattenTolerance is used when no tolerance is given: a fraction of each curve's size
	relativeFlattenTolerance = 1e-3
	maxCurveSegments         = 4096
	maxSplineSubdivisions    = 12
	// fullCurveEpsilon (degrees) absorbs rounding noise when an edge covers a full circle or ellipse
	fullCurveEpsilon = 1e-9
)

// Polygon returns the boundary path as a polygon in the hatch's object coordinate system. Curves (bulges, arcs,
// ellipses and splines) are flattened so that no segment deviates from the curve by more than tolerance; a
// tolerance <= 0 uses 0.1% of each curve's size. The polygon is implicitly closed: the last point does not repeat the
// first. exact is false if a spline could not be evaluated and its fit points or control polygon were used instead.
func (p *HatchBoundaryPath) Polygon(tolerance float64) (points [][2]float64, exact bool) {
	exact = true
	if p.IsPolyline() {
		points = flattenBulgedPolyline(p.Vertices, p.Bulges, p.IsClosed, tolerance)
	} else {
		for _, edge := range p.Edges {
			edgePoints, edgeExact := flattenHatchEdge(edge, tolerance)
			exact = exact && edgeExact
			for i, point := range edgePoints {
				if i == 0 && len(points) > 0 && nearlyEqual2(points[len(points)-1], point) {
					// edges are chained: skip the start point that repeats the previous end point
					continue
				}
				points = append(points, point)
			}
		}
	}

	if len(points) > 1 && nearlyEqual2(points[0], points[len(points)-1]) {
		points = points[:len(points)-1]
	}
	return
}

// BoundaryPolygons returns all boundary paths as polygons in the hatch's object coordinate system; see
// HatchBoundaryPath.Polygon.
func (h *Hatch) BoundaryPolygons(tolerance float64) (polygons [][][2]float64) {
	for i := range h.Paths {
		points, _ := h.Paths[i].Polygon(tolerance)
		polygons = append(polygons, points)
	}
	return
}

// BoundaryPolygonsWCS returns all boundary paths as polygons in world coordinates, applying the hatch's extrusion
// direction and elevation.
func (h *Hatch) BoundaryPolygonsWCS(tolerance float64) (polygons [][]Point) {
	toWCS := OCSToWCSMatrix(h.ExtrusionDirection)
	for _, ocsPoints := range h.BoundaryPolygons(tolerance) {
		points := make([]Point, len(ocsPoints))
		for i, p := range ocsPoints {
			points[i] = toWCS.TransformPoint(Point{p[0], p[1], h.elevation})
		}
		polygons = append(polygons, points)
	}
	return
}

func flattenBulgedPolyline(vertices [][2]float64, bulges []float64, isClosed bool, tolerance float64) (points [][2]float64) {
	if len(vertices) == 0 {
		return nil
	}

	bulgeAt := func(i int) float64 {
		if i < len(bulges) {
			return bulges[i]
		}
		return 0.0
	}

	points = append(points, vertices[0])
	segmentCount := len(vertices) - 1
	if isClosed {
		segmentCount++
	}
	for i := 0; i < segmentCount; i++ {
		start := vertices[i]
		end := vertices[(i+1)%len(vertices)]
		points = append(points, flattenBulge(start, end, bulgeAt(i), tolerance)...)
	}
	return
}

// flattenBulge returns the points after start along the segment from start to end with the given bulge (the tangent
// of a quarter of the arc's included angle; positive is counter-clockwise).
func flattenBulge(start, end [2]float64, bulge, tolerance float64) [][2]float64 {
	center, radius, startAngle, sweep, ok := bulgeArc(start, end, bulge)
	if !ok {
		return [][2]float64{end}
	}

	points := arcPoints(center, radius, startAngle, sweep, tolerance)
	// end exactly on the next vertex
	points[len(points)-1] = end
	return points
}

// bulgeArc returns the circular arc of a polyline segment with a bulge: its center, radius, start angle and signed
// sweep (radians, positive is counter-clockwise). It returns false for straight segments.
func bulgeArc(start, end [2]float64, bulge float64) (center [2]float64, radius, startAngle, sweep float64, ok bool) {
	dx, dy := end[0]-start[0], end[1]-start[1]
	chord := math.Hypot(dx, dy)
	if bulge == 0 || chord == 0 {
		return
	}

	sweep = 4.0 * math.Atan(bulge)
	radius = chord / (2.0 * math.Abs(math.Sin(sweep/2.0)))
	// the center is on the left of the chord for counter-clockwise arcs
	offset := chord * (1.0 - bulge*bulge) / (4.0 * bulge)
	center = [2]float64{
		start[0] + dx/2.0 - dy/chord*offset,
		start[1] + dy/2.0 + dx/chord*offset,
	}
	startAngle = math.Atan2(start[1]-center[1], start[0]-center[0])
	return center, radius, startAngle, sweep, true
}

func flattenHatchEdge(edge HatchEdge, tolerance float64) (points [][2]float64, exact bool) {
	switch edge := edge.(type) {
	case *HatchLineEdge:
		return [][2]float64{edge.Start, edge.End}, true
	case *HatchArcEdge:
		start, sweep := counterClockwiseAngles(edge.StartAngle, edge.EndAngle, edge.IsCounterClockwise)
		startRadians := start * math.Pi / 180.0
		points = append([][2]float64{pointOnCircle(edge.Center, edge.Radius, startRadians)},
			arcPoints(edge.Center, edge.Radius, startRadians, sweep*math.Pi/180.0, tolerance)...)
		if !edge.IsCounterClockwise {
			reversePoints(points)
		}
		return points, true
	case *HatchEllipseEdge:
		points = flattenEllipseEdge(edge, tolerance)
		if !edge.IsCounterClockwise {
			reversePoints(points)
		}
		return points, true
	case *HatchSplineEdge:
		return flattenSplineEdge(edge, tolerance)
	}
	return nil, false
}

// counterClockwiseAngles returns the start angle and counter-clockwise sweep (degrees) of an arc or ellipse edge.
// Clockwise edges store their angles mirrored (360 - angle); the returned range is then traversed in reverse.
func counterClockwiseAngles(startAngle, endAngle float64, isCounterClockwise bool) (start, sweep float64) {
	start, end := startAngle, endAngle
	if !isCounterClockwise {
		start, end = 360.0-endAngle, 360.0-startAngle
	}
	sweep = math.Mod(end-start, 360.0)
	if sweep < 0 {
		sweep += 360.0
	}
	if (sweep < fullCurveEpsilon || sweep > 360.0-fullCurveEpsilon) && endAngle != startAngle {
		// a full curve, possibly with rounding noise (e.g. 332.5 .. 692.5)
		sweep = 360.0
	}
	return start, sweep
}

func flattenEllipseEdge(edge *HatchEllipseEdge, tolerance float64) [][2]float64 {
	major := edge.MajorAxis
	minor := [2]float64{-major[1] * edge.MinorAxisRatio, major[0] * edge.MinorAxisRatio}
	pointAt := func(parameter float64) [2]float64 {
		sin, cos := math.Sincos(parameter)
		return [2]float64{
			edge.Center[0] + cos*major[0] + sin*minor[0],
			edge.Center[1] + cos*major[1] + sin*minor[1],
		}
	}

	startAngle, angleSweep := counterClockwiseAngles(edge.StartAngle, edge.EndAngle, edge.IsCounterClockwise)
	startParameter := ellipseAngleToParameter(edge.MinorAxisRatio, startAngle*math.Pi/180.0)
	endParameter := ellipseAngleToParameter(edge.MinorAxisRatio, (startAngle+angleSweep)*math.Pi/180.0)
	parameterSweep := math.Mod(endParameter-startParameter, 2.0*math.Pi)
	if parameterSweep < 0 {
		parameterSweep += 2.0 * math.Pi
	}
	if angleSweep >= 360.0 || (parameterSweep == 0 && angleSweep > 0) {
		parameterSweep = 2.0 * math.Pi
	}

	// in parameter space the curve bends at most like a circle with the major radius (|P''| <= a), so the circle's
	// segment count bounds the chord deviation
	count := arcSegmentCount(math.Hypot(major[0], major[1]), parameterSweep, tolerance)

	points := make([][2]float64, 0, count+1)
	for i := 0; i <= count; i++ {
		points = append(points, pointAt(startParameter+parameterSweep*float64(i)/float64(count)))
	}
	return points
}

// ellipseAngleToParameter converts a real angle around an ellipse's center into the ellipse parameter.
func ellipseAngleToParameter(ratio, angle float64) float64 {
	if ratio == 0 {
		return angle
	}
	sin, cos := math.Sincos(angle)
	return math.Atan2(sin/ratio, cos)
}

func flattenSplineEdge(edge *HatchSplineEdge, tolerance float64) ([][2]float64, bool) {
	degree := edge.Degree
	controlPoints := edge.ControlPoints
	knots := edge.Knots
	valid := degree >= 1 && len(controlPoints) > degree && len(knots) == len(controlPoints)+degree+1
	for i := 1; valid && i < len(knots); i++ {
		valid = knots[i] >= knots[i-1]
	}
	if valid {
		valid = knots[len(controlPoints)] > knots[degree]
	}
	if !valid {
		if len(edge.FitPoints) >= 2 {
			return append([][2]float64(nil), edge.FitPoints...), false
		}
		return append([][2]float64(nil), controlPoints...), false
	}

	weights := make([]float64, len(controlPoints))
	for i := range weights {
		weights[i] = 1.0
		if edge.IsRational && i < len(edge.Weights) && edge.Weights[i] > 0 {
			weights[i] = edge.Weights[i]
		}
	}
	if tolerance <= 0 {
		tolerance = relativeFlattenTolerance * boundingDiagonal(controlPoints)
	}

	evaluate := func(u float64) [2]float64 {
		return evaluateRationalBSpline(degree, knots, controlPoints, weights, u)
	}

	start := knots[degree]
	points := [][2]float64{evaluate(start)}
	for span := degree; span < len(controlPoints); span++ {
		u0, u1 := knots[span], knots[span+1]
		if u1 <= u0 {
			continue
		}
		// split every span a few times before refining, so curves that cross their chord are not missed
		pieces := degree + 1
		for piece := 0; piece < pieces; piece++ {
			a := u0 + (u1-u0)*float64(piece)/float64(pieces)
			b := u0 + (u1-u0)*float64(piece+1)/float64(pieces)
			points = appendSplinePoints(points, evaluate, a, b, evaluate(a), evaluate(b), tolerance, 0)
		}
	}
	return points, true
}

// appendSplinePoints appends the points after a up to b, subdividing until the curve stays within tolerance of the
// chord.
func appendSplinePoints(points [][2]float64, evaluate func(float64) [2]float64, a, b float64, pa, pb [2]float64, tolerance float64, depth int) [][2]float64 {
	middle := (a + b) / 2.0
	pm := evaluate(middle)
	if depth < maxSplineSubdivisions && distanceToSegment(pm, pa, pb) > tolerance {
		points = appendSplinePoints(points, evaluate, a, middle, pa, pm, tolerance, depth+1)
		return appendSplinePoints(points, evaluate, middle, b, pm, pb, tolerance, depth+1)
	}
	return append(points, pb)
}

// evaluateRationalBSpline evaluates a (rational) B-spline at u with de Boor's algorithm in homogeneous coordinates.
func evaluateRationalBSpline(degree int, knots []float64, controlPoints [][2]float64, weights []float64, u float64) [2]float64 {
	n := len(controlPoints)
	// find the knot span k with knots[k] <= u < knots[k+1], clamped to the valid range
	k := degree
	for k < n-1 && u >= knots[k+1] {
		k++
	}

	d := make([][3]float64, degree+1)
	for j := 0; j <= degree; j++ {
		i := j + k - degree
		w := weights[i]
		d[j] = [3]float64{controlPoints[i][0] * w, controlPoints[i][1] * w, w}
	}
	for r := 1; r <= degree; r++ {
		for j := degree; j >= r; j-- {
			i := j + k - degree
			denominator := knots[i+degree-r+1] - knots[i]
			alpha := 0.0
			if denominator != 0 {
				alpha = (u - knots[i]) / denominator
			}
			for c := 0; c < 3; c++ {
				d[j][c] = (1.0-alpha)*d[j-1][c] + alpha*d[j][c]
			}
		}
	}

	w := d[degree][2]
	if w == 0 {
		return [2]float64{d[degree][0], d[degree][1]}
	}
	return [2]float64{d[degree][0] / w, d[degree][1] / w}
}

// arcPoints returns the points after the start point of a circular arc; sweep is signed (radians, positive is
// counter-clockwise).
func arcPoints(center [2]float64, radius, startAngle, sweep, tolerance float64) [][2]float64 {
	count := arcSegmentCount(radius, sweep, tolerance)
	points := make([][2]float64, 0, count)
	for i := 1; i <= count; i++ {
		points = append(points, pointOnCircle(center, radius, startAngle+sweep*float64(i)/float64(count)))
	}
	return points
}

// arcSegmentCount returns how many chords keep an arc within tolerance (the chord's sagitta).
func arcSegmentCount(radius, sweep, tolerance float64) int {
	sweep = math.Abs(sweep)
	if radius <= 0 || sweep == 0 {
		return 1
	}
	if tolerance <= 0 {
		tolerance = relativeFlattenTolerance * radius
	}

	maxStep := math.Pi / 2.0
	if tolerance < radius {
		maxStep = math.Min(maxStep, 2.0*math.Acos(1.0-tolerance/radius))
	}
	count := int(math.Ceil(sweep / maxStep))
	return max(1, min(count, maxCurveSegments))
}

func pointOnCircle(center [2]float64, radius, angle float64) [2]float64 {
	sin, cos := math.Sincos(angle)
	return [2]float64{center[0] + radius*cos, center[1] + radius*sin}
}

func distanceToSegment(p, a, b [2]float64) float64 {
	dx, dy := b[0]-a[0], b[1]-a[1]
	lengthSquared := dx*dx + dy*dy
	if lengthSquared == 0 {
		return math.Hypot(p[0]-a[0], p[1]-a[1])
	}
	t := math.Max(0, math.Min(1, ((p[0]-a[0])*dx+(p[1]-a[1])*dy)/lengthSquared))
	return math.Hypot(p[0]-(a[0]+t*dx), p[1]-(a[1]+t*dy))
}

func boundingDiagonal(points [][2]float64) float64 {
	if len(points) == 0 {
		return 0
	}
	minX, minY, maxX, maxY := points[0][0], points[0][1], points[0][0], points[0][1]
	for _, p := range points[1:] {
		minX, maxX = math.Min(minX, p[0]), math.Max(maxX, p[0])
		minY, maxY = math.Min(minY, p[1]), math.Max(maxY, p[1])
	}
	return math.Hypot(maxX-minX, maxY-minY)
}

func nearlyEqual2(a, b [2]float64) bool {
	scale := math.Max(1.0, math.Max(math.Abs(a[0]), math.Abs(a[1])))
	return math.Abs(a[0]-b[0]) <= 1e-9*scale && math.Abs(a[1]-b[1]) <= 1e-9*scale
}

func reversePoints(points [][2]float64) {
	for i, j := 0, len(points)-1; i < j; i, j = i+1, j-1 {
		points[i], points[j] = points[j], points[i]
	}
}

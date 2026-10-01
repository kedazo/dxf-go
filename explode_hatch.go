package dxf

import (
	"math"
)

// hatchPlane maps hatch coordinates (2D, in the hatch's object coordinate system) through a plane transformation.
type hatchPlane struct {
	plane     planeTransform
	elevation float64
	scale     float64 // uniform scale, if similar
	similar   bool
	mirrored  bool
}

func (h hatchPlane) point(p [2]float64) [2]float64 {
	q := h.plane.point(Point{p[0], p[1], h.elevation})
	return [2]float64{q.X, q.Y}
}

func (h hatchPlane) vector(v [2]float64) [2]float64 {
	x, y := h.plane.vector2(v[0], v[1])
	return [2]float64{x, y}
}

// transformHatch transforms a hatch exactly: boundary paths and edges follow the transformation (arcs become
// ellipse edges when scaled non-uniformly) and the pattern lines are mapped as line families. Associativity is
// dropped because the copy has no boundary objects.
func (t *entityTransformer) transformHatch(hatch *Hatch) []Entity {
	plane, ok := t.plane(hatch.ExtrusionDirection, hatch)
	if !ok {
		return nil
	}
	scale, similar := plane.similarityScale()
	h := hatchPlane{plane: plane, elevation: hatch.Elevation(), scale: scale, similar: similar, mirrored: plane.determinant() < 0}

	c := CloneEntity(hatch).(*Hatch)
	c.IsAssociative = false
	for i := range c.Paths {
		c.Paths[i] = h.transformPath(hatch.Paths[i])
	}
	for i, seed := range hatch.SeedPoints {
		c.SeedPoints[i] = h.point(seed)
	}

	for i, line := range hatch.PatternLines {
		sin, cos := math.Sincos(line.Angle * math.Pi / 180)
		direction := h.vector([2]float64{cos, sin})
		dashScale := math.Hypot(direction[0], direction[1])
		base := h.point([2]float64{line.BaseX, line.BaseY})
		offset := h.vector([2]float64{line.OffsetX, line.OffsetY})
		c.PatternLines[i].Angle = normalizeDegrees(math.Atan2(direction[1], direction[0]) * 180 / math.Pi)
		c.PatternLines[i].BaseX, c.PatternLines[i].BaseY = base[0], base[1]
		c.PatternLines[i].OffsetX, c.PatternLines[i].OffsetY = offset[0], offset[1]
		for j := range line.Dashes {
			c.PatternLines[i].Dashes[j] = line.Dashes[j] * dashScale
		}
	}
	if similar {
		c.PatternAngle = plane.angle(hatch.PatternAngle)
		c.PatternScale = hatch.PatternScale * scale
		c.PixelSize = hatch.PixelSize * scale
	} else {
		c.PixelSize = hatch.PixelSize * math.Sqrt(math.Abs(plane.determinant()))
		if !hatch.SolidFill {
			t.report(IssueApproximated, hatch, "the hatch pattern is distorted: its pattern lines are exact, but PatternAngle and PatternScale can't describe them")
		}
	}
	if hatch.Gradient != nil {
		c.Gradient.Angle = plane.angle(hatch.Gradient.Angle*180/math.Pi) * math.Pi / 180
	}

	c.SetElevation(plane.point(Point{0, 0, hatch.Elevation()}).Z)
	c.ExtrusionDirection = plane.normal
	return []Entity{c}
}

func (h hatchPlane) transformPath(path HatchBoundaryPath) HatchBoundaryPath {
	result := HatchBoundaryPath{PathType: path.PathType, IsClosed: path.IsClosed}
	if path.IsPolyline() {
		hasBulges := false
		for _, bulge := range path.Bulges {
			hasBulges = hasBulges || bulge != 0
		}
		if !h.similar && hasBulges {
			// the arcs become elliptical: store the path as edges
			result.PathType &^= 2
			result.IsClosed = false
			result.Edges = h.bulgedPathEdges(path)
			return result
		}

		for _, vertex := range path.Vertices {
			result.Vertices = append(result.Vertices, h.point(vertex))
		}
		for _, bulge := range path.Bulges {
			if h.mirrored {
				bulge = -bulge
			}
			result.Bulges = append(result.Bulges, bulge)
		}
		return result
	}

	for _, edge := range path.Edges {
		result.Edges = append(result.Edges, h.transformEdge(edge))
	}
	return result
}

func (h hatchPlane) bulgedPathEdges(path HatchBoundaryPath) (edges []HatchEdge) {
	segmentCount := len(path.Vertices)
	if segmentCount < 2 {
		return nil
	}
	for i := 0; i < segmentCount; i++ {
		// hatch boundaries are always closed
		start, end := path.Vertices[i], path.Vertices[(i+1)%segmentCount]
		bulge := 0.0
		if i < len(path.Bulges) {
			bulge = path.Bulges[i]
		}
		center, radius, startAngle, sweep, isArc := bulgeArc(start, end, bulge)
		if !isArc {
			edges = append(edges, &HatchLineEdge{Start: h.point(start), End: h.point(end)})
			continue
		}
		from, isCounterClockwise := startAngle, true
		if sweep < 0 {
			from, sweep, isCounterClockwise = startAngle+sweep, -sweep, false
		}
		edges = append(edges, h.ellipseEdge(center, [2]float64{radius, 0}, [2]float64{0, radius}, from, sweep, isCounterClockwise))
	}
	return
}

func (h hatchPlane) transformEdge(edge HatchEdge) HatchEdge {
	switch edge := edge.(type) {
	case *HatchLineEdge:
		return &HatchLineEdge{Start: h.point(edge.Start), End: h.point(edge.End)}
	case *HatchArcEdge:
		start, sweep := counterClockwiseAngles(edge.StartAngle, edge.EndAngle, edge.IsCounterClockwise)
		if h.similar {
			isCounterClockwise := edge.IsCounterClockwise
			newStart := h.plane.angle(start)
			if h.mirrored {
				// the counter-clockwise range runs the other way after mirroring
				newStart = h.plane.angle(start + sweep)
				isCounterClockwise = !isCounterClockwise
			}
			result := &HatchArcEdge{Center: h.point(edge.Center), Radius: edge.Radius * h.scale, IsCounterClockwise: isCounterClockwise}
			result.StartAngle, result.EndAngle = storedEdgeAngles(newStart, sweep, isCounterClockwise)
			return result
		}
		return h.ellipseEdge(edge.Center, [2]float64{edge.Radius, 0}, [2]float64{0, edge.Radius}, start*math.Pi/180, sweep*math.Pi/180, edge.IsCounterClockwise)
	case *HatchEllipseEdge:
		start, sweep := counterClockwiseAngles(edge.StartAngle, edge.EndAngle, edge.IsCounterClockwise)
		startParameter := ellipseAngleToParameter(edge.MinorAxisRatio, start*math.Pi/180)
		endParameter := ellipseAngleToParameter(edge.MinorAxisRatio, (start+sweep)*math.Pi/180)
		parameterSweep := math.Mod(endParameter-startParameter+4*math.Pi, 2*math.Pi)
		if sweep >= 360 || (parameterSweep == 0 && sweep > 0) {
			parameterSweep = 2 * math.Pi
		}
		minor := [2]float64{-edge.MajorAxis[1] * edge.MinorAxisRatio, edge.MajorAxis[0] * edge.MinorAxisRatio}
		return h.ellipseEdge(edge.Center, edge.MajorAxis, minor, startParameter, parameterSweep, edge.IsCounterClockwise)
	case *HatchSplineEdge:
		result := *edge
		result.Knots = cloneSlice(edge.Knots)
		result.Weights = cloneSlice(edge.Weights)
		result.ControlPoints = make([][2]float64, len(edge.ControlPoints))
		for i, point := range edge.ControlPoints {
			result.ControlPoints[i] = h.point(point)
		}
		result.FitPoints = nil
		for _, point := range edge.FitPoints {
			result.FitPoints = append(result.FitPoints, h.point(point))
		}
		result.StartTangent = h.vector(edge.StartTangent)
		result.EndTangent = h.vector(edge.EndTangent)
		return &result
	}
	return edge
}

// ellipseEdge returns the ellipse edge for the transformed curve center + cos(t)·u + sin(t)·v, t from start over
// sweep (radians, counter-clockwise), traversed counter-clockwise or not.
func (h hatchPlane) ellipseEdge(center, u, v [2]float64, start, sweep float64, isCounterClockwise bool) *HatchEllipseEdge {
	u, v = h.vector(u), h.vector(v)
	t0 := 0.5 * math.Atan2(2*(u[0]*v[0]+u[1]*v[1]), u[0]*u[0]+u[1]*u[1]-v[0]*v[0]-v[1]*v[1])
	sin0, cos0 := math.Sincos(t0)
	major := [2]float64{cos0*u[0] + sin0*v[0], cos0*u[1] + sin0*v[1]}
	minor := [2]float64{-sin0*u[0] + cos0*v[0], -sin0*u[1] + cos0*v[1]}

	parameterStart := start - t0
	if major[0]*minor[1]-major[1]*minor[0] < 0 {
		// mirrored: with the minor axis flipped, the parameters run backwards
		parameterStart = -(parameterStart + sweep)
		isCounterClockwise = !isCounterClockwise
	}
	majorLength := math.Hypot(major[0], major[1])
	ratio := 0.0
	if majorLength > 0 {
		ratio = math.Hypot(minor[0], minor[1]) / majorLength
	}

	// edge angles are real angles around the center, not parameters
	startAngle := ellipseParameterToAngle(ratio, parameterStart)
	angleSweep := normalizeDegrees(ellipseParameterToAngle(ratio, parameterStart+sweep) - startAngle)
	if sweep >= 2*math.Pi-transformEpsilon || angleSweep == 0 {
		angleSweep = 360
	}
	result := &HatchEllipseEdge{Center: h.point(center), MajorAxis: major, MinorAxisRatio: ratio, IsCounterClockwise: isCounterClockwise}
	result.StartAngle, result.EndAngle = storedEdgeAngles(startAngle, angleSweep, isCounterClockwise)
	return result
}

// ellipseParameterToAngle converts an ellipse parameter (radians) into the real angle (degrees) around the center.
func ellipseParameterToAngle(ratio, parameter float64) float64 {
	sin, cos := math.Sincos(parameter)
	return normalizeDegrees(math.Atan2(ratio*sin, cos) * 180 / math.Pi)
}

// storedEdgeAngles returns the angles to store for an edge covering the counter-clockwise range from start over
// sweep (degrees): clockwise edges are stored mirrored (360 - angle) with start and end swapped.
func storedEdgeAngles(start, sweep float64, isCounterClockwise bool) (float64, float64) {
	from := normalizeDegrees(start)
	if !isCounterClockwise {
		from = normalizeDegrees(360 - (start + sweep))
	}
	if sweep >= 360 {
		return from, from + 360
	}
	return from, normalizeDegrees(from + sweep)
}

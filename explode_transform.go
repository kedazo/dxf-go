package dxf

import (
	"math"
	"strings"
)

const transformEpsilon = 1e-9

// TransformEntity returns copies of e transformed by m. Entities keep their type when that is exact; when it isn't
// (e.g. circles under non-uniform scaling) they are converted, and what can't be represented is reported as issues.
func TransformEntity(e Entity, m Matrix, options ExplodeOptions) ([]Entity, []WalkIssue) {
	transformer := entityTransformer{m: m, options: options}
	return transformer.transform(e), transformer.issues
}

type entityTransformer struct {
	m       Matrix
	options ExplodeOptions
	issues  []WalkIssue
}

func (t *entityTransformer) report(kind IssueKind, e Entity, message string) {
	t.issues = append(t.issues, WalkIssue{Kind: kind, Entity: e, Message: message})
}

func (t *entityTransformer) transform(e Entity) []Entity {
	if t.m == IdentityMatrix() {
		return []Entity{CloneEntity(e)}
	}

	switch ent := e.(type) {
	case *Line:
		c := CloneEntity(ent).(*Line)
		c.P1 = t.m.TransformPoint(ent.P1)
		c.P2 = t.m.TransformPoint(ent.P2)
		c.ExtrusionDirection, c.Thickness = t.extrusion(ent.ExtrusionDirection, ent.Thickness)
		return []Entity{c}
	case *ModelPoint:
		c := CloneEntity(ent).(*ModelPoint)
		c.Location = t.m.TransformPoint(ent.Location)
		c.ExtrusionDirection, c.Thickness = t.extrusion(ent.ExtrusionDirection, ent.Thickness)
		return []Entity{c}
	case *Face:
		c := CloneEntity(ent).(*Face)
		c.FirstCorner = t.m.TransformPoint(ent.FirstCorner)
		c.SecondCorner = t.m.TransformPoint(ent.SecondCorner)
		c.ThirdCorner = t.m.TransformPoint(ent.ThirdCorner)
		c.FourthCorner = t.m.TransformPoint(ent.FourthCorner)
		return []Entity{c}
	case *Ray:
		c := CloneEntity(ent).(*Ray)
		c.StartPoint = t.m.TransformPoint(ent.StartPoint)
		c.UnitDirectionVector = t.direction(ent.UnitDirectionVector)
		return []Entity{c}
	case *XLine:
		c := CloneEntity(ent).(*XLine)
		c.FirstPoint = t.m.TransformPoint(ent.FirstPoint)
		c.UnitDirectionVector = t.direction(ent.UnitDirectionVector)
		return []Entity{c}
	case *Solid:
		c := CloneEntity(ent).(*Solid)
		if !t.transformCorners(e, &c.FirstCorner, &c.SecondCorner, &c.ThirdCorner, &c.FourthCorner, &c.ExtrusionDirection, &c.Thickness) {
			return nil
		}
		return []Entity{c}
	case *Trace:
		c := CloneEntity(ent).(*Trace)
		if !t.transformCorners(e, &c.FirstCorner, &c.SecondCorner, &c.ThirdCorner, &c.FourthCorner, &c.ExtrusionDirection, &c.Thickness) {
			return nil
		}
		return []Entity{c}
	case *Circle:
		return t.transformCircle(ent)
	case *Arc:
		return t.transformArc(ent)
	case *Ellipse:
		return t.transformEllipse(ent)
	case *LWPolyline:
		return t.transformLWPolyline(ent)
	case *Polyline:
		return t.transformPolyline(ent)
	case *Spline:
		return t.transformSpline(ent)
	case *Text:
		c := CloneEntity(ent).(*Text)
		if !t.transformText(e, textGeometry{&c.Location, &c.SecondAlignmentPoint, &c.Height, &c.Rotation, &c.RelativeXScaleFactor, &c.ObliqueAngle, &c.TextGenerationFlags, &c.Normal, &c.Thickness}) {
			return nil
		}
		return []Entity{c}
	case *Attribute:
		c := CloneEntity(ent).(*Attribute)
		if !t.transformText(e, textGeometry{&c.Location, &c.SecondAlignmentPoint, &c.TextHeight, &c.Rotation, &c.RelativeXScaleFactor, &c.ObliqueAngle, &c.TextGenerationFlags, &c.Normal, &c.Thickness}) {
			return nil
		}
		c.AlignmentPoint = t.m.TransformPoint(ent.AlignmentPoint)
		t.transformMTextInPlace(&c.MText, e)
		return []Entity{c}
	case *AttributeDefinition:
		c := CloneEntity(ent).(*AttributeDefinition)
		if !t.transformText(e, textGeometry{&c.Location, &c.SecondAlignmentPoint, &c.TextHeight, &c.Rotation, &c.RelativeXScaleFactor, &c.ObliqueAngle, &c.TextGenerationFlags, &c.Normal, &c.Thickness}) {
			return nil
		}
		c.AlignmentPoint = t.m.TransformPoint(ent.AlignmentPoint)
		t.transformMTextInPlace(&c.MText, e)
		return []Entity{c}
	case *MText:
		c := CloneEntity(ent).(*MText)
		if !t.transformMTextInPlace(c, e) {
			return nil
		}
		return []Entity{c}
	case *Leader:
		return t.transformLeader(ent)
	case RasterImage:
		c := CloneEntity(e)
		image := c.(RasterImage)
		image.SetLocation(t.m.TransformPoint(ent.Location()))
		image.SetUVector(t.m.TransformVector(ent.UVector()))
		image.SetVVector(t.m.TransformVector(ent.VVector()))
		return []Entity{c}
	}

	t.report(IssueUnsupported, e, strings.ToUpper(e.typeString())+" entities can't be transformed")
	return nil
}

// extrusion transforms an extrusion direction and scales the thickness along it.
func (t *entityTransformer) extrusion(direction Vector, thickness float64) (Vector, float64) {
	transformed := t.m.TransformVector(direction)
	length := transformed.Length()
	if length == 0 {
		return direction, thickness
	}
	return transformed.Scale(1 / length), thickness * length
}

func (t *entityTransformer) direction(direction Vector) Vector {
	transformed := t.m.TransformVector(direction).Normalize()
	if transformed.IsZero(0) {
		return direction
	}
	return transformed
}

// planeTransform maps the object coordinate system of a planar entity to the one it has after the transformation.
type planeTransform struct {
	ocsToNewOCS    Matrix
	normal         Vector
	thicknessScale float64
}

// plane returns the transformation of the plane with the given normal. The new normal keeps the orientation of the
// entity (so arcs stay counter-clockwise); a resulting (0, 0, -1) normal is turned into (0, 0, 1) unless the options
// keep it, in which case curves are mirrored within the plane.
func (t *entityTransformer) plane(normal Vector, e Entity) (planeTransform, bool) {
	if normal.IsZero(0) {
		normal = *NewZAxis()
	}
	xAxis, yAxis := ArbitraryAxis(normal)
	u, v := t.m.TransformVector(xAxis), t.m.TransformVector(yAxis)
	newNormal := u.Cross(v)
	if newNormal.Length() <= transformEpsilon*u.Length()*v.Length() || newNormal.Length() == 0 {
		t.report(IssueDegenerate, e, "the transformation collapses the "+strings.ToUpper(e.typeString())+" plane")
		return planeTransform{}, false
	}
	newNormal = newNormal.Normalize()
	if !t.options.KeepNegativeExtrusion && isNegativeZ(newNormal) {
		newNormal = *NewZAxis()
	}

	transformedNormal := t.m.TransformVector(normal.Normalize())
	return planeTransform{
		ocsToNewOCS:    WCSToOCSMatrix(newNormal).Mul(t.m).Mul(OCSToWCSMatrix(normal)),
		normal:         newNormal,
		thicknessScale: transformedNormal.Dot(newNormal),
	}, true
}

func isNegativeZ(v Vector) bool {
	return v.Z < 0 && math.Abs(v.X) < transformEpsilon && math.Abs(v.Y) < transformEpsilon
}

// point transforms a point given in the old object coordinate system into the new one.
func (p planeTransform) point(q Point) Point {
	return p.ocsToNewOCS.TransformPoint(q)
}

// vector2 transforms an in-plane direction.
func (p planeTransform) vector2(x, y float64) (float64, float64) {
	g := p.ocsToNewOCS
	return g[0][0]*x + g[0][1]*y, g[1][0]*x + g[1][1]*y
}

// determinant is negative when the in-plane mapping mirrors.
func (p planeTransform) determinant() float64 {
	g := p.ocsToNewOCS
	return g[0][0]*g[1][1] - g[0][1]*g[1][0]
}

// similarityScale returns the uniform scale factor if the in-plane mapping preserves shapes.
func (p planeTransform) similarityScale() (float64, bool) {
	g := p.ocsToNewOCS
	x := math.Hypot(g[0][0], g[1][0])
	y := math.Hypot(g[0][1], g[1][1])
	dot := g[0][0]*g[0][1] + g[1][0]*g[1][1]
	similar := math.Abs(x-y) <= transformEpsilon*math.Max(x, y) && math.Abs(dot) <= transformEpsilon*x*y
	return x, similar
}

// angle transforms an in-plane angle (degrees).
func (p planeTransform) angle(degrees float64) float64 {
	sin, cos := math.Sincos(degrees * math.Pi / 180)
	x, y := p.vector2(cos, sin)
	return normalizeDegrees(math.Atan2(y, x) * 180 / math.Pi)
}

func (p planeTransform) thickness(thickness float64) float64 {
	return thickness * p.thicknessScale
}

func normalizeDegrees(angle float64) float64 {
	angle = math.Mod(angle, 360)
	if angle < 0 {
		angle += 360
	}
	return angle
}

// arcSweep returns the counter-clockwise sweep from start to end in degrees; equal angles are a full circle.
func arcSweep(start, end float64) float64 {
	sweep := normalizeDegrees(end - start)
	if sweep == 0 {
		sweep = 360
	}
	return sweep
}

func (t *entityTransformer) transformCorners(e Entity, first, second, third, fourth *Point, extrusion *Vector, thickness *float64) bool {
	plane, ok := t.plane(*extrusion, e)
	if !ok {
		return false
	}
	*first, *second, *third, *fourth = plane.point(*first), plane.point(*second), plane.point(*third), plane.point(*fourth)
	*extrusion = plane.normal
	*thickness = plane.thickness(*thickness)
	return true
}

func (t *entityTransformer) transformCircle(circle *Circle) []Entity {
	plane, ok := t.plane(circle.Normal, circle)
	if !ok {
		return nil
	}
	scale, similar := plane.similarityScale()
	if !similar {
		return t.circularArcAsEllipse(circle, circle.Normal, circle.Center, circle.Radius, 0, 2*math.Pi, circle.Thickness)
	}
	c := CloneEntity(circle).(*Circle)
	c.Center = plane.point(circle.Center)
	c.Radius = circle.Radius * scale
	c.Normal = plane.normal
	c.Thickness = plane.thickness(circle.Thickness)
	return []Entity{c}
}

func (t *entityTransformer) transformArc(arc *Arc) []Entity {
	plane, ok := t.plane(arc.Normal, arc)
	if !ok {
		return nil
	}
	scale, similar := plane.similarityScale()
	if !similar {
		start := arc.StartAngle * math.Pi / 180
		return t.circularArcAsEllipse(arc, arc.Normal, arc.Center, arc.Radius, start, start+arcSweep(arc.StartAngle, arc.EndAngle)*math.Pi/180, arc.Thickness)
	}
	c := CloneEntity(arc).(*Arc)
	c.Center = plane.point(arc.Center)
	c.Radius = arc.Radius * scale
	c.Normal = plane.normal
	c.Thickness = plane.thickness(arc.Thickness)
	sweep := arcSweep(arc.StartAngle, arc.EndAngle)
	if plane.determinant() > 0 {
		c.StartAngle = plane.angle(arc.StartAngle)
	} else {
		// a mirrored arc runs the other way: its new start is the image of the old end
		c.StartAngle = plane.angle(arc.EndAngle)
	}
	c.EndAngle = c.StartAngle + sweep
	if sweep < 360 {
		c.EndAngle = normalizeDegrees(c.EndAngle)
	}
	return []Entity{c}
}

// circularArcAsEllipse converts a circle or arc (given in its object coordinate system, angles in radians) that is
// scaled non-uniformly into the exact ELLIPSE.
func (t *entityTransformer) circularArcAsEllipse(e Entity, normal Vector, center Point, radius, start, end, thickness float64) []Entity {
	geometry, ok := t.circularArcGeometry(normal, center, radius, start, end)
	if !ok {
		t.report(IssueDegenerate, e, "the transformation collapses the "+strings.ToUpper(e.typeString()))
		return nil
	}
	ellipse := NewEllipse()
	copyEntityProperties(e, ellipse)
	geometry.apply(ellipse)
	if thickness != 0 {
		t.report(IssueApproximated, e, "an ELLIPSE has no thickness")
	}
	return []Entity{ellipse}
}

// circularArcGeometry returns the transformed ellipse of an arc in an object coordinate system.
func (t *entityTransformer) circularArcGeometry(normal Vector, center Point, radius, start, end float64) (ellipseGeometry, bool) {
	if normal.IsZero(0) {
		normal = *NewZAxis()
	}
	toWCS := OCSToWCSMatrix(normal)
	u := t.m.TransformVector(toWCS.TransformVector(Vector{radius, 0, 0}))
	v := t.m.TransformVector(toWCS.TransformVector(Vector{0, radius, 0}))
	return t.ellipseFromConjugateAxes(t.m.TransformPoint(toWCS.TransformPoint(center)), u, v, start, end)
}

// ellipseGeometry is an ellipse in world coordinates; angles are ellipse parameters in radians.
type ellipseGeometry struct {
	center     Point
	majorAxis  Vector
	normal     Vector
	ratio      float64
	startAngle float64
	endAngle   float64
}

// ellipseFromConjugateAxes finds the principal axes of the ellipse center + cos(t)·u + sin(t)·v for t from start to
// end (radians).
func (t *entityTransformer) ellipseFromConjugateAxes(center Point, u, v Vector, start, end float64) (ellipseGeometry, bool) {
	t0 := 0.5 * math.Atan2(2*u.Dot(v), u.Dot(u)-v.Dot(v))
	sin0, cos0 := math.Sincos(t0)
	major := u.Scale(cos0).Add(v.Scale(sin0))
	minor := u.Scale(-sin0).Add(v.Scale(cos0))
	normal := major.Cross(minor)
	if major.Length() == 0 || normal.Length() <= transformEpsilon*major.Length()*minor.Length() || normal.Length() == 0 {
		return ellipseGeometry{}, false
	}

	sweep := math.Mod(end-start, 2*math.Pi)
	if sweep < 0 {
		sweep += 2 * math.Pi
	}
	full := sweep <= transformEpsilon || sweep >= 2*math.Pi-transformEpsilon
	if full {
		sweep = 2 * math.Pi
	}
	start -= t0
	normal = normal.Normalize()
	if !t.options.KeepNegativeExtrusion && isNegativeZ(normal) {
		// with the opposite normal the minor axis points the other way and the parameters run backwards
		normal = *NewZAxis()
		start = -(start + sweep)
	}
	if full {
		start = 0
	}
	start = math.Mod(start, 2*math.Pi)
	if start < 0 {
		start += 2 * math.Pi
	}

	return ellipseGeometry{
		center:     center,
		majorAxis:  major,
		normal:     normal,
		ratio:      minor.Length() / major.Length(),
		startAngle: start,
		endAngle:   start + sweep,
	}, true
}

func (g ellipseGeometry) apply(ellipse *Ellipse) {
	ellipse.Center = g.center
	ellipse.MajorAxis = g.majorAxis
	ellipse.Normal = g.normal
	ellipse.MinorAxisRatio = g.ratio
	ellipse.StartAngle = g.startAngle
	ellipse.EndAngle = g.endAngle
}

func (t *entityTransformer) transformEllipse(ellipse *Ellipse) []Entity {
	normal := ellipse.Normal.Normalize()
	if normal.IsZero(0) {
		normal = *NewZAxis()
	}
	minor := normal.Cross(ellipse.MajorAxis).Scale(ellipse.MinorAxisRatio)
	geometry, ok := t.ellipseFromConjugateAxes(t.m.TransformPoint(ellipse.Center), t.m.TransformVector(ellipse.MajorAxis), t.m.TransformVector(minor), ellipse.StartAngle, ellipse.EndAngle)
	if !ok {
		t.report(IssueDegenerate, ellipse, "the transformation collapses the ELLIPSE")
		return nil
	}
	c := CloneEntity(ellipse).(*Ellipse)
	geometry.apply(c)
	return []Entity{c}
}

func (t *entityTransformer) transformLWPolyline(polyline *LWPolyline) []Entity {
	plane, ok := t.plane(polyline.ExtrusionDirection, polyline)
	if !ok {
		return nil
	}
	scale, similar := plane.similarityScale()
	if !similar && lwPolylineHasBulges(polyline) {
		vertices := make([]bulgeVertex, len(polyline.Vertices))
		hasWidths := polyline.ConstantWidth != 0
		for i, vertex := range polyline.Vertices {
			vertices[i] = bulgeVertex{vertex.X, vertex.Y, vertex.Bulge}
			hasWidths = hasWidths || vertex.StartingWidth != 0 || vertex.EndingWidth != 0
		}
		return t.splitBulgedPolyline(polyline, polyline.ExtrusionDirection, polyline.Elevation(), vertices, polyline.IsClosed(), hasWidths, polyline.Thickness)
	}

	widthScale := t.widthScale(plane, scale, similar)
	hasWidths := polyline.ConstantWidth != 0
	mirrored := plane.determinant() < 0
	c := CloneEntity(polyline).(*LWPolyline)
	elevation := polyline.Elevation()
	for i, vertex := range polyline.Vertices {
		p := plane.point(Point{vertex.X, vertex.Y, elevation})
		c.Vertices[i].X, c.Vertices[i].Y = p.X, p.Y
		if mirrored {
			c.Vertices[i].Bulge = -vertex.Bulge
		}
		c.Vertices[i].StartingWidth *= widthScale
		c.Vertices[i].EndingWidth *= widthScale
		hasWidths = hasWidths || vertex.StartingWidth != 0 || vertex.EndingWidth != 0
	}
	c.ConstantWidth *= widthScale
	c.SetElevation(plane.point(Point{0, 0, elevation}).Z)
	c.ExtrusionDirection = plane.normal
	c.Thickness = plane.thickness(polyline.Thickness)
	if hasWidths && !similar {
		t.report(IssueApproximated, polyline, "LWPOLYLINE widths can't be scaled non-uniformly")
	}
	return []Entity{c}
}

// widthScale returns how polyline widths scale; under non-uniform scaling they get the average scale.
func (t *entityTransformer) widthScale(plane planeTransform, scale float64, similar bool) float64 {
	if similar {
		return scale
	}
	return math.Sqrt(math.Abs(plane.determinant()))
}

func lwPolylineHasBulges(polyline *LWPolyline) bool {
	for _, vertex := range polyline.Vertices {
		if vertex.Bulge != 0 {
			return true
		}
	}
	return false
}

func polylineHasBulges(polyline *Polyline) bool {
	for _, vertex := range polyline.Vertices {
		if vertex.Bulge != 0 {
			return true
		}
	}
	return false
}

type bulgeVertex struct {
	x, y, bulge float64
}

// splitBulgedPolyline converts a polyline whose arcs become elliptical into LINE and ELLIPSE entities.
func (t *entityTransformer) splitBulgedPolyline(e Entity, normal Vector, elevation float64, vertices []bulgeVertex, isClosed, hasWidths bool, thickness float64) (result []Entity) {
	if normal.IsZero(0) {
		normal = *NewZAxis()
	}
	toWCS := OCSToWCSMatrix(normal)
	world := func(v bulgeVertex) Point {
		return t.m.TransformPoint(toWCS.TransformPoint(Point{v.x, v.y, elevation}))
	}

	segmentCount := len(vertices) - 1
	if isClosed {
		segmentCount = len(vertices)
	}
	for i := 0; i < segmentCount; i++ {
		start, end := vertices[i], vertices[(i+1)%len(vertices)]
		center, radius, startAngle, sweep, isArc := bulgeArc([2]float64{start.x, start.y}, [2]float64{end.x, end.y}, start.bulge)
		if !isArc {
			line := NewLine()
			copyEntityProperties(e, line)
			line.P1, line.P2 = world(start), world(end)
			result = append(result, line)
			continue
		}

		// ellipses always run counter-clockwise
		from, to := startAngle, startAngle+sweep
		if sweep < 0 {
			from, to = to, from
		}
		geometry, ok := t.circularArcGeometry(normal, Point{center[0], center[1], elevation}, radius, from, to)
		if !ok {
			t.report(IssueDegenerate, e, "the transformation collapses a "+strings.ToUpper(e.typeString())+" arc")
			continue
		}
		ellipse := NewEllipse()
		copyEntityProperties(e, ellipse)
		geometry.apply(ellipse)
		result = append(result, ellipse)
	}

	if hasWidths || thickness != 0 {
		t.report(IssueApproximated, e, strings.ToUpper(e.typeString())+" arcs became ellipses; widths and thickness were dropped")
	}
	return
}

func (t *entityTransformer) transformPolyline(polyline *Polyline) []Entity {
	c := CloneEntity(polyline).(*Polyline)
	if polyline.Is3DPolyline() || polyline.Is3DPolygonMesh() || polyline.IsPolyfaceMesh() {
		// vertices are in world coordinates
		for i, vertex := range polyline.Vertices {
			if vertex.IsPolyfaceMeshVertex() && !vertex.Is3DPolygonMesh() {
				// polyface face records only hold indices
				continue
			}
			c.Vertices[i].Location = t.m.TransformPoint(vertex.Location)
		}
		return []Entity{c}
	}

	plane, ok := t.plane(polyline.Normal, polyline)
	if !ok {
		return nil
	}
	scale, similar := plane.similarityScale()
	if !similar && polylineHasBulges(polyline) {
		vertices := make([]bulgeVertex, len(polyline.Vertices))
		hasWidths := polyline.DefaultStartingWidth != 0 || polyline.DefaultEndingWidth != 0
		for i, vertex := range polyline.Vertices {
			vertices[i] = bulgeVertex{vertex.Location.X, vertex.Location.Y, vertex.Bulge}
			hasWidths = hasWidths || vertex.StartingWidth != 0 || vertex.EndingWidth != 0
		}
		return t.splitBulgedPolyline(polyline, polyline.Normal, polyline.Location.Z, vertices, polyline.IsClosed(), hasWidths, polyline.Thickness)
	}

	widthScale := t.widthScale(plane, scale, similar)
	hasWidths := polyline.DefaultStartingWidth != 0 || polyline.DefaultEndingWidth != 0
	mirrored := plane.determinant() < 0
	elevation := polyline.Location.Z
	for i, vertex := range polyline.Vertices {
		p := plane.point(Point{vertex.Location.X, vertex.Location.Y, elevation})
		c.Vertices[i].Location = p
		if mirrored {
			c.Vertices[i].Bulge = -vertex.Bulge
		}
		c.Vertices[i].StartingWidth *= widthScale
		c.Vertices[i].EndingWidth *= widthScale
		hasWidths = hasWidths || vertex.StartingWidth != 0 || vertex.EndingWidth != 0
	}
	c.DefaultStartingWidth *= widthScale
	c.DefaultEndingWidth *= widthScale
	c.Location = Point{0, 0, plane.point(Point{0, 0, elevation}).Z}
	c.Normal = plane.normal
	c.Thickness = plane.thickness(polyline.Thickness)
	if hasWidths && !similar {
		t.report(IssueApproximated, polyline, "POLYLINE widths can't be scaled non-uniformly")
	}
	return []Entity{c}
}

func (t *entityTransformer) transformSpline(spline *Spline) []Entity {
	c := CloneEntity(spline).(*Spline)
	// NURBS are invariant under affine transformations: transforming the control points is exact
	for i := range c.ControlPoints {
		c.ControlPoints[i].Point = t.m.TransformPoint(spline.ControlPoints[i].Point)
	}
	for i := range c.FitPoints {
		c.FitPoints[i] = t.m.TransformPoint(spline.FitPoints[i])
	}
	if !spline.StartTangent.IsZero(0) {
		c.StartTangent = t.direction(spline.StartTangent)
	}
	if !spline.EndTangent.IsZero(0) {
		c.EndTangent = t.direction(spline.EndTangent)
	}
	if !spline.Normal.IsZero(0) {
		if normal, ok := t.m.TransformNormal(spline.Normal); ok {
			c.Normal = normal
		}
	}
	return []Entity{c}
}

func (t *entityTransformer) transformLeader(leader *Leader) []Entity {
	c := CloneEntity(leader).(*Leader)
	for i, vertex := range leader.Vertices {
		c.Vertices[i] = t.m.TransformPoint(vertex)
	}
	if normal, ok := t.m.TransformNormal(leader.Normal); ok && !leader.Normal.IsZero(0) {
		c.Normal = normal
	}
	c.Right = t.direction(leader.Right)
	c.BlockOffset = t.m.TransformVector(leader.BlockOffset)
	c.AnnotationOffset = t.m.TransformVector(leader.AnnotationOffset)
	if math.Abs(math.Abs(t.m.Determinant3())-1) > transformEpsilon {
		t.report(IssueApproximated, leader, "LEADER arrowheads keep the size of their dimension style")
	}
	return []Entity{c}
}

// textGeometry points to the fields of TEXT, ATTRIB and ATTDEF that describe their placement.
type textGeometry struct {
	location             *Point
	secondAlignmentPoint *Point
	height               *float64
	rotation             *float64 // degrees
	widthFactor          *float64
	obliqueAngle         *float64 // degrees
	generationFlags      *int     // 2 = backward, 4 = upside down
	normal               *Vector
	thickness            *float64
}

const (
	textBackward   = 2
	textUpsideDown = 4
)

// transformText transforms single-line text exactly: any non-degenerate in-plane mapping is a rotation, a height,
// a width factor, an oblique angle and possibly mirroring (the backward flag).
func (t *entityTransformer) transformText(e Entity, g textGeometry) bool {
	plane, ok := t.plane(*g.normal, e)
	if !ok {
		return false
	}

	rotation := *g.rotation * math.Pi / 180
	oblique := *g.obliqueAngle * math.Pi / 180
	height := *g.height
	widthFactor := *g.widthFactor
	if widthFactor == 0 {
		widthFactor = 1
	}

	// the glyph frame: baseline direction scaled by the width and the slanted up direction scaled by the height
	sin, cos := math.Sincos(rotation)
	baselineX, baselineY := cos*widthFactor*height, sin*widthFactor*height
	upX, upY := (-sin+math.Tan(oblique)*cos)*height, (cos+math.Tan(oblique)*sin)*height
	if *g.generationFlags&textBackward != 0 {
		baselineX, baselineY = -baselineX, -baselineY
	}
	if *g.generationFlags&textUpsideDown != 0 {
		upX, upY = -upX, -upY
	}
	baselineX, baselineY = plane.vector2(baselineX, baselineY)
	upX, upY = plane.vector2(upX, upY)

	baselineLength := math.Hypot(baselineX, baselineY)
	if baselineLength == 0 {
		t.report(IssueDegenerate, e, "the transformation collapses the text")
		return false
	}
	directionX, directionY := baselineX/baselineLength, baselineY/baselineLength
	perpendicularHeight := directionX*upY - directionY*upX
	if math.Abs(perpendicularHeight) <= transformEpsilon*baselineLength {
		t.report(IssueDegenerate, e, "the transformation collapses the text")
		return false
	}
	slant := (upX*directionX + upY*directionY) / math.Abs(perpendicularHeight)

	flags := *g.generationFlags &^ (textBackward | textUpsideDown)
	if perpendicularHeight < 0 {
		// mirrored text: the backward flag mirrors the glyphs along a baseline that points the other way
		flags |= textBackward
		directionX, directionY = -directionX, -directionY
		slant = -slant
	}

	*g.location = plane.point(*g.location)
	*g.secondAlignmentPoint = plane.point(*g.secondAlignmentPoint)
	*g.height = math.Abs(perpendicularHeight)
	*g.widthFactor = baselineLength / *g.height
	*g.rotation = normalizeDegrees(math.Atan2(directionY, directionX) * 180 / math.Pi)
	*g.obliqueAngle = math.Atan(slant) * 180 / math.Pi
	*g.generationFlags = flags
	*g.normal = plane.normal
	*g.thickness = plane.thickness(*g.thickness)
	return true
}

// transformMTextInPlace transforms MTEXT. Its frame can only rotate and scale uniformly, so other transformations
// are approximated (and reported); mirroring is kept by viewing the text from the back (a flipped normal).
func (t *entityTransformer) transformMTextInPlace(mtext *MText, e Entity) bool {
	normal := mtext.ExtrusionDirection.Normalize()
	if normal.IsZero(0) {
		normal = *NewZAxis()
	}
	xAxis := mtext.XAxisDirection.Normalize()
	if xAxis.IsZero(0) {
		sin, cos := math.Sincos(mtext.RotationAngle)
		xAxis = OCSToWCSMatrix(normal).TransformVector(Vector{cos, sin, 0})
	}
	yAxis := normal.Cross(xAxis)

	newX := t.m.TransformVector(xAxis)
	newY := t.m.TransformVector(yAxis)
	newNormal := newX.Cross(newY)
	if newX.Length() == 0 || newNormal.Length() <= transformEpsilon*newX.Length()*newY.Length() || newNormal.Length() == 0 {
		t.report(IssueDegenerate, e, "the transformation collapses the MTEXT")
		return false
	}
	newNormal = newNormal.Normalize()
	widthScale := newX.Length()
	heightScale := newNormal.Cross(newX.Normalize()).Dot(newY)

	newXDirection := newX.Normalize()
	ocsX := WCSToOCSMatrix(newNormal).TransformVector(newXDirection)
	mtext.InsertionPoint = t.m.TransformPoint(mtext.InsertionPoint)
	mtext.ExtrusionDirection = newNormal
	mtext.XAxisDirection = newXDirection
	mtext.RotationAngle = math.Atan2(ocsX.Y, ocsX.X)
	mtext.InitialTextHeight *= heightScale
	mtext.VerticalHeight *= heightScale
	mtext.ReferenceRectangleWidth *= widthScale
	mtext.HorizontalWidth *= widthScale
	mtext.ColumnWidth *= widthScale
	mtext.ColumnGutter *= widthScale
	for i := range mtext.ColumnHeights {
		mtext.ColumnHeights[i] *= heightScale
	}

	similar := math.Abs(widthScale-heightScale) <= transformEpsilon*widthScale && math.Abs(newX.Normalize().Dot(newY)) <= transformEpsilon*newY.Length()
	if !similar {
		t.report(IssueApproximated, e, "MTEXT can't be scaled non-uniformly or slanted; its height and width were scaled separately")
	} else if math.Abs(heightScale-1) > transformEpsilon && strings.Contains(mtext.FormattedText(), `\H`) {
		t.report(IssueApproximated, e, "MTEXT inline heights (\\H) are not scaled")
	}
	return true
}

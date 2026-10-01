package dxf

import (
	"fmt"
	"math"
	"testing"
)

const exactness = 1e-9

// similarityTransforms keep shapes (they may rotate, scale uniformly, mirror and tilt the plane).
var similarityTransforms = map[string]Matrix{
	"rotated":         TranslationMatrix(Vector{5, 6, 7}).Mul(RotationZMatrix(math.Pi / 6)).Mul(ScaleMatrix(2, 2, 2)),
	"mirrored":        ScaleMatrix(-2, 2, 2).Mul(RotationZMatrix(0.2)),
	"tilted":          OCSToWCSMatrix(Vector{1, 1, 1}).Mul(ScaleMatrix(1.5, 1.5, 1.5)),
	"mirrored tilted": OCSToWCSMatrix(Vector{0, 1, 1}).Mul(ScaleMatrix(1, -1, 1)).Mul(TranslationMatrix(Vector{1, 2, 3})),
}

func assertNearPointWithin(t *testing.T, name string, expected, actual Point) {
	t.Helper()
	if expected.Sub(actual).Length() > exactness*math.Max(1, expected.ToVector().Length()) {
		t.Errorf("%s: expected %s, got %s", name, expected.String(), actual.String())
	}
}

func ocsCirclePoint(normal Vector, center Point, radius, angleDegrees float64) Point {
	sin, cos := math.Sincos(angleDegrees * math.Pi / 180)
	return OCSToWCSMatrix(normal).TransformPoint(Point{center.X + radius*cos, center.Y + radius*sin, center.Z})
}

func angleWithin(angle, start, sweep float64) bool {
	offset := normalizeDegrees(angle - start)
	return offset <= sweep+1e-6 || offset >= 360-1e-6
}

// assertOnArc checks that a world point lies on an arc (or a circle when sweep is 360).
func assertOnArc(t *testing.T, name string, p Point, normal Vector, center Point, radius, start, sweep float64) {
	t.Helper()
	q := WCSToOCSMatrix(normal).TransformPoint(p)
	distance := math.Hypot(q.X-center.X, q.Y-center.Y)
	angle := math.Atan2(q.Y-center.Y, q.X-center.X) * 180 / math.Pi
	if math.Abs(distance-radius) > exactness*math.Max(1, radius) || math.Abs(q.Z-center.Z) > exactness*math.Max(1, math.Abs(center.Z)) || !angleWithin(angle, start, sweep) {
		t.Errorf("%s: %s is not on the arc (distance %v of %v, z %v of %v, angle %v)", name, p.String(), distance, radius, q.Z, center.Z, angle)
	}
}

func ellipsePoint(e *Ellipse, parameter float64) Point {
	minor := e.Normal.Normalize().Cross(e.MajorAxis).Scale(e.MinorAxisRatio)
	sin, cos := math.Sincos(parameter)
	return e.Center.Add(e.MajorAxis.Scale(cos)).Add(minor.Scale(sin))
}

// assertOnEllipse checks that a world point lies on the elliptical arc.
func assertOnEllipse(t *testing.T, name string, p Point, e *Ellipse) {
	t.Helper()
	normal := e.Normal.Normalize()
	major := e.MajorAxis
	minor := normal.Cross(major).Scale(e.MinorAxisRatio)
	q := p.Sub(e.Center)
	x := q.Dot(major) / major.Dot(major)
	y := q.Dot(minor) / minor.Dot(minor)
	z := q.Dot(normal)
	parameter := math.Atan2(y, x)
	sweep := e.EndAngle - e.StartAngle
	offset := math.Mod(parameter-e.StartAngle+4*math.Pi, 2*math.Pi)
	if math.Abs(x*x+y*y-1) > 1e-8 || math.Abs(z) > exactness*math.Max(1, major.Length()) || (offset > sweep+1e-8 && offset < 2*math.Pi-1e-8) {
		t.Errorf("%s: %s is not on the ellipse (x²+y² = %v, z = %v, parameter %v)", name, p.String(), x*x+y*y, z, parameter)
	}
}

func transformSingle(t *testing.T, e Entity, m Matrix) Entity {
	t.Helper()
	result, issues := TransformEntity(e, m, ExplodeOptions{})
	if len(issues) > 0 {
		t.Fatalf("unexpected issues: %v", issues)
	}
	if len(result) != 1 {
		t.Fatalf("expected one entity, got %d", len(result))
	}
	return result[0]
}

func TestTransformArcAndCircleExactly(t *testing.T) {
	arc := NewArc()
	arc.Center = Point{1, 2, 3}
	arc.Radius = 2
	arc.StartAngle, arc.EndAngle = 300, 60 // crosses 0°
	arc.Normal = Vector{0, 0, 1}
	tiltedArc := NewArc()
	tiltedArc.Center, tiltedArc.Radius, tiltedArc.StartAngle, tiltedArc.EndAngle = Point{1, 1, 1}, 1, 10, 200
	tiltedArc.Normal = Vector{0.3, -0.4, 0.5}

	for name, m := range similarityTransforms {
		for _, original := range []*Arc{arc, tiltedArc} {
			transformed := transformSingle(t, original, m).(*Arc)
			sweep := arcSweep(original.StartAngle, original.EndAngle)
			assertNearFloat64(t, sweep, arcSweep(transformed.StartAngle, transformed.EndAngle))
			for i := 0; i <= 8; i++ {
				p := m.TransformPoint(ocsCirclePoint(original.Normal, original.Center, original.Radius, original.StartAngle+sweep*float64(i)/8))
				assertOnArc(t, name, p, transformed.Normal, transformed.Center, transformed.Radius, transformed.StartAngle, sweep)
			}
		}

		circle := NewCircle()
		circle.Center, circle.Radius = Point{-1, 0, 2}, 3
		transformed := transformSingle(t, circle, m).(*Circle)
		for i := 0; i < 8; i++ {
			p := m.TransformPoint(ocsCirclePoint(circle.Normal, circle.Center, circle.Radius, float64(i)*45))
			assertOnArc(t, name, p, transformed.Normal, transformed.Center, transformed.Radius, 0, 360)
		}
	}
}

func TestTransformMirroredArcUsesPositiveExtrusion(t *testing.T) {
	arc := NewArc()
	arc.Radius, arc.StartAngle, arc.EndAngle = 1, 0, 90
	mirror := ScaleMatrix(-1, 1, 1)

	transformed := transformSingle(t, arc, mirror).(*Arc)
	assertNearVector(t, Vector{0, 0, 1}, transformed.Normal)
	assertNearFloat64(t, 90, transformed.StartAngle)
	assertNearFloat64(t, 180, transformed.EndAngle)

	kept, _ := TransformEntity(arc, mirror, ExplodeOptions{KeepNegativeExtrusion: true})
	keptArc := kept[0].(*Arc)
	assertNearVector(t, Vector{0, 0, -1}, keptArc.Normal)
	assertNearFloat64(t, 0, keptArc.StartAngle)
	assertNearFloat64(t, 90, keptArc.EndAngle)
}

func TestTransformEllipseExactly(t *testing.T) {
	ellipse := NewEllipse()
	ellipse.Center = Point{1, 2, 0}
	ellipse.MajorAxis = Vector{3, 1, 0}
	ellipse.MinorAxisRatio = 0.4
	ellipse.StartAngle, ellipse.EndAngle = 5.5, 2.0 // wraps around 2π

	transforms := map[string]Matrix{"non-uniform": ScaleMatrix(1, 3, 1).Mul(RotationZMatrix(0.3)), "sheared": {{1, 0.7, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}, {0, 0, 0, 1}}}
	for name, m := range similarityTransforms {
		transforms[name] = m
	}
	for name, m := range transforms {
		transformed := transformSingle(t, ellipse, m).(*Ellipse)
		assert(t, transformed.MinorAxisRatio <= 1, name+": minor axis is longer than the major axis")
		sweep := math.Mod(ellipse.EndAngle-ellipse.StartAngle+2*math.Pi, 2*math.Pi)
		for i := 0; i <= 8; i++ {
			p := m.TransformPoint(ellipsePoint(ellipse, ellipse.StartAngle+sweep*float64(i)/8))
			assertOnEllipse(t, name, p, transformed)
		}
		assertNearPointWithin(t, name+" start", m.TransformPoint(ellipsePoint(ellipse, ellipse.StartAngle)), func() Point {
			// the start point may become the end point when the transformation mirrors
			start, end := ellipsePoint(transformed, transformed.StartAngle), ellipsePoint(transformed, transformed.EndAngle)
			expected := m.TransformPoint(ellipsePoint(ellipse, ellipse.StartAngle))
			if expected.Sub(start).Length() < expected.Sub(end).Length() {
				return start
			}
			return end
		}())
	}
}

func TestTransformLWPolylineWithBulgesExactly(t *testing.T) {
	polyline := NewLWPolyline()
	polyline.SetElevation(2)
	polyline.Vertices = []LwVertex{{X: 0, Y: 0, Bulge: 0.5, StartingWidth: 0.1, EndingWidth: 0.2}, {X: 4, Y: 0, Bulge: -1}, {X: 4, Y: 4}}
	for name, m := range similarityTransforms {
		transformed := transformSingle(t, polyline, m).(*LWPolyline)
		toWCS := OCSToWCSMatrix(transformed.ExtrusionDirection)
		scale := math.Cbrt(math.Abs(m.Determinant3()))
		for i, vertex := range polyline.Vertices {
			expected := m.TransformPoint(OCSToWCSMatrix(polyline.ExtrusionDirection).TransformPoint(Point{vertex.X, vertex.Y, 2}))
			got := transformed.Vertices[i]
			assertNearPointWithin(t, name, expected, toWCS.TransformPoint(Point{got.X, got.Y, transformed.Elevation()}))
			assertNearFloat64(t, math.Abs(vertex.Bulge), math.Abs(got.Bulge))
			assertNearFloat64(t, vertex.StartingWidth*scale, got.StartingWidth)
		}
		// the middle of every bulge arc maps onto the middle of the transformed arc
		for i := 0; i < 2; i++ {
			a, b := polyline.Vertices[i], polyline.Vertices[i+1]
			originalMiddle := flattenBulge([2]float64{a.X, a.Y}, [2]float64{b.X, b.Y}, a.Bulge, 0)
			ta, tb := transformed.Vertices[i], transformed.Vertices[i+1]
			transformedMiddle := flattenBulge([2]float64{ta.X, ta.Y}, [2]float64{tb.X, tb.Y}, ta.Bulge, 0)
			// both flattenings have the same even number of segments, so the middle point is exactly on the arc
			om := originalMiddle[len(originalMiddle)/2-1]
			tm := transformedMiddle[len(transformedMiddle)/2-1]
			expected := m.TransformPoint(OCSToWCSMatrix(polyline.ExtrusionDirection).TransformPoint(Point{om[0], om[1], 2}))
			assertNearPointWithin(t, name+" bulge middle", expected, toWCS.TransformPoint(Point{tm[0], tm[1], transformed.Elevation()}))
		}
	}
}

// textFrame returns the world origin, baseline and up vectors of single-line text glyphs.
func textFrame(location Point, height, rotation, widthFactor, oblique float64, flags int, normal Vector) (Point, Vector, Vector) {
	sin, cos := math.Sincos(rotation * math.Pi / 180)
	tan := math.Tan(oblique * math.Pi / 180)
	baseline := Vector{cos * widthFactor * height, sin * widthFactor * height, 0}
	up := Vector{(-sin + tan*cos) * height, (cos + tan*sin) * height, 0}
	if flags&textBackward != 0 {
		baseline = baseline.Neg()
	}
	if flags&textUpsideDown != 0 {
		up = up.Neg()
	}
	toWCS := OCSToWCSMatrix(normal)
	return toWCS.TransformPoint(location), toWCS.TransformVector(baseline), toWCS.TransformVector(up)
}

func TestTransformTextExactly(t *testing.T) {
	text := NewText()
	text.Location = Point{1, 2, 0.5}
	text.SecondAlignmentPoint = Point{3, 2, 0.5}
	text.Height = 2.5
	text.Rotation = 20
	text.RelativeXScaleFactor = 0.8
	text.ObliqueAngle = 15

	transforms := map[string]Matrix{"non-uniform": ScaleMatrix(1, 3, 1), "sheared": {{1, 0.5, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}, {0, 0, 0, 1}}}
	for name, m := range similarityTransforms {
		transforms[name] = m
	}
	for name, m := range transforms {
		for _, flags := range []int{0, textBackward, textUpsideDown} {
			text.TextGenerationFlags = flags
			transformed := transformSingle(t, text, m).(*Text)
			origin, baseline, up := textFrame(text.Location, text.Height, text.Rotation, text.RelativeXScaleFactor, text.ObliqueAngle, text.TextGenerationFlags, text.Normal)
			newOrigin, newBaseline, newUp := textFrame(transformed.Location, transformed.Height, transformed.Rotation, transformed.RelativeXScaleFactor, transformed.ObliqueAngle, transformed.TextGenerationFlags, transformed.Normal)
			label := fmt.Sprintf("%s (flags %d)", name, flags)
			assertNearPointWithin(t, label+" origin", m.TransformPoint(origin), newOrigin)
			assertNearPointWithin(t, label+" baseline", m.TransformVector(baseline).ToPoint(), newBaseline.ToPoint())
			assertNearPointWithin(t, label+" up", m.TransformVector(up).ToPoint(), newUp.ToPoint())
			assertNearPointWithin(t, label+" second point", m.TransformPoint(OCSToWCSMatrix(text.Normal).TransformPoint(text.SecondAlignmentPoint)),
				OCSToWCSMatrix(transformed.Normal).TransformPoint(transformed.SecondAlignmentPoint))
		}
	}
}

func TestTransformPointsOfLinearEntities(t *testing.T) {
	m := similarityTransforms["mirrored tilted"]

	line := lineEntity(Point{1, 2, 3}, Point{4, 5, 6})
	line.Thickness = 2
	transformedLine := transformSingle(t, line, m).(*Line)
	assertNearPointWithin(t, "line", m.TransformPoint(line.P2), transformedLine.P2)
	assertNearFloat64(t, 2, transformedLine.Thickness)

	spline := NewSpline()
	spline.ControlPoints = []ControlPoint{{Point{0, 0, 0}, 1}, {Point{1, 1, 0}, 0.5}}
	spline.FitPoints = []Point{{2, 2, 2}}
	transformedSpline := transformSingle(t, spline, m).(*Spline)
	assertNearPointWithin(t, "spline", m.TransformPoint(spline.ControlPoints[1].Point), transformedSpline.ControlPoints[1].Point)
	assertNearFloat64(t, 0.5, transformedSpline.ControlPoints[1].Weight)
	assertNearPointWithin(t, "fit point", m.TransformPoint(spline.FitPoints[0]), transformedSpline.FitPoints[0])

	solid := NewSolid()
	solid.FirstCorner, solid.SecondCorner, solid.ThirdCorner, solid.FourthCorner = Point{0, 0, 1}, Point{1, 0, 1}, Point{0, 1, 1}, Point{1, 1, 1}
	transformedSolid := transformSingle(t, solid, m).(*Solid)
	assertNearPointWithin(t, "solid", m.TransformPoint(solid.ThirdCorner), OCSToWCSMatrix(transformedSolid.ExtrusionDirection).TransformPoint(transformedSolid.ThirdCorner))

	polyface := NewPolyline()
	polyface.SetIsPolyfaceMesh(true)
	location := NewVertex()
	location.Location = Point{1, 0, 0}
	location.Flags = 192
	face := NewVertex()
	face.Flags = 128
	face.PolyfaceMeshVertexIndex1 = 1
	polyface.Vertices = []Vertex{*location, *face}
	transformedPolyface := transformSingle(t, polyface, m).(*Polyline)
	assertNearPointWithin(t, "polyface vertex", m.TransformPoint(location.Location), transformedPolyface.Vertices[0].Location)
	assertEqPoint(t, Point{}, transformedPolyface.Vertices[1].Location)
}

func TestTransformMText(t *testing.T) {
	mtext := NewMText()
	mtext.InsertionPoint = Point{1, 1, 0}
	mtext.InitialTextHeight = 2
	mtext.ReferenceRectangleWidth = 10
	mtext.Text = "text"

	transformed := transformSingle(t, mtext, similarityTransforms["rotated"]).(*MText)
	assertNearPointWithin(t, "insertion", similarityTransforms["rotated"].TransformPoint(mtext.InsertionPoint), transformed.InsertionPoint)
	assertNearFloat64(t, 4, transformed.InitialTextHeight)
	assertNearFloat64(t, 20, transformed.ReferenceRectangleWidth)
	assertNearVector(t, Vector{math.Cos(math.Pi / 6), math.Sin(math.Pi / 6), 0}, transformed.XAxisDirection)
	assertNearFloat64(t, math.Pi/6, transformed.RotationAngle)

	// mirrored MTEXT is seen from the back
	mirrored := transformSingle(t, mtext, ScaleMatrix(-1, 1, 1)).(*MText)
	assertNearVector(t, Vector{0, 0, -1}, mirrored.ExtrusionDirection)
	assertNearVector(t, Vector{-1, 0, 0}, mirrored.XAxisDirection)

	// non-uniform scaling is approximated
	result, issues := TransformEntity(mtext, ScaleMatrix(1, 3, 1), ExplodeOptions{})
	assertEqInt(t, 1, len(result))
	assertEqInt(t, 1, len(issues))
	assertEqInt(t, int(IssueApproximated), int(issues[0].Kind))
	assertNearFloat64(t, 6, result[0].(*MText).InitialTextHeight)
}

func TestTransformReportsUnsupportedEntities(t *testing.T) {
	result, issues := TransformEntity(NewRegion(), RotationZMatrix(1), ExplodeOptions{})
	assertEqInt(t, 0, len(result))
	assertEqInt(t, 1, len(issues))
	assertEqInt(t, int(IssueUnsupported), int(issues[0].Kind))

	// the identity keeps everything
	result, issues = TransformEntity(NewRegion(), IdentityMatrix(), ExplodeOptions{})
	assertEqInt(t, 1, len(result))
	assertEqInt(t, 0, len(issues))
}

func explodeDrawing() Drawing {
	drawing := *NewDrawing()
	inner := lineEntity(Point{0, 0, 0}, Point{1, 0, 0})
	inner.SetLayer("0")
	inner.SetColor(ByBlock())
	inner.SetLineWeight(NewLineWeightByBlock())
	constant := NewAttributeDefinition()
	constant.SetIsConstant(true)
	constant.Value = "constant"
	variable := NewAttributeDefinition()
	variable.TextTag = "NAME"
	// BYBLOCK passes through the inner INSERT to the outer one
	innerInsert := insertEntity("INNER", Point{10, 0, 0})
	innerInsert.SetColor(ByBlock())
	innerInsert.SetLineWeight(NewLineWeightByBlock())
	drawing.Blocks = append(drawing.Blocks,
		Block{Name: "INNER", Entities: []Entity{inner}},
		Block{Name: "OUTER", Entities: []Entity{innerInsert, constant, variable}},
	)

	outer := insertEntity("OUTER", Point{100, 0, 0})
	outer.SetLayer("WALLS")
	outer.SetColor(Color(3))
	outer.SetLineWeight(LineWeight(50))
	attribute := NewAttribute()
	attribute.AttributeTag = "NAME"
	attribute.Value = "Konyha"
	attribute.Location = Point{105, 5, 0}
	hidden := NewAttribute()
	hidden.Value = "hidden"
	hidden.SetIsInvisible(true)
	outer.Attributes = []Attribute{*attribute, *hidden}
	drawing.Entities = append(drawing.Entities, outer, lineEntity(Point{}, Point{0, 1, 0}))
	return drawing
}

func TestExplodeRecursively(t *testing.T) {
	drawing := explodeDrawing()
	result := drawing.Explode(ExplodeOptions{Recursive: true, InheritProperties: true})
	assertEqInt(t, 0, len(result.Issues))

	var lines []*Line
	var texts []*Text
	for _, e := range result.Entities {
		switch ent := e.(type) {
		case *Line:
			lines = append(lines, ent)
		case *Text:
			texts = append(texts, ent)
		default:
			t.Errorf("unexpected %T", e)
		}
	}
	assertEqInt(t, 2, len(lines))
	assertNearPoint(t, Point{110, 0, 0}, lines[0].P1)
	assertEqString(t, "WALLS", lines[0].Layer())
	assertEqInt(t, 3, int(lines[0].Color()))
	assertEqInt(t, 50, int(lines[0].LineWeight()))
	assertEqUInt64(t, 0, uint64(lines[0].Handle()))
	// top-level entities are copied as they are
	assertNearPoint(t, Point{0, 1, 0}, lines[1].P2)

	// the constant ATTDEF and the visible ATTRIB become TEXT; the variable ATTDEF and the hidden ATTRIB don't
	assertEqInt(t, 2, len(texts))
	assertEqString(t, "constant", texts[0].Value)
	assertNearPoint(t, Point{100, 0, 0}, texts[0].Location)
	assertEqString(t, "Konyha", texts[1].Value)
	assertNearPoint(t, Point{105, 5, 0}, texts[1].Location)
	assertEqString(t, "WALLS", texts[1].Layer())

	// the drawing is not changed
	assertEqInt(t, 2, len(drawing.Entities))
	assertEqPoint(t, Point{0, 0, 0}, drawing.Blocks[0].Entities[0].(*Line).P1)
}

func TestExplodeOneLevel(t *testing.T) {
	drawing := explodeDrawing()
	result := drawing.ExplodeInsert(drawing.Entities[0].(*Insert), ExplodeOptions{})
	inserts := 0
	for _, e := range result.Entities {
		if insert, ok := e.(*Insert); ok {
			inserts++
			assertEqString(t, "INNER", insert.Name)
		}
	}
	// nested INSERTs are kept (transforming them is not supported yet, so they are reported)
	assertEqInt(t, 0, inserts)
	assertEqInt(t, 1, len(result.Issues))
}

func TestResolveInheritedStopsAtNonBlockValues(t *testing.T) {
	line := lineEntity(Point{}, Point{1, 0, 0})
	line.SetColor(ByBlock())
	inner := insertEntity("INNER", Point{})
	inner.SetColor(ByLayer())
	inner.SetLayer("INNER-LAYER")
	outer := insertEntity("OUTER", Point{})
	outer.SetColor(Color(1))
	ResolveInherited(line, []*Insert{outer, inner})
	// the nearest INSERT decides: BYLAYER of the INSERT's layer
	assertEqInt(t, int(ByLayer()), int(line.Color()))
	assertEqString(t, "INNER-LAYER", line.Layer())
}

func TestExplodeWithoutInheritance(t *testing.T) {
	drawing := explodeDrawing()
	result := drawing.Explode(ExplodeOptions{Recursive: true})
	line := result.Entities[0].(*Line)
	assertEqString(t, "0", line.Layer())
	assertEqInt(t, int(ByBlock()), int(line.Color()))
}

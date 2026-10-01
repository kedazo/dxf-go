package dxf

import (
	"testing"
)

func viewportPairs(withClipBoundary bool) []CodePair {
	pairs := []CodePair{
		NewStringCodePair(0, "LWPOLYLINE"),
		NewStringCodePair(5, "8A"),
		NewShortCodePair(67, 1),
		NewIntCodePair(90, 4),
		NewShortCodePair(70, 1),
		NewDoubleCodePair(10, 10), NewDoubleCodePair(20, 80),
		NewDoubleCodePair(10, 410), NewDoubleCodePair(20, 80),
		NewDoubleCodePair(10, 410), NewDoubleCodePair(20, 280),
		NewDoubleCodePair(10, 10), NewDoubleCodePair(20, 280),
		NewStringCodePair(0, "VIEWPORT"),
		NewStringCodePair(5, "8B"),
		NewShortCodePair(67, 1),
		NewStringCodePair(100, "AcDbEntity"),
		NewStringCodePair(100, "AcDbViewport"),
		NewDoubleCodePair(10, 210), NewDoubleCodePair(20, 180), NewDoubleCodePair(30, 0),
		NewDoubleCodePair(40, 400),
		NewDoubleCodePair(41, 200),
		NewShortCodePair(68, 3),
		NewShortCodePair(69, 3),
		NewDoubleCodePair(12, 0), NewDoubleCodePair(22, 0),
		NewDoubleCodePair(16, 0), NewDoubleCodePair(26, 0), NewDoubleCodePair(36, 1),
		NewDoubleCodePair(17, 1000), NewDoubleCodePair(27, -400), NewDoubleCodePair(37, 0),
		NewDoubleCodePair(45, 20),
		NewDoubleCodePair(51, 90),
		NewStringCodePair(331, "2A"),
	}
	if withClipBoundary {
		pairs = append(pairs, NewIntCodePair(90, 65536+32768), NewStringCodePair(340, "8A"))
	} else {
		pairs = append(pairs, NewIntCodePair(90, 32768))
	}
	return pairs
}

func readViewport(t *testing.T, withClipBoundary bool) *Viewport {
	entities := parseEntities(t, viewportPairs(withClipBoundary)...)
	assertEqInt(t, 2, len(entities))
	viewport, ok := entities[1].(*Viewport)
	if !ok {
		t.Fatalf("expected a *Viewport, got %T", entities[1])
	}
	return viewport
}

func TestReadViewport(t *testing.T) {
	viewport := readViewport(t, true)
	assertEqPoint(t, Point{210, 180, 0}, viewport.Center)
	assertEqFloat64(t, 400, viewport.Width)
	assertEqFloat64(t, 200, viewport.Height)
	assertEqInt(t, 3, int(viewport.ID))
	assertEqPoint(t, Point{1000, -400, 0}, viewport.ViewTarget)
	assertEqFloat64(t, 90, viewport.TwistAngle)
	assertEqBool(t, true, viewport.IsInPaperSpace())
	assertEqBool(t, true, viewport.IsOn())
	assertEqBool(t, false, viewport.IsPaperSpaceView())
	assertEqBool(t, true, viewport.IsNonRectangularClipping())
	assertEqBool(t, false, viewport.IsTurnedOff())
	assertEqInt(t, 1, len(viewport.FrozenLayerHandles))
	assertEqUInt64(t, 0x2A, uint64(viewport.FrozenLayerHandles[0]))
	if _, ok := (*viewport.ClipBoundary()).(*LWPolyline); !ok {
		t.Errorf("expected the clip boundary to resolve to the LWPOLYLINE, got %T", *viewport.ClipBoundary())
	}
}

func TestViewportModelToPaperMatrix(t *testing.T) {
	viewport := readViewport(t, true)
	assertEqFloat64(t, 10, viewport.Scale())
	m := viewport.ModelToPaperMatrix()
	// the target is shown at the center; the view is twisted 90 degrees counter-clockwise and scaled by 10
	assertNearPoint(t, Point{210, 180, 0}, m.TransformPoint(Point{1000, -400, 0}))
	assertNearPoint(t, Point{210, 190, 0}, m.TransformPoint(Point{1001, -400, 0}))
	assertNearPoint(t, Point{200, 180, 0}, m.TransformPoint(Point{1000, -399, 0}))

	// the view center is the target's offset in display coordinates
	viewport.ViewCenter = Point{1, 2, 0}
	assertNearPoint(t, Point{200, 160, 0}, viewport.ModelToPaperMatrix().TransformPoint(Point{1000, -400, 0}))
}

func TestViewportClipPolygon(t *testing.T) {
	clipped := readViewport(t, true)
	points, ok := clipped.ClipPolygon(0)
	assertEqBool(t, true, ok)
	assertEqInt(t, 4, len(points))
	assertNearPoint(t, Point{10, 80, 0}, points[0])
	assertNearPoint(t, Point{410, 280, 0}, points[2])

	rectangular := readViewport(t, false)
	points, ok = rectangular.ClipPolygon(0)
	assertEqBool(t, true, ok)
	assertEqInt(t, 4, len(points))
	assertNearPoint(t, Point{10, 80, 0}, points[0])
	assertNearPoint(t, Point{410, 280, 0}, points[2])
}

func TestViewportClipPolygonOfCircle(t *testing.T) {
	viewport := NewViewport()
	viewport.SetIsNonRectangularClipping(true)
	circle := NewCircle()
	circle.Center = Point{5, 5, 0}
	circle.Radius = 2
	var item DrawingItem = circle
	viewport.SetClipBoundary(&item)
	points, ok := viewport.ClipPolygon(0.001)
	assertEqBool(t, true, ok)
	for _, p := range points {
		assertNearFloat64(t, 2, Point{p.X - 5, p.Y - 5, 0}.ToVector().Length())
	}

	// an unsupported boundary falls back to the rectangle
	var line DrawingItem = NewLine()
	viewport.SetClipBoundary(&line)
	points, ok = viewport.ClipPolygon(0)
	assertEqBool(t, false, ok)
	assertEqInt(t, 4, len(points))
}

func TestViewportFrozenLayers(t *testing.T) {
	viewport := NewViewport()
	viewport.FrozenLayerHandles = []Handle{0x2A, 0x99}
	d := NewDrawing()
	layer := *NewLayer()
	layer.Name = "frozen"
	layer.SetHandle(0x2A)
	d.Layers = append(d.Layers, layer)
	frozen := viewport.FrozenLayers(d)
	assertEqInt(t, 1, len(frozen))
	assertEqString(t, "frozen", frozen[0])
}

func TestWriteViewport(t *testing.T) {
	viewport := readViewport(t, true)
	actual := drawingCodePairsFromEntity(t, viewport, R2004)
	assertContainsCodePairs(t, []CodePair{
		NewStringCodePair(100, "AcDbViewport"),
		NewDoubleCodePair(10, 210), NewDoubleCodePair(20, 180), NewDoubleCodePair(30, 0),
		NewDoubleCodePair(40, 400),
		NewDoubleCodePair(41, 200),
		NewShortCodePair(68, 3),
		NewShortCodePair(69, 3),
		NewDoubleCodePair(12, 0), NewDoubleCodePair(22, 0),
	}, actual)
	assertContainsCodePairs(t, []CodePair{
		NewDoubleCodePair(51, 90),
		NewShortCodePair(72, 100),
		NewStringCodePair(331, "2A"),
		NewIntCodePair(90, 65536+32768),
		NewStringCodePair(340, "8A"),
	}, actual)
}

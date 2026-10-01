package dxf

import "math"

// Viewport is the VIEWPORT entity: a window on a layout (paper space) that shows model space. Not to be confused with
// ViewPort, an item of the VPORT table that describes the tiled views of model space.

// IsPaperSpaceView reports whether the viewport is its layout's own view of paper space (ID 1) rather than a view
// that shows model space on the sheet.
func (v *Viewport) IsPaperSpaceView() bool {
	return v.ID == 1
}

// IsOn reports whether the viewport is shown: status 0 and the "turned off" flag both mean off.
func (v *Viewport) IsOn() bool {
	return v.Status != 0 && !v.IsTurnedOff()
}

// Scale returns the paper space units per model space unit, e.g. 10 for a 1:100 view of a model in meters on a layout
// in millimeters, or 0 if the view height isn't set.
func (v *Viewport) Scale() float64 {
	if v.ViewHeight == 0 {
		return 0
	}
	return v.Height / v.ViewHeight
}

// ModelToPaperMatrix maps model space coordinates to the paper space coordinates where the viewport shows them: the
// model is viewed along ViewDirection, the view target lies at ViewCenter, the view is turned counter-clockwise by
// TwistAngle, scaled by Scale() and ViewCenter is placed at the viewport's Center. The resulting Z is the scaled
// depth along the view direction. Perspective views are mapped as parallel projections.
func (v *Viewport) ModelToPaperMatrix() Matrix {
	scale := v.Scale()
	return TranslationMatrix(v.Center.ToVector()).
		Mul(ScaleMatrix(scale, scale, scale)).
		Mul(TranslationMatrix(v.ViewCenter.ToVector().Neg())).
		Mul(RotationZMatrix(v.TwistAngle * math.Pi / 180)).
		Mul(WCSToOCSMatrix(v.ViewDirection)).
		Mul(TranslationMatrix(v.ViewTarget.ToVector().Neg()))
}

// ClipPolygon returns the outline that the viewport clips its view to, in paper space. With non-rectangular clipping it
// is the clip boundary entity (LWPOLYLINE, 2D POLYLINE or CIRCLE), with curves flattened as in
// HatchBoundaryPath.Polygon; otherwise it is the viewport rectangle. ok is false if the boundary entity is missing or of
// another type; the rectangle is returned then.
func (v *Viewport) ClipPolygon(tolerance float64) (points []Point, ok bool) {
	if v.IsNonRectangularClipping() && v.ClipBoundary() != nil {
		if points, ok = outlineOf(*v.ClipBoundary(), tolerance); ok {
			return points, true
		}
	} else {
		ok = true
	}

	halfWidth, halfHeight := v.Width/2, v.Height/2
	for _, corner := range [][2]float64{{-halfWidth, -halfHeight}, {halfWidth, -halfHeight}, {halfWidth, halfHeight}, {-halfWidth, halfHeight}} {
		points = append(points, Point{v.Center.X + corner[0], v.Center.Y + corner[1], v.Center.Z})
	}
	return points, ok
}

// outlineOf returns the closed outline of a clip boundary entity in world coordinates.
func outlineOf(item DrawingItem, tolerance float64) (points []Point, ok bool) {
	var ocsPoints [][2]float64
	var normal Vector
	var elevation float64
	switch e := item.(type) {
	case *LWPolyline:
		vertices, bulges := make([][2]float64, len(e.Vertices)), make([]float64, len(e.Vertices))
		for i, vertex := range e.Vertices {
			vertices[i], bulges[i] = [2]float64{vertex.X, vertex.Y}, vertex.Bulge
		}
		ocsPoints = flattenBulgedPolyline(vertices, bulges, true, tolerance)
		normal, elevation = e.ExtrusionDirection, e.Elevation()
	case *Polyline:
		if e.Is3DPolyline() || e.Is3DPolygonMesh() || e.IsPolyfaceMesh() {
			return nil, false
		}
		vertices, bulges := make([][2]float64, len(e.Vertices)), make([]float64, len(e.Vertices))
		for i, vertex := range e.Vertices {
			vertices[i], bulges[i] = [2]float64{vertex.Location.X, vertex.Location.Y}, vertex.Bulge
		}
		ocsPoints = flattenBulgedPolyline(vertices, bulges, true, tolerance)
		normal, elevation = e.Normal, e.Location.Z
	case *Circle:
		ocsCenter := WCSToOCSMatrix(e.Normal).TransformPoint(e.Center)
		ocsPoints = arcPoints([2]float64{ocsCenter.X, ocsCenter.Y}, e.Radius, 0, 2*math.Pi, tolerance)
		normal, elevation = e.Normal, ocsCenter.Z
	default:
		return nil, false
	}

	if len(ocsPoints) > 1 && nearlyEqual2(ocsPoints[0], ocsPoints[len(ocsPoints)-1]) {
		ocsPoints = ocsPoints[:len(ocsPoints)-1]
	}
	if len(ocsPoints) < 3 {
		return nil, false
	}
	toWCS := OCSToWCSMatrix(normal)
	for _, p := range ocsPoints {
		points = append(points, toWCS.TransformPoint(Point{p[0], p[1], elevation}))
	}
	return points, true
}

// FrozenLayers returns the names of the layers that are frozen in this viewport; handles that match no layer of the
// drawing are skipped.
func (v *Viewport) FrozenLayers(d *Drawing) (names []string) {
	layerNames := make(map[Handle]string, len(d.Layers))
	for i := range d.Layers {
		layerNames[d.Layers[i].Handle()] = d.Layers[i].Name
	}
	for _, handle := range v.FrozenLayerHandles {
		if name, ok := layerNames[handle]; ok {
			names = append(names, name)
		}
	}
	return
}

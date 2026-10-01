package dxf

// HatchStyle is the hatch island detection style (code 75).
type HatchStyle int16

const (
	HatchStyleOddParity HatchStyle = 0
	HatchStyleOutermost HatchStyle = 1
	HatchStyleEntire    HatchStyle = 2
)

// HatchPatternType tells where the hatch pattern comes from (code 76).
type HatchPatternType int16

const (
	HatchPatternTypeUserDefined HatchPatternType = 0
	HatchPatternTypePredefined  HatchPatternType = 1
	HatchPatternTypeCustom      HatchPatternType = 2
)

// HatchBoundaryPath represents one boundary loop of a hatch. A polyline path (PathType bit 2) uses Vertices and
// Bulges, any other path is a chain of Edges. All coordinates are in the hatch's object coordinate system.
type HatchBoundaryPath struct {
	PathType int // code 92 flags: 1 = external, 2 = polyline, 4 = derived, 8 = textbox, 16 = outermost

	// polyline paths
	Vertices [][2]float64
	Bulges   []float64 // one bulge per vertex, or nil if the path has no bulges
	IsClosed bool

	// edge paths
	Edges []HatchEdge

	// handles of the boundary objects of an associative hatch
	SourceHandles []Handle
}

// IsPolyline reports whether the path is stored as a polyline rather than as edges.
func (p *HatchBoundaryPath) IsPolyline() bool {
	return p.PathType&2 != 0
}

// HatchEdge is one edge of an edge-type boundary path: *HatchLineEdge, *HatchArcEdge, *HatchEllipseEdge or
// *HatchSplineEdge.
type HatchEdge interface {
	hatchEdgeType() int16
}

// HatchLineEdge is a straight boundary edge.
type HatchLineEdge struct {
	Start [2]float64
	End   [2]float64
}

// HatchArcEdge is a circular boundary edge. Angles are in degrees; for clockwise edges AutoCAD stores the angles
// mirrored (negated).
type HatchArcEdge struct {
	Center             [2]float64
	Radius             float64
	StartAngle         float64
	EndAngle           float64
	IsCounterClockwise bool
}

// HatchEllipseEdge is an elliptical boundary edge. MajorAxis is relative to Center; angles are in degrees and, for
// clockwise edges, stored mirrored like arc edges.
type HatchEllipseEdge struct {
	Center             [2]float64
	MajorAxis          [2]float64
	MinorAxisRatio     float64
	StartAngle         float64
	EndAngle           float64
	IsCounterClockwise bool
}

// HatchSplineEdge is a NURBS boundary edge. Weights are only used by rational splines. Fit data is only written
// for R2010 and later.
type HatchSplineEdge struct {
	Degree        int
	IsRational    bool
	IsPeriodic    bool
	Knots         []float64
	ControlPoints [][2]float64
	Weights       []float64
	FitPoints     [][2]float64
	StartTangent  [2]float64
	EndTangent    [2]float64
}

func (*HatchLineEdge) hatchEdgeType() int16    { return 1 }
func (*HatchArcEdge) hatchEdgeType() int16     { return 2 }
func (*HatchEllipseEdge) hatchEdgeType() int16 { return 3 }
func (*HatchSplineEdge) hatchEdgeType() int16  { return 4 }

// HatchPatternLine represents one line definition in a hatch pattern.
type HatchPatternLine struct {
	Angle   float64   // code 53 (degrees)
	BaseX   float64   // code 43
	BaseY   float64   // code 44
	OffsetX float64   // code 45
	OffsetY float64   // code 46
	Dashes  []float64 // code 49 values
}

// HatchGradient is the gradient fill definition of a hatch (R2004+).
type HatchGradient struct {
	IsGradient    bool    // code 450: false for an ordinary solid fill
	Reserved      int     // code 451
	Angle         float64 // code 460, radians
	Shift         float64 // code 461
	IsSingleColor bool    // code 452
	Tint          float64 // code 462
	Colors        []HatchGradientColor
	Name          string // code 470, e.g. "LINEAR"
}

// HatchGradientColor is one color stop of a gradient.
type HatchGradientColor struct {
	Value     float64 // code 463
	Color     Color   // code 63, 0 if not set
	TrueColor int     // code 421
}

// Hatch represents a DXF HATCH entity.
type Hatch struct {
	// fields for Entity interface
	handle           Handle
	isInPaperSpace   bool
	layer            string
	lineTypeName     string
	elevation        float64
	materialHandle   string
	color            Color
	lineWeight       LineWeight
	lineTypeScale    float64
	isVisible        bool
	imageByteCount   int
	previewImageData []string
	color24Bit       int
	colorName        string
	transparency     int
	shadowMode       ShadowMode
	pointerOwner     pointer
	pointerPlotStyle pointer

	// Hatch-specific fields
	ExtrusionDirection Vector
	PatternName        string
	SolidFill          bool // code 70
	IsAssociative      bool // code 71
	Paths              []HatchBoundaryPath
	Style              HatchStyle
	PatternType        HatchPatternType
	PatternAngle       float64 // code 52, degrees
	PatternScale       float64 // code 41
	IsPatternDouble    bool    // code 77
	PatternLines       []HatchPatternLine
	PixelSize          float64 // code 47
	SeedPoints         [][2]float64
	Gradient           *HatchGradient

	// hatch data is collected while reading and parsed once the entity is complete
	isReadingHatchData bool
	hatchData          []CodePair
}

func NewHatch() *Hatch {
	return &Hatch{
		handle:             0,
		isInPaperSpace:     false,
		layer:              "0",
		lineTypeName:       "BYLAYER",
		elevation:          0.0,
		materialHandle:     "BYLAYER",
		color:              ByLayer(),
		lineWeight:         NewLineWeightStandard(),
		lineTypeScale:      1.0,
		isVisible:          true,
		previewImageData:   []string{},
		shadowMode:         ShadowModeCastsAndReceivesShadows,
		ExtrusionDirection: *NewZAxis(),
		PatternName:        "SOLID",
		SolidFill:          true,
		PatternType:        HatchPatternTypePredefined,
		PatternScale:       1.0,
	}
}

func (e *Hatch) typeString() string      { return "HATCH" }
func (e *Hatch) minVersion() AcadVersion { return R14 }
func (e *Hatch) maxVersion() AcadVersion { return R2018 }

func (e *Hatch) tryApplyCodePair(codePair CodePair) {
	if e.isReadingHatchData {
		e.hatchData = append(e.hatchData, codePair)
		return
	}

	switch codePair.Code {
	case 100:
		if value, ok := codePair.Value.(StringCodePairValue); ok && value.Value == "AcDbHatch" {
			e.isReadingHatchData = true
		}
	case 2, 10, 70, 71, 91, 210:
		// files without subclass markers start the hatch data right away
		e.isReadingHatchData = true
		e.hatchData = append(e.hatchData, codePair)
	default:
		// codes like 92 and 330 mean something else inside the hatch data, so they are only common entity codes here
		tryApplyCodePairForEntity(e, codePair)
	}
}

// parseHatchData interprets the collected hatch data; the counts in the data drive the parsing, but every expected
// code is checked, so a malformed hatch can't swallow unrelated values.
func (e *Hatch) parseHatchData() {
	r := &hatchDataReader{pairs: e.hatchData}
	e.hatchData = nil
	e.isReadingHatchData = false

	var gradientColor *HatchGradientColor
	gradient := func() *HatchGradient {
		if e.Gradient == nil {
			e.Gradient = &HatchGradient{}
		}
		return e.Gradient
	}

	for !r.done() {
		pair := r.next()
		switch pair.Code {
		case 30:
			e.elevation = doubleValue(pair)
		case 210:
			e.ExtrusionDirection.X = doubleValue(pair)
		case 220:
			e.ExtrusionDirection.Y = doubleValue(pair)
		case 230:
			e.ExtrusionDirection.Z = doubleValue(pair)
		case 2:
			e.PatternName = stringValue(pair)
		case 70:
			e.SolidFill = shortValue(pair) != 0
		case 71:
			e.IsAssociative = shortValue(pair) != 0
		case 91:
			count := intValue(pair)
			for i := 0; i < count; i++ {
				path, ok := r.readBoundaryPath()
				if !ok {
					break
				}
				e.Paths = append(e.Paths, path)
			}
		case 75:
			e.Style = HatchStyle(shortValue(pair))
		case 76:
			e.PatternType = HatchPatternType(shortValue(pair))
		case 52:
			e.PatternAngle = doubleValue(pair)
		case 41:
			e.PatternScale = doubleValue(pair)
		case 77:
			e.IsPatternDouble = shortValue(pair) != 0
		case 78:
			count := int(shortValue(pair))
			for i := 0; i < count; i++ {
				line, ok := r.readPatternLine()
				if !ok {
					break
				}
				e.PatternLines = append(e.PatternLines, line)
			}
		case 47:
			e.PixelSize = doubleValue(pair)
		case 98:
			count := intValue(pair)
			for i := 0; i < count; i++ {
				point, ok := r.readPoint(10, 20)
				if !ok {
					break
				}
				e.SeedPoints = append(e.SeedPoints, point)
			}
		case 450:
			gradient().IsGradient = intValue(pair) != 0
		case 451:
			gradient().Reserved = intValue(pair)
		case 452:
			gradient().IsSingleColor = intValue(pair) != 0
		case 453:
			// color count; the colors follow as 463/63/421
			gradient()
		case 460:
			gradient().Angle = doubleValue(pair)
		case 461:
			gradient().Shift = doubleValue(pair)
		case 462:
			gradient().Tint = doubleValue(pair)
		case 463:
			g := gradient()
			g.Colors = append(g.Colors, HatchGradientColor{Value: doubleValue(pair)})
			gradientColor = &g.Colors[len(g.Colors)-1]
		case 63:
			if gradientColor != nil {
				gradientColor.Color = Color(shortValue(pair))
			}
		case 421:
			if gradientColor != nil {
				gradientColor.TrueColor = intValue(pair)
			}
		case 470:
			gradient().Name = stringValue(pair)
		default:
			// e.g. the elevation point's X/Y (always 0) or extended data
		}
	}
}

// hatchDataReader is a cursor over the collected hatch data.
type hatchDataReader struct {
	pairs    []CodePair
	position int
}

func (r *hatchDataReader) done() bool {
	return r.position >= len(r.pairs)
}

func (r *hatchDataReader) next() CodePair {
	pair := r.pairs[r.position]
	r.position++
	return pair
}

// peekCode returns the code `offset` pairs ahead, or -1.
func (r *hatchDataReader) peekCode(offset int) int {
	if r.position+offset >= len(r.pairs) {
		return -1
	}
	return r.pairs[r.position+offset].Code
}

// take consumes the next pair only if it has the expected code.
func (r *hatchDataReader) take(code int) (CodePair, bool) {
	if r.peekCode(0) != code {
		return CodePair{}, false
	}
	return r.next(), true
}

func (r *hatchDataReader) takeDouble(code int) (float64, bool) {
	pair, ok := r.take(code)
	return doubleValue(pair), ok
}

func (r *hatchDataReader) takeShort(code int) (int16, bool) {
	pair, ok := r.take(code)
	return shortValue(pair), ok
}

func (r *hatchDataReader) takeInt(code int) (int, bool) {
	pair, ok := r.take(code)
	return intValue(pair), ok
}

func (r *hatchDataReader) readPoint(xCode, yCode int) ([2]float64, bool) {
	x, ok := r.takeDouble(xCode)
	if !ok {
		return [2]float64{}, false
	}
	y, _ := r.takeDouble(yCode)
	return [2]float64{x, y}, true
}

func (r *hatchDataReader) readBoundaryPath() (path HatchBoundaryPath, ok bool) {
	pathType, ok := r.takeInt(92)
	if !ok {
		return
	}
	path.PathType = pathType

	if path.IsPolyline() {
		hasBulge, _ := r.takeShort(72)
		isClosed, _ := r.takeShort(73)
		path.IsClosed = isClosed != 0
		count, _ := r.takeInt(93)
		if hasBulge != 0 {
			path.Bulges = make([]float64, 0, count)
		}
		for i := 0; i < count; i++ {
			vertex, ok := r.readPoint(10, 20)
			if !ok {
				break
			}
			path.Vertices = append(path.Vertices, vertex)
			bulge, hasVertexBulge := r.takeDouble(42)
			if hasVertexBulge && path.Bulges == nil {
				// a bulge without the has-bulge flag: the earlier vertices had none
				path.Bulges = make([]float64, len(path.Vertices)-1, count)
			}
			if path.Bulges != nil {
				path.Bulges = append(path.Bulges, bulge)
			}
		}
	} else {
		count, _ := r.takeInt(93)
		for i := 0; i < count; i++ {
			edge, ok := r.readEdge(i == count-1)
			if !ok {
				break
			}
			path.Edges = append(path.Edges, edge)
		}
	}

	if count, ok := r.takeInt(97); ok {
		for i := 0; i < count; i++ {
			pair, ok := r.take(330)
			if !ok {
				break
			}
			path.SourceHandles = append(path.SourceHandles, handleFromString(stringValue(pair)))
		}
	}

	return path, true
}

func (r *hatchDataReader) readEdge(isLastEdge bool) (HatchEdge, bool) {
	edgeType, ok := r.takeShort(72)
	if !ok {
		return nil, false
	}

	switch edgeType {
	case 1:
		edge := &HatchLineEdge{}
		edge.Start, _ = r.readPoint(10, 20)
		edge.End, _ = r.readPoint(11, 21)
		return edge, true
	case 2:
		edge := &HatchArcEdge{}
		edge.Center, _ = r.readPoint(10, 20)
		edge.Radius, _ = r.takeDouble(40)
		edge.StartAngle, _ = r.takeDouble(50)
		edge.EndAngle, _ = r.takeDouble(51)
		ccw, _ := r.takeShort(73)
		edge.IsCounterClockwise = ccw != 0
		return edge, true
	case 3:
		edge := &HatchEllipseEdge{}
		edge.Center, _ = r.readPoint(10, 20)
		edge.MajorAxis, _ = r.readPoint(11, 21)
		edge.MinorAxisRatio, _ = r.takeDouble(40)
		edge.StartAngle, _ = r.takeDouble(50)
		edge.EndAngle, _ = r.takeDouble(51)
		ccw, _ := r.takeShort(73)
		edge.IsCounterClockwise = ccw != 0
		return edge, true
	case 4:
		return r.readSplineEdge(isLastEdge), true
	default:
		return nil, false
	}
}

func (r *hatchDataReader) readSplineEdge(isLastEdge bool) *HatchSplineEdge {
	edge := &HatchSplineEdge{}
	edge.Degree, _ = r.takeInt(94)
	rational, _ := r.takeShort(73)
	edge.IsRational = rational != 0
	periodic, _ := r.takeShort(74)
	edge.IsPeriodic = periodic != 0
	knotCount, _ := r.takeInt(95)
	controlPointCount, _ := r.takeInt(96)
	for i := 0; i < knotCount; i++ {
		knot, ok := r.takeDouble(40)
		if !ok {
			break
		}
		edge.Knots = append(edge.Knots, knot)
	}
	for i := 0; i < controlPointCount; i++ {
		point, ok := r.readPoint(10, 20)
		if !ok {
			break
		}
		edge.ControlPoints = append(edge.ControlPoints, point)
		if weight, ok := r.takeDouble(42); ok {
			edge.Weights = append(edge.Weights, weight)
		}
	}

	if r.hasSplineFitData(isLastEdge) {
		fitCount, _ := r.takeInt(97)
		for i := 0; i < fitCount; i++ {
			point, ok := r.readPoint(11, 21)
			if !ok {
				break
			}
			edge.FitPoints = append(edge.FitPoints, point)
		}
		edge.StartTangent, _ = r.readPoint(12, 22)
		edge.EndTangent, _ = r.readPoint(13, 23)
	}

	return edge
}

// hasSplineFitData tells whether a following 97 is the spline's fit point count (R2010+) or the path's source
// boundary count, which also uses 97.
func (r *hatchDataReader) hasSplineFitData(isLastEdge bool) bool {
	if r.peekCode(0) != 97 {
		return false
	}
	if !isLastEdge {
		// the path's source boundary count only follows the last edge
		return true
	}

	count := intValue(r.pairs[r.position])
	following := r.peekCode(1)
	if count > 0 {
		return following == 11
	}
	return following == 97 || following == 12
}

func (r *hatchDataReader) readPatternLine() (line HatchPatternLine, ok bool) {
	line.Angle, ok = r.takeDouble(53)
	if !ok {
		return
	}
	line.BaseX, _ = r.takeDouble(43)
	line.BaseY, _ = r.takeDouble(44)
	line.OffsetX, _ = r.takeDouble(45)
	line.OffsetY, _ = r.takeDouble(46)
	dashCount, _ := r.takeShort(79)
	for i := 0; i < int(dashCount); i++ {
		dash, ok := r.takeDouble(49)
		if !ok {
			break
		}
		line.Dashes = append(line.Dashes, dash)
	}
	return line, true
}

func doubleValue(pair CodePair) float64 {
	if value, ok := pair.Value.(DoubleCodePairValue); ok {
		return value.Value
	}
	return 0
}

func shortValue(pair CodePair) int16 {
	if value, ok := pair.Value.(ShortCodePairValue); ok {
		return value.Value
	}
	return 0
}

func intValue(pair CodePair) int {
	switch value := pair.Value.(type) {
	case IntCodePairValue:
		return value.Value
	case ShortCodePairValue:
		return int(value.Value)
	}
	return 0
}

func stringValue(pair CodePair) string {
	if value, ok := pair.Value.(StringCodePairValue); ok {
		return value.Value
	}
	return ""
}

func (e *Hatch) codePairs(version AcadVersion) (pairs []CodePair) {
	pairs = append(pairs, NewStringCodePair(0, "HATCH"))
	pairs = append(pairs, codePairsForEntity(e, version)...)
	return
}

// Entity interface boilerplate
func (e *Hatch) pointers() []*pointer {
	return []*pointer{&e.pointerOwner, &e.pointerPlotStyle}
}
func (e *Hatch) getOwnerPointer() pointer           { return e.pointerOwner }
func (e *Hatch) setOwnerPointerHandle(h Handle)     { e.pointerOwner.handle = h }
func (e *Hatch) Owner() *DrawingItem                { return e.pointerOwner.value }
func (e *Hatch) SetOwner(val *DrawingItem)          { e.pointerOwner.value = val }
func (e *Hatch) getPlotStylePointer() pointer       { return e.pointerPlotStyle }
func (e *Hatch) setPlotStylePointerHandle(h Handle) { e.pointerPlotStyle.handle = h }
func (e *Hatch) PlotStyle() *DrawingItem            { return e.pointerPlotStyle.value }
func (e *Hatch) SetPlotStyle(val *DrawingItem)      { e.pointerPlotStyle.value = val }
func (e *Hatch) Handle() Handle                     { return e.handle }
func (e *Hatch) SetHandle(val Handle)               { e.handle = val }
func (e *Hatch) IsInPaperSpace() bool               { return e.isInPaperSpace }
func (e *Hatch) SetIsInPaperSpace(val bool)         { e.isInPaperSpace = val }
func (e *Hatch) Layer() string                      { return e.layer }
func (e *Hatch) SetLayer(val string)                { e.layer = val }
func (e *Hatch) LineTypeName() string               { return e.lineTypeName }
func (e *Hatch) SetLineTypeName(val string)         { e.lineTypeName = val }
func (e *Hatch) Elevation() float64                 { return e.elevation }
func (e *Hatch) SetElevation(val float64)           { e.elevation = val }
func (e *Hatch) MaterialHandle() string             { return e.materialHandle }
func (e *Hatch) SetMaterialHandle(val string)       { e.materialHandle = val }
func (e *Hatch) Color() Color                       { return e.color }
func (e *Hatch) SetColor(val Color)                 { e.color = val }
func (e *Hatch) LineWeight() LineWeight             { return e.lineWeight }
func (e *Hatch) SetLineWeight(val LineWeight)       { e.lineWeight = val }
func (e *Hatch) LineTypeScale() float64             { return e.lineTypeScale }
func (e *Hatch) SetLineTypeScale(val float64)       { e.lineTypeScale = val }
func (e *Hatch) IsVisible() bool                    { return e.isVisible }
func (e *Hatch) SetIsVisible(val bool)              { e.isVisible = val }
func (e *Hatch) ImageByteCount() int                { return e.imageByteCount }
func (e *Hatch) SetImageByteCount(val int)          { e.imageByteCount = val }
func (e *Hatch) PreviewImageData() []string         { return e.previewImageData }
func (e *Hatch) SetPreviewImageData(val []string)   { e.previewImageData = val }
func (e *Hatch) Color24Bit() int                    { return e.color24Bit }
func (e *Hatch) SetColor24Bit(val int)              { e.color24Bit = val }
func (e *Hatch) ColorName() string                  { return e.colorName }
func (e *Hatch) SetColorName(val string)            { e.colorName = val }
func (e *Hatch) Transparency() int                  { return e.transparency }
func (e *Hatch) SetTransparency(val int)            { e.transparency = val }
func (e *Hatch) ShadowMode() ShadowMode             { return e.shadowMode }
func (e *Hatch) SetShadowMode(val ShadowMode)       { e.shadowMode = val }

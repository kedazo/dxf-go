package dxf

// HatchBoundaryPath represents one boundary loop of a hatch.
type HatchBoundaryPath struct {
	PathType int          // code 92 flags
	Vertices [][2]float64 // polyline boundary vertices
}

// HatchPatternLine represents one line definition in a hatch pattern.
type HatchPatternLine struct {
	Angle   float64   // code 53 (degrees)
	BaseX   float64   // code 43
	BaseY   float64   // code 44
	OffsetX float64   // code 45
	OffsetY float64   // code 46
	Dashes  []float64 // code 49 values
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
	PatternName  string
	SolidFill    bool    // code 70: 1=solid, 0=pattern
	PatternScale float64 // code 41
	PatternAngle float64 // code 52
	Paths        []HatchBoundaryPath
	PatternLines []HatchPatternLine

	// parsing state (private)
	parseState       int // 0=header, 1=paths, 2=pattern
	pathCount        int // code 91
	currentPathType  int
	currentVertCount int
	patternLineCount int // code 78
	currentDashCount int // code 79
	lastSubclass     string
}

func NewHatch() *Hatch {
	return &Hatch{
		handle:         0,
		isInPaperSpace: false,
		layer:          "0",
		lineTypeName:   "BYLAYER",
		elevation:      0.0,
		materialHandle: "BYLAYER",
		color:          ByLayer(),
		lineWeight:     NewLineWeightStandard(),
		lineTypeScale:  1.0,
		isVisible:      true,
		previewImageData: []string{},
		shadowMode:     ShadowModeCastsAndReceivesShadows,
		PatternScale:   1.0,
	}
}

func (e *Hatch) typeString() string { return "HATCH" }
func (e *Hatch) minVersion() AcadVersion { return R13 }
func (e *Hatch) maxVersion() AcadVersion { return R2018 }

func (e *Hatch) tryApplyCodePair(codePair CodePair) {
	switch codePair.Code {
	case 100:
		e.lastSubclass = codePair.Value.(StringCodePairValue).Value
		if e.lastSubclass == "AcDbHatch" {
			e.parseState = 0 // entering hatch data
		}
	case 2:
		e.PatternName = codePair.Value.(StringCodePairValue).Value
	case 70:
		if e.lastSubclass == "AcDbHatch" {
			e.SolidFill = codePair.Value.(ShortCodePairValue).Value == 1
		}
	case 41:
		e.PatternScale = codePair.Value.(DoubleCodePairValue).Value
	case 52:
		e.PatternAngle = codePair.Value.(DoubleCodePairValue).Value
	case 91:
		e.pathCount = codePair.Value.(IntCodePairValue).Value
		e.parseState = 1 // reading boundary paths
	case 92:
		// start a new boundary path
		e.currentPathType = codePair.Value.(IntCodePairValue).Value
		e.Paths = append(e.Paths, HatchBoundaryPath{PathType: e.currentPathType})
	case 93:
		e.currentVertCount = codePair.Value.(IntCodePairValue).Value
	case 10:
		if e.parseState == 1 && len(e.Paths) > 0 {
			// boundary vertex X — start new vertex
			path := &e.Paths[len(e.Paths)-1]
			path.Vertices = append(path.Vertices, [2]float64{codePair.Value.(DoubleCodePairValue).Value, 0})
		}
	case 20:
		if e.parseState == 1 && len(e.Paths) > 0 {
			// boundary vertex Y — augment last vertex
			path := &e.Paths[len(e.Paths)-1]
			if len(path.Vertices) > 0 {
				path.Vertices[len(path.Vertices)-1][1] = codePair.Value.(DoubleCodePairValue).Value
			}
		}
	case 78:
		e.patternLineCount = int(codePair.Value.(ShortCodePairValue).Value)
		e.parseState = 2 // reading pattern lines
	case 53:
		if e.parseState == 2 {
			// start a new pattern line
			e.PatternLines = append(e.PatternLines, HatchPatternLine{
				Angle: codePair.Value.(DoubleCodePairValue).Value,
			})
		}
	case 43:
		if e.parseState == 2 && len(e.PatternLines) > 0 {
			e.PatternLines[len(e.PatternLines)-1].BaseX = codePair.Value.(DoubleCodePairValue).Value
		}
	case 44:
		if e.parseState == 2 && len(e.PatternLines) > 0 {
			e.PatternLines[len(e.PatternLines)-1].BaseY = codePair.Value.(DoubleCodePairValue).Value
		}
	case 45:
		if e.parseState == 2 && len(e.PatternLines) > 0 {
			e.PatternLines[len(e.PatternLines)-1].OffsetX = codePair.Value.(DoubleCodePairValue).Value
		}
	case 46:
		if e.parseState == 2 && len(e.PatternLines) > 0 {
			e.PatternLines[len(e.PatternLines)-1].OffsetY = codePair.Value.(DoubleCodePairValue).Value
		}
	case 79:
		e.currentDashCount = int(codePair.Value.(ShortCodePairValue).Value)
	case 49:
		if e.parseState == 2 && len(e.PatternLines) > 0 {
			e.PatternLines[len(e.PatternLines)-1].Dashes = append(
				e.PatternLines[len(e.PatternLines)-1].Dashes,
				codePair.Value.(DoubleCodePairValue).Value,
			)
		}
	default:
		tryApplyCodePairForEntity(e, codePair)
	}
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
func (e *Hatch) getOwnerPointer() pointer          { return e.pointerOwner }
func (e *Hatch) setOwnerPointerHandle(h Handle)     { e.pointerOwner.handle = h }
func (e *Hatch) Owner() *DrawingItem                { return e.pointerOwner.value }
func (e *Hatch) SetOwner(val *DrawingItem)          { e.pointerOwner.value = val }
func (e *Hatch) getPlotStylePointer() pointer        { return e.pointerPlotStyle }
func (e *Hatch) setPlotStylePointerHandle(h Handle)  { e.pointerPlotStyle.handle = h }
func (e *Hatch) PlotStyle() *DrawingItem             { return e.pointerPlotStyle.value }
func (e *Hatch) SetPlotStyle(val *DrawingItem)       { e.pointerPlotStyle.value = val }
func (e *Hatch) Handle() Handle                      { return e.handle }
func (e *Hatch) SetHandle(val Handle)                { e.handle = val }
func (e *Hatch) IsInPaperSpace() bool                { return e.isInPaperSpace }
func (e *Hatch) SetIsInPaperSpace(val bool)          { e.isInPaperSpace = val }
func (e *Hatch) Layer() string                       { return e.layer }
func (e *Hatch) SetLayer(val string)                 { e.layer = val }
func (e *Hatch) LineTypeName() string                { return e.lineTypeName }
func (e *Hatch) SetLineTypeName(val string)          { e.lineTypeName = val }
func (e *Hatch) Elevation() float64                  { return e.elevation }
func (e *Hatch) SetElevation(val float64)            { e.elevation = val }
func (e *Hatch) MaterialHandle() string              { return e.materialHandle }
func (e *Hatch) SetMaterialHandle(val string)        { e.materialHandle = val }
func (e *Hatch) Color() Color                        { return e.color }
func (e *Hatch) SetColor(val Color)                  { e.color = val }
func (e *Hatch) LineWeight() LineWeight              { return e.lineWeight }
func (e *Hatch) SetLineWeight(val LineWeight)        { e.lineWeight = val }
func (e *Hatch) LineTypeScale() float64              { return e.lineTypeScale }
func (e *Hatch) SetLineTypeScale(val float64)        { e.lineTypeScale = val }
func (e *Hatch) IsVisible() bool                     { return e.isVisible }
func (e *Hatch) SetIsVisible(val bool)               { e.isVisible = val }
func (e *Hatch) ImageByteCount() int                 { return e.imageByteCount }
func (e *Hatch) SetImageByteCount(val int)           { e.imageByteCount = val }
func (e *Hatch) PreviewImageData() []string          { return e.previewImageData }
func (e *Hatch) SetPreviewImageData(val []string)    { e.previewImageData = val }
func (e *Hatch) Color24Bit() int                     { return e.color24Bit }
func (e *Hatch) SetColor24Bit(val int)               { e.color24Bit = val }
func (e *Hatch) ColorName() string                   { return e.colorName }
func (e *Hatch) SetColorName(val string)             { e.colorName = val }
func (e *Hatch) Transparency() int                   { return e.transparency }
func (e *Hatch) SetTransparency(val int)             { e.transparency = val }
func (e *Hatch) ShadowMode() ShadowMode             { return e.shadowMode }
func (e *Hatch) SetShadowMode(val ShadowMode)       { e.shadowMode = val }

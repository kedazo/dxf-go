package dxf

// entityCommon holds the fields every entity has; hand-written entities embed it to implement the Entity interface.
type entityCommon struct {
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
}

func newEntityCommon() entityCommon {
	return entityCommon{
		layer:            "0",
		lineTypeName:     "BYLAYER",
		materialHandle:   "BYLAYER",
		color:            ByLayer(),
		lineWeight:       NewLineWeightStandard(),
		lineTypeScale:    1.0,
		isVisible:        true,
		previewImageData: []string{},
		shadowMode:       ShadowModeCastsAndReceivesShadows,
	}
}

func (e *entityCommon) pointers() []*pointer {
	return []*pointer{&e.pointerOwner, &e.pointerPlotStyle}
}
func (e *entityCommon) getOwnerPointer() pointer           { return e.pointerOwner }
func (e *entityCommon) setOwnerPointerHandle(h Handle)     { e.pointerOwner.handle = h }
func (e *entityCommon) Owner() *DrawingItem                { return e.pointerOwner.value }
func (e *entityCommon) SetOwner(val *DrawingItem)          { e.pointerOwner.value = val }
func (e *entityCommon) getPlotStylePointer() pointer       { return e.pointerPlotStyle }
func (e *entityCommon) setPlotStylePointerHandle(h Handle) { e.pointerPlotStyle.handle = h }
func (e *entityCommon) PlotStyle() *DrawingItem            { return e.pointerPlotStyle.value }
func (e *entityCommon) SetPlotStyle(val *DrawingItem)      { e.pointerPlotStyle.value = val }
func (e *entityCommon) Handle() Handle                     { return e.handle }
func (e *entityCommon) SetHandle(val Handle)               { e.handle = val }
func (e *entityCommon) IsInPaperSpace() bool               { return e.isInPaperSpace }
func (e *entityCommon) SetIsInPaperSpace(val bool)         { e.isInPaperSpace = val }
func (e *entityCommon) Layer() string                      { return e.layer }
func (e *entityCommon) SetLayer(val string)                { e.layer = val }
func (e *entityCommon) LineTypeName() string               { return e.lineTypeName }
func (e *entityCommon) SetLineTypeName(val string)         { e.lineTypeName = val }
func (e *entityCommon) Elevation() float64                 { return e.elevation }
func (e *entityCommon) SetElevation(val float64)           { e.elevation = val }
func (e *entityCommon) MaterialHandle() string             { return e.materialHandle }
func (e *entityCommon) SetMaterialHandle(val string)       { e.materialHandle = val }
func (e *entityCommon) Color() Color                       { return e.color }
func (e *entityCommon) SetColor(val Color)                 { e.color = val }
func (e *entityCommon) LineWeight() LineWeight             { return e.lineWeight }
func (e *entityCommon) SetLineWeight(val LineWeight)       { e.lineWeight = val }
func (e *entityCommon) LineTypeScale() float64             { return e.lineTypeScale }
func (e *entityCommon) SetLineTypeScale(val float64)       { e.lineTypeScale = val }
func (e *entityCommon) IsVisible() bool                    { return e.isVisible }
func (e *entityCommon) SetIsVisible(val bool)              { e.isVisible = val }
func (e *entityCommon) ImageByteCount() int                { return e.imageByteCount }
func (e *entityCommon) SetImageByteCount(val int)          { e.imageByteCount = val }
func (e *entityCommon) PreviewImageData() []string         { return e.previewImageData }
func (e *entityCommon) SetPreviewImageData(val []string)   { e.previewImageData = val }
func (e *entityCommon) Color24Bit() int                    { return e.color24Bit }
func (e *entityCommon) SetColor24Bit(val int)              { e.color24Bit = val }
func (e *entityCommon) ColorName() string                  { return e.colorName }
func (e *entityCommon) SetColorName(val string)            { e.colorName = val }
func (e *entityCommon) Transparency() int                  { return e.transparency }
func (e *entityCommon) SetTransparency(val int)            { e.transparency = val }
func (e *entityCommon) ShadowMode() ShadowMode             { return e.shadowMode }
func (e *entityCommon) SetShadowMode(val ShadowMode)       { e.shadowMode = val }

package dxf

import (
	"strings"
)

// Layout is a LAYOUT object: a layout tab (or the model tab) with its plot settings. Layouts are read from the OBJECTS
// section, which is not written.
type Layout struct {
	Handle Handle

	// plot settings (AcDbPlotSettings)
	PageSetupName         string     // code 1
	PlotConfigurationName string     // code 2, the printer or plotter
	PaperSizeName         string     // code 4
	PlotViewName          string     // code 6
	PlotStyleSheetName    string     // code 7
	MarginLeft            float64    // code 40, millimeters
	MarginBottom          float64    // code 41, millimeters
	MarginRight           float64    // code 42, millimeters
	MarginTop             float64    // code 43, millimeters
	PaperWidth            float64    // code 44, millimeters
	PaperHeight           float64    // code 45, millimeters
	PlotOrigin            [2]float64 // codes 46/47, millimeters
	PlotWindowMin         [2]float64 // codes 48/49
	PlotWindowMax         [2]float64 // codes 140/141
	CustomScaleNumerator  float64    // code 142, paper units
	CustomScaleDenom      float64    // code 143, drawing units
	PlotLayoutFlags       int        // code 70 of the plot settings
	PaperUnits            int16      // code 72: 0 = inches, 1 = millimeters, 2 = pixels
	PlotRotation          int16      // code 73: 0, 1, 2, 3 = 0, 90, 180, 270 degrees counter-clockwise
	PlotType              int16      // code 74: 0 = display, 1 = extents, 2 = limits, 3 = view, 4 = window, 5 = layout
	StandardScaleType     int16      // code 75
	ScaleFactor           float64    // code 147

	// layout (AcDbLayout)
	Name                     string // code 1
	Flags                    int    // code 70 of the layout
	TabOrder                 int    // code 71; the model tab is 0
	LimitsMin                Point  // codes 10/20
	LimitsMax                Point  // codes 11/21
	InsertionBase            Point  // codes 12/22/32
	ExtentsMin               Point  // codes 14/24/34
	ExtentsMax               Point  // codes 15/25/35
	Elevation                float64
	BlockRecordHandle        Handle // code 330 of the layout: the BLOCK_RECORD of its *Model_Space or *Paper_Space block
	LastActiveViewportHandle Handle // code 331

	// the layout's UCS, as it was when the layout was last active; zero vectors if the file has none
	UCSOrigin           Point  // codes 13/23/33
	UCSXAxis            Vector // codes 16/26/36
	UCSYAxis            Vector // codes 17/27/37
	UCSOrthographicType int16  // code 76: 0 = not orthographic, 1 = top, 2 = bottom, 3 = front, 4 = back, 5 = left, 6 = right
	NamedUCSHandle      Handle // code 345, a UCS table record; 0 if the UCS is unnamed or the world
	BaseUCSHandle       Handle // code 346, the UCS the orthographic type is relative to; 0 = the world

	XData XData // extended data (1001…)
}

// ImageDefinition is an IMAGEDEF object: the image file that IMAGE entities show. Image definitions are read from the
// OBJECTS section, which is not written.
type ImageDefinition struct {
	Handle          Handle
	ClassVersion    int    // code 90
	FileName        string // code 1, as stored: often relative to the drawing
	ImageSize       Vector // codes 10/20, pixels
	PixelSize       Vector // codes 11/21, the size of one pixel in drawing units
	IsLoaded        bool   // code 280
	ResolutionUnits int16  // code 281: 0 = none, 2 = centimeters, 5 = inches
	XData           XData  // extended data (1001…)
}

// objectParsers read the objects this library models from their group codes; other objects are skipped.
var objectParsers = map[string]func(drawing *Drawing, pairs []CodePair){
	"LAYOUT": func(d *Drawing, pairs []CodePair) { d.Layouts = append(d.Layouts, parseLayout(pairs)) },
	"IMAGEDEF": func(d *Drawing, pairs []CodePair) {
		d.ImageDefinitions = append(d.ImageDefinitions, parseImageDefinition(pairs))
	},
	"RASTERVARIABLES":  func(d *Drawing, pairs []CodePair) { d.RasterVariables = parseRasterVariables(pairs) },
	"WIPEOUTVARIABLES": func(d *Drawing, pairs []CodePair) { d.WipeoutVariables = parseWipeoutVariables(pairs) },
	"MLEADERSTYLE": func(d *Drawing, pairs []CodePair) {
		d.MLeaderStyles = append(d.MLeaderStyles, parseMLeaderStyle(pairs))
	},
	"TABLESTYLE": func(d *Drawing, pairs []CodePair) { d.TableStyles = append(d.TableStyles, parseTableStyle(pairs)) },
}

// readObjectsSection reads the objects in objectParsers; other objects are skipped.
func readObjectsSection(drawing *Drawing, np CodePair, reader codePairReader) (nextPair CodePair, err error) {
	nextPair = np
	for err == nil && !nextPair.isEndSection() {
		if nextPair.Code != 0 {
			// stray data outside an object
			nextPair, err = reader.readCodePair()
			continue
		}

		parse := objectParsers[nextPair.Value.(StringCodePairValue).Value]
		var pairs []CodePair
		for nextPair, err = reader.readCodePair(); err == nil && nextPair.Code != 0; nextPair, err = reader.readCodePair() {
			if parse != nil {
				pairs = append(pairs, nextPair)
			}
		}
		if parse != nil {
			parse(drawing, pairs)
		}
	}
	return
}

func parseLayout(pairs []CodePair) (layout Layout) {
	layout.XData = xdataFromPairs(pairs)
	subclass := ""
	inApplicationGroup := false
	for _, pair := range pairs {
		switch {
		case pair.Code == 102:
			// {ACAD_REACTORS ... } and similar groups hold handles that aren't the layout's own
			inApplicationGroup = strings.HasPrefix(stringValue(pair), "{")
			continue
		case inApplicationGroup:
			continue
		case pair.Code == 100:
			subclass = stringValue(pair)
			continue
		case pair.Code == 5:
			layout.Handle = handleFromString(stringValue(pair))
			continue
		}

		if subclass == "AcDbLayout" {
			switch pair.Code {
			case 1:
				layout.Name = stringValue(pair)
			case 70:
				layout.Flags = int(shortValue(pair))
			case 71:
				layout.TabOrder = int(shortValue(pair))
			case 10:
				layout.LimitsMin.X = doubleValue(pair)
			case 20:
				layout.LimitsMin.Y = doubleValue(pair)
			case 11:
				layout.LimitsMax.X = doubleValue(pair)
			case 21:
				layout.LimitsMax.Y = doubleValue(pair)
			case 12:
				layout.InsertionBase.X = doubleValue(pair)
			case 22:
				layout.InsertionBase.Y = doubleValue(pair)
			case 32:
				layout.InsertionBase.Z = doubleValue(pair)
			case 14:
				layout.ExtentsMin.X = doubleValue(pair)
			case 24:
				layout.ExtentsMin.Y = doubleValue(pair)
			case 34:
				layout.ExtentsMin.Z = doubleValue(pair)
			case 15:
				layout.ExtentsMax.X = doubleValue(pair)
			case 25:
				layout.ExtentsMax.Y = doubleValue(pair)
			case 35:
				layout.ExtentsMax.Z = doubleValue(pair)
			case 146:
				layout.Elevation = doubleValue(pair)
			case 330:
				layout.BlockRecordHandle = handleFromString(stringValue(pair))
			case 331:
				layout.LastActiveViewportHandle = handleFromString(stringValue(pair))
			case 13, 23, 33:
				applyPointCodePair(&layout.UCSOrigin, 13, pair)
			case 16:
				layout.UCSXAxis.X = doubleValue(pair)
			case 26:
				layout.UCSXAxis.Y = doubleValue(pair)
			case 36:
				layout.UCSXAxis.Z = doubleValue(pair)
			case 17:
				layout.UCSYAxis.X = doubleValue(pair)
			case 27:
				layout.UCSYAxis.Y = doubleValue(pair)
			case 37:
				layout.UCSYAxis.Z = doubleValue(pair)
			case 76:
				layout.UCSOrthographicType = shortValue(pair)
			case 345:
				layout.NamedUCSHandle = handleFromString(stringValue(pair))
			case 346:
				layout.BaseUCSHandle = handleFromString(stringValue(pair))
			}
		} else if subclass == "AcDbPlotSettings" {
			switch pair.Code {
			case 1:
				layout.PageSetupName = stringValue(pair)
			case 2:
				layout.PlotConfigurationName = stringValue(pair)
			case 4:
				layout.PaperSizeName = stringValue(pair)
			case 6:
				layout.PlotViewName = stringValue(pair)
			case 7:
				layout.PlotStyleSheetName = stringValue(pair)
			case 40:
				layout.MarginLeft = doubleValue(pair)
			case 41:
				layout.MarginBottom = doubleValue(pair)
			case 42:
				layout.MarginRight = doubleValue(pair)
			case 43:
				layout.MarginTop = doubleValue(pair)
			case 44:
				layout.PaperWidth = doubleValue(pair)
			case 45:
				layout.PaperHeight = doubleValue(pair)
			case 46:
				layout.PlotOrigin[0] = doubleValue(pair)
			case 47:
				layout.PlotOrigin[1] = doubleValue(pair)
			case 48:
				layout.PlotWindowMin[0] = doubleValue(pair)
			case 49:
				layout.PlotWindowMin[1] = doubleValue(pair)
			case 140:
				layout.PlotWindowMax[0] = doubleValue(pair)
			case 141:
				layout.PlotWindowMax[1] = doubleValue(pair)
			case 142:
				layout.CustomScaleNumerator = doubleValue(pair)
			case 143:
				layout.CustomScaleDenom = doubleValue(pair)
			case 70:
				layout.PlotLayoutFlags = int(shortValue(pair))
			case 72:
				layout.PaperUnits = shortValue(pair)
			case 73:
				layout.PlotRotation = shortValue(pair)
			case 74:
				layout.PlotType = shortValue(pair)
			case 75:
				layout.StandardScaleType = shortValue(pair)
			case 147:
				layout.ScaleFactor = doubleValue(pair)
			}
		}
	}
	return
}

func parseImageDefinition(pairs []CodePair) (definition ImageDefinition) {
	definition.XData = xdataFromPairs(pairs)
	inApplicationGroup := false
	for _, pair := range pairs {
		switch {
		case pair.Code == 102:
			inApplicationGroup = strings.HasPrefix(stringValue(pair), "{")
		case inApplicationGroup:
		case pair.Code == 5:
			definition.Handle = handleFromString(stringValue(pair))
		case pair.Code == 90:
			definition.ClassVersion = intValue(pair)
		case pair.Code == 1:
			definition.FileName = stringValue(pair)
		case pair.Code == 10:
			definition.ImageSize.X = doubleValue(pair)
		case pair.Code == 20:
			definition.ImageSize.Y = doubleValue(pair)
		case pair.Code == 11:
			definition.PixelSize.X = doubleValue(pair)
		case pair.Code == 21:
			definition.PixelSize.Y = doubleValue(pair)
		case pair.Code == 280:
			definition.IsLoaded = shortValue(pair) != 0
		case pair.Code == 281:
			definition.ResolutionUnits = shortValue(pair)
		}
	}
	return
}

// IsModel reports whether this is the model tab rather than a paper space layout.
func (l *Layout) IsModel() bool {
	return l.TabOrder == 0 || strings.EqualFold(l.Name, "Model")
}

// SheetRectangle returns the sheet's outline in the layout's paper space coordinates. Paper space (0,0) is at the
// lower-left corner of the printable area, i.e. inside the margins and moved by the plot origin; the paper is turned
// by PlotRotation, and its size is converted to inches when PaperUnits is 0. ok is false if the paper size isn't set.
// Only unrotated layouts have been checked against real files.
func (l *Layout) SheetRectangle() (minimum, maximum Point, ok bool) {
	if l.PaperWidth <= 0 || l.PaperHeight <= 0 {
		return Point{}, Point{}, false
	}

	width, height := l.PaperWidth, l.PaperHeight
	left, bottom := l.MarginLeft, l.MarginBottom
	switch l.PlotRotation {
	case 1:
		// turned 90 degrees counter-clockwise: the layout's left edge lies on the paper's bottom edge
		width, height = height, width
		left, bottom = l.MarginBottom, l.MarginRight
	case 2:
		left, bottom = l.MarginRight, l.MarginTop
	case 3:
		width, height = height, width
		left, bottom = l.MarginTop, l.MarginLeft
	}

	scale := 1.0
	if l.PaperUnits == 0 {
		scale = 1 / 25.4
	}
	minimum = Point{-(left + l.PlotOrigin[0]) * scale, -(bottom + l.PlotOrigin[1]) * scale, 0}
	maximum = Point{minimum.X + width*scale, minimum.Y + height*scale, 0}
	return minimum, maximum, true
}

// LayoutByName returns the layout with the given name (case-insensitive), or nil.
func (d *Drawing) LayoutByName(name string) *Layout {
	for i := range d.Layouts {
		if strings.EqualFold(d.Layouts[i].Name, name) {
			return &d.Layouts[i]
		}
	}
	return nil
}

// LayoutEntities returns the entities drawn on a layout. The model tab's entities are the model space entities of
// Entities. The active layout's entities are the paper space entities of Entities; every other layout's entities live
// in its own *Paper_Space<n> block. nil is returned if the layout's block can't be found.
func (d *Drawing) LayoutEntities(layout *Layout) (entities []Entity) {
	blockName := ""
	for i := range d.BlockRecords {
		record := &d.BlockRecords[i]
		if (layout.BlockRecordHandle != 0 && record.Handle() == layout.BlockRecordHandle) ||
			(layout.Handle != 0 && handleFromString(record.LayoutHandle) == layout.Handle) {
			blockName = record.Name
			break
		}
	}
	if blockName == "" {
		if !layout.IsModel() {
			return nil
		}
		blockName = "*Model_Space"
	}

	isModelSpace := strings.EqualFold(blockName, "*Model_Space")
	if isModelSpace || strings.EqualFold(blockName, "*Paper_Space") {
		for _, e := range d.Entities {
			if e.IsInPaperSpace() != isModelSpace {
				entities = append(entities, e)
			}
		}
		return
	}
	if block := d.BlockByName(blockName); block != nil {
		return block.Entities
	}
	return nil
}

// ImageDefinition returns the IMAGEDEF that an IMAGE (or another raster image entity) shows, or nil.
func (d *Drawing) ImageDefinition(image RasterImage) *ImageDefinition {
	handle := handleFromString(image.imageDefinitionHandle())
	if handle == 0 {
		return nil
	}
	for i := range d.ImageDefinitions {
		if d.ImageDefinitions[i].Handle == handle {
			return &d.ImageDefinitions[i]
		}
	}
	return nil
}

package dxf

// RasterVariables is the RASTERVARIABLES object: the drawing-wide settings of raster images (IMAGE entities).
type RasterVariables struct {
	Handle       Handle
	ClassVersion int // code 90
	// ImageFrame is the IMAGEFRAME setting, code 70: 0 = no frame, 1 = frame shown and plotted, 2 = frame shown but
	// not plotted.
	ImageFrame           int16
	IsHighDisplayQuality bool  // code 71
	ImageUnits           int16 // code 72: 0 = none, 1 = mm, 2 = cm, 3 = m, 4 = km, 5 = in, 6 = ft, 7 = yd, 8 = mi
	XData                XData // extended data (1001…)
}

// IsFramePlotted reports whether the outlines of images are plotted.
func (v *RasterVariables) IsFramePlotted() bool { return v.ImageFrame == 1 }

// WipeoutVariables is the WIPEOUTVARIABLES object: the drawing-wide settings of WIPEOUT entities.
type WipeoutVariables struct {
	Handle       Handle
	ClassVersion int // code 90
	// Frame is the WIPEOUTFRAME setting, code 70: 0 = no frame, 1 = frame shown and plotted, 2 = frame shown but not
	// plotted.
	Frame int16
	XData XData // extended data (1001…)
}

// IsFramePlotted reports whether the outlines of wipeouts are plotted.
func (v *WipeoutVariables) IsFramePlotted() bool { return v.Frame == 1 }

// ObjectColor is a colour as MLEADERSTYLE and MULTILEADER store it (codes 91–94): the high byte is the kind, 0xC0 =
// BYLAYER, 0xC1 = BYBLOCK, 0xC2 = true colour (RGB in the low 24 bits), 0xC3 = ACI (in the low byte).
type ObjectColor int32

// IsByLayer reports whether the colour is BYLAYER.
func (c ObjectColor) IsByLayer() bool { return uint32(c)>>24 == 0xC0 }

// IsByBlock reports whether the colour is BYBLOCK.
func (c ObjectColor) IsByBlock() bool { return uint32(c)>>24 == 0xC1 }

// TrueColor returns the RGB value (0xRRGGBB) of a true colour.
func (c ObjectColor) TrueColor() (rgb int, ok bool) {
	return int(uint32(c) & 0xFFFFFF), uint32(c)>>24 == 0xC2
}

// ACI returns the AutoCAD Color Index of an indexed colour.
func (c ObjectColor) ACI() (color Color, ok bool) {
	return Color(uint32(c) & 0xFF), uint32(c)>>24 == 0xC3
}

// MLeaderStyle is an MLEADERSTYLE object: the defaults of the MULTILEADER entities that use it. Its name is the key
// of its entry in the ACAD_MLEADERSTYLE dictionary.
type MLeaderStyle struct {
	Handle      Handle
	Description string // code 3

	ContentType                   int16   // code 170: 0 = none, 1 = block, 2 = MTEXT, 3 = tolerance
	DrawMLeaderOrder              int16   // code 171
	DrawLeaderOrder               int16   // code 172
	MaxLeaderSegmentPoints        int     // code 90
	FirstSegmentAngleConstraint   float64 // code 40
	SecondSegmentAngleConstraint  float64 // code 41
	LeaderLineType                int16   // code 173: 0 = invisible, 1 = straight, 2 = spline
	LeaderLineColor               ObjectColor
	LeaderLineTypeHandle          Handle     // code 340, the LTYPE
	LeaderLineWeight              LineWeight // code 92
	IsLandingEnabled              bool       // code 290
	LandingGap                    float64    // code 42
	IsDoglegEnabled               bool       // code 291
	DoglegLength                  float64    // code 43
	ArrowheadHandle               Handle     // code 341, the arrowhead's BLOCK_RECORD; 0 = closed filled
	ArrowheadSize                 float64    // code 44
	DefaultMTextContents          string     // code 300
	TextStyleHandle               Handle     // code 342, the STYLE
	TextLeftAttachment            int16      // code 174
	TextAngleType                 int16      // code 175
	TextAlignment                 int16      // code 176
	TextRightAttachment           int16      // code 178
	TextColor                     ObjectColor
	TextHeight                    float64 // code 45
	IsTextFrameEnabled            bool    // code 292
	IsTextAlwaysLeftAligned       bool    // code 297
	AlignGap                      float64 // code 46
	BlockContentHandle            Handle  // code 343, the BLOCK_RECORD
	BlockContentColor             ObjectColor
	BlockContentScale             Vector  // codes 47/49/140
	IsBlockContentScaleEnabled    bool    // code 293
	BlockContentRotation          float64 // code 141, radians
	IsBlockContentRotationEnabled bool    // code 294
	BlockContentConnection        int16   // code 177: 0 = extents, 1 = base point
	Scale                         float64 // code 142, the overall scale; 0 = annotative
	IsOverwritePropertyValue      bool    // code 295
	IsAnnotative                  bool    // code 296
	BreakGapSize                  float64 // code 143
	TextAttachmentDirection       int16   // code 271: 0 = horizontal, 1 = vertical
	BottomTextAttachment          int16   // code 272
	TopTextAttachment             int16   // code 273
	XData                         XData   // extended data (1001…), e.g. ACAD_MLEADERVER
}

// TableStyle is a TABLESTYLE object: the defaults of the ACAD_TABLE entities that use it. Its name is the key of its
// entry in the ACAD_TABLESTYLE dictionary.
type TableStyle struct {
	Handle                    Handle
	Version                   int16   // the first code 280
	Description               string  // code 3
	FlowDirection             int16   // code 70: 0 = down, 1 = up
	Flags                     int16   // code 71
	HorizontalCellMargin      float64 // code 40
	VerticalCellMargin        float64 // code 41
	IsTitleSuppressed         bool    // the second code 280
	IsColumnHeadingSuppressed bool    // code 281
	// CellStyles are the data, column header and title cell styles, in that order.
	CellStyles []TableCellStyle
	XData      XData // extended data (1001…)
}

// TableCellStyle is one of a TABLESTYLE's cell styles.
type TableCellStyle struct {
	TextStyleName string  // code 7
	TextHeight    float64 // code 140
	Alignment     int16   // code 170: 1 = top left … 9 = bottom right
	TextColor     Color   // code 62
	FillColor     Color   // code 63
	IsFillEnabled bool    // code 283
	DataType      int     // code 90
	UnitType      int     // code 91
	FormatString  string  // code 1
	// the six borders are codes 274–279 (line weight), 284–289 (visibility) and 64–69 (colour), in the same order
	BorderWeights [6]LineWeight
	BorderVisible [6]bool
	BorderColors  [6]Color
}

// MLeaderStyle returns the MLEADERSTYLE a MULTILEADER uses, or nil.
func (d *Drawing) MLeaderStyle(leader *MLeader) *MLeaderStyle {
	for i := range d.MLeaderStyles {
		if leader.StyleHandle != 0 && d.MLeaderStyles[i].Handle == leader.StyleHandle {
			return &d.MLeaderStyles[i]
		}
	}
	return nil
}

// TableStyle returns the TABLESTYLE an ACAD_TABLE uses, or nil.
func (d *Drawing) TableStyle(table *Table) *TableStyle {
	for i := range d.TableStyles {
		if table.StyleHandle != 0 && d.TableStyles[i].Handle == table.StyleHandle {
			return &d.TableStyles[i]
		}
	}
	return nil
}

func parseRasterVariables(pairs []CodePair) *RasterVariables {
	variables := &RasterVariables{XData: xdataFromPairs(pairs)}
	for _, pair := range pairs {
		switch pair.Code {
		case 5:
			variables.Handle = handleFromString(stringValue(pair))
		case 90:
			variables.ClassVersion = intValue(pair)
		case 70:
			variables.ImageFrame = shortValue(pair)
		case 71:
			variables.IsHighDisplayQuality = shortValue(pair) != 0
		case 72:
			variables.ImageUnits = shortValue(pair)
		}
	}
	return variables
}

func parseWipeoutVariables(pairs []CodePair) *WipeoutVariables {
	variables := &WipeoutVariables{XData: xdataFromPairs(pairs)}
	for _, pair := range pairs {
		switch pair.Code {
		case 5:
			variables.Handle = handleFromString(stringValue(pair))
		case 90:
			variables.ClassVersion = intValue(pair)
		case 70:
			variables.Frame = shortValue(pair)
		}
	}
	return variables
}

func parseMLeaderStyle(pairs []CodePair) (style MLeaderStyle) {
	style.XData = xdataFromPairs(pairs)
	for _, pair := range pairs {
		switch pair.Code {
		case 5:
			style.Handle = handleFromString(stringValue(pair))
		case 3:
			style.Description = stringValue(pair)
		case 170:
			style.ContentType = shortValue(pair)
		case 171:
			style.DrawMLeaderOrder = shortValue(pair)
		case 172:
			style.DrawLeaderOrder = shortValue(pair)
		case 90:
			style.MaxLeaderSegmentPoints = intValue(pair)
		case 40:
			style.FirstSegmentAngleConstraint = doubleValue(pair)
		case 41:
			style.SecondSegmentAngleConstraint = doubleValue(pair)
		case 173:
			style.LeaderLineType = shortValue(pair)
		case 91:
			style.LeaderLineColor = ObjectColor(intValue(pair))
		case 340:
			style.LeaderLineTypeHandle = handleFromString(stringValue(pair))
		case 92:
			style.LeaderLineWeight = LineWeight(intValue(pair))
		case 290:
			style.IsLandingEnabled = flagValue(pair)
		case 42:
			style.LandingGap = doubleValue(pair)
		case 291:
			style.IsDoglegEnabled = flagValue(pair)
		case 43:
			style.DoglegLength = doubleValue(pair)
		case 341:
			style.ArrowheadHandle = handleFromString(stringValue(pair))
		case 44:
			style.ArrowheadSize = doubleValue(pair)
		case 300:
			style.DefaultMTextContents = stringValue(pair)
		case 342:
			style.TextStyleHandle = handleFromString(stringValue(pair))
		case 174:
			style.TextLeftAttachment = shortValue(pair)
		case 175:
			style.TextAngleType = shortValue(pair)
		case 176:
			style.TextAlignment = shortValue(pair)
		case 178:
			style.TextRightAttachment = shortValue(pair)
		case 93:
			style.TextColor = ObjectColor(intValue(pair))
		case 45:
			style.TextHeight = doubleValue(pair)
		case 292:
			style.IsTextFrameEnabled = flagValue(pair)
		case 297:
			style.IsTextAlwaysLeftAligned = flagValue(pair)
		case 46:
			style.AlignGap = doubleValue(pair)
		case 343:
			style.BlockContentHandle = handleFromString(stringValue(pair))
		case 94:
			style.BlockContentColor = ObjectColor(intValue(pair))
		case 47:
			style.BlockContentScale.X = doubleValue(pair)
		case 49:
			style.BlockContentScale.Y = doubleValue(pair)
		case 140:
			style.BlockContentScale.Z = doubleValue(pair)
		case 293:
			style.IsBlockContentScaleEnabled = flagValue(pair)
		case 141:
			style.BlockContentRotation = doubleValue(pair)
		case 294:
			style.IsBlockContentRotationEnabled = flagValue(pair)
		case 177:
			style.BlockContentConnection = shortValue(pair)
		case 142:
			style.Scale = doubleValue(pair)
		case 295:
			style.IsOverwritePropertyValue = flagValue(pair)
		case 296:
			style.IsAnnotative = flagValue(pair)
		case 143:
			style.BreakGapSize = doubleValue(pair)
		case 271:
			style.TextAttachmentDirection = shortValue(pair)
		case 272:
			style.BottomTextAttachment = shortValue(pair)
		case 273:
			style.TopTextAttachment = shortValue(pair)
		}
	}
	return
}

func parseTableStyle(pairs []CodePair) (style TableStyle) {
	style.XData = xdataFromPairs(pairs)
	afterHeader := false // the second 280 (title suppressed) comes after the margins, the first (version) before
	var cell *TableCellStyle
	for _, pair := range pairs {
		if pair.Code == 7 {
			// every cell style starts with its text style
			style.CellStyles = append(style.CellStyles, TableCellStyle{})
			cell = &style.CellStyles[len(style.CellStyles)-1]
		}
		if cell != nil {
			parseTableCellStyle(cell, pair)
			continue
		}
		switch pair.Code {
		case 5:
			style.Handle = handleFromString(stringValue(pair))
		case 280:
			if afterHeader {
				style.IsTitleSuppressed = shortValue(pair) != 0
			} else {
				style.Version = shortValue(pair)
			}
		case 3:
			style.Description = stringValue(pair)
		case 70:
			style.FlowDirection = shortValue(pair)
			afterHeader = true
		case 71:
			style.Flags = shortValue(pair)
			afterHeader = true
		case 40:
			style.HorizontalCellMargin = doubleValue(pair)
			afterHeader = true
		case 41:
			style.VerticalCellMargin = doubleValue(pair)
			afterHeader = true
		case 281:
			style.IsColumnHeadingSuppressed = shortValue(pair) != 0
		}
	}
	return
}

func parseTableCellStyle(cell *TableCellStyle, pair CodePair) {
	switch code := pair.Code; {
	case code == 7:
		cell.TextStyleName = stringValue(pair)
	case code == 140:
		cell.TextHeight = doubleValue(pair)
	case code == 170:
		cell.Alignment = shortValue(pair)
	case code == 62:
		cell.TextColor = Color(shortValue(pair))
	case code == 63:
		cell.FillColor = Color(shortValue(pair))
	case code == 283:
		cell.IsFillEnabled = shortValue(pair) != 0
	case code == 90:
		cell.DataType = intValue(pair)
	case code == 91:
		cell.UnitType = intValue(pair)
	case code == 1:
		cell.FormatString = stringValue(pair)
	case code >= 274 && code <= 279:
		cell.BorderWeights[code-274] = LineWeight(shortValue(pair))
	case code >= 284 && code <= 289:
		cell.BorderVisible[code-284] = shortValue(pair) != 0
	case code >= 64 && code <= 69:
		cell.BorderColors[code-64] = Color(shortValue(pair))
	}
}

// flagValue reads a flag that is a bool (codes 290–299) or a short.
func flagValue(pair CodePair) bool {
	return boolValue(pair) || shortValue(pair) != 0
}

package dxf

import (
	"testing"
)

func objectsDrawing(t *testing.T, objects ...CodePair) Drawing {
	pairs := []CodePair{NewStringCodePair(0, "SECTION"), NewStringCodePair(2, "OBJECTS")}
	pairs = append(pairs, objects...)
	pairs = append(pairs, NewStringCodePair(0, "ENDSEC"), NewStringCodePair(0, "EOF"))
	return parseFromCodePairs(t, pairs...)
}

func TestReadRasterAndWipeoutVariables(t *testing.T) {
	drawing := objectsDrawing(t,
		NewStringCodePair(0, "RASTERVARIABLES"), NewStringCodePair(5, "2A"),
		NewStringCodePair(100, "AcDbRasterVariables"),
		NewIntCodePair(90, 0), NewShortCodePair(70, 2), NewShortCodePair(71, 1), NewShortCodePair(72, 3),
		NewStringCodePair(0, "WIPEOUTVARIABLES"), NewStringCodePair(5, "2B"),
		NewStringCodePair(102, "{ACAD_REACTORS"), NewStringCodePair(330, "C"), NewStringCodePair(102, "}"),
		NewStringCodePair(330, "C"),
		NewStringCodePair(100, "AcDbWipeoutVariables"), NewShortCodePair(70, 1),
	)

	raster := drawing.RasterVariables
	assert(t, raster != nil, "expected RASTERVARIABLES")
	assertEqInt(t, 0x2A, int(raster.Handle))
	assertEqInt(t, 2, int(raster.ImageFrame))
	assertEqBool(t, false, raster.IsFramePlotted())
	assertEqBool(t, true, raster.IsHighDisplayQuality)
	assertEqInt(t, 3, int(raster.ImageUnits))

	wipeout := drawing.WipeoutVariables
	assert(t, wipeout != nil, "expected WIPEOUTVARIABLES")
	assertEqInt(t, 0x2B, int(wipeout.Handle))
	assertEqBool(t, true, wipeout.IsFramePlotted())

	empty := objectsDrawing(t)
	assert(t, empty.RasterVariables == nil && empty.WipeoutVariables == nil, "expected no variables objects")
}

func TestReadMLeaderStyle(t *testing.T) {
	drawing := objectsDrawing(t,
		NewStringCodePair(0, "MLEADERSTYLE"), NewStringCodePair(5, "1A"),
		NewStringCodePair(100, "AcDbMLeaderStyle"),
		NewShortCodePair(170, 2),
		NewIntCodePair(90, 2),
		NewShortCodePair(173, 1),
		NewIntCodePair(91, -1056964608), // 0xC1000000: BYBLOCK
		NewStringCodePair(340, "14"),
		NewIntCodePair(92, -2),
		NewBoolCodePair(290, true),
		NewDoubleCodePair(42, 0.09),
		NewBoolCodePair(291, true),
		NewDoubleCodePair(43, 0.36),
		NewStringCodePair(3, "Standard"),
		NewDoubleCodePair(44, 0.18),
		NewStringCodePair(342, "11"),
		NewIntCodePair(93, -1040187392+0x123456), // 0xC2123456: RGB
		NewDoubleCodePair(45, 0.25),
		NewIntCodePair(94, -1023410171), // 0xC3000005: ACI 5
		NewDoubleCodePair(47, 1.5), NewDoubleCodePair(49, 2.0), NewDoubleCodePair(140, 1.0),
		NewDoubleCodePair(142, 50.0),
		NewShortCodePair(272, 9),
	)
	assertEqInt(t, 1, len(drawing.MLeaderStyles))
	style := drawing.MLeaderStyles[0]
	assertEqInt(t, 0x1A, int(style.Handle))
	assertEqString(t, "Standard", style.Description)
	assertEqInt(t, 2, int(style.ContentType))
	assertEqInt(t, 2, style.MaxLeaderSegmentPoints)
	assertEqInt(t, 1, int(style.LeaderLineType))
	assertEqBool(t, true, style.LeaderLineColor.IsByBlock())
	assertEqInt(t, 0x14, int(style.LeaderLineTypeHandle))
	assertEqBool(t, true, style.LeaderLineWeight.ByBlock())
	assertEqBool(t, true, style.IsLandingEnabled)
	assertEqFloat64(t, 0.09, style.LandingGap)
	assertEqBool(t, true, style.IsDoglegEnabled)
	assertEqFloat64(t, 0.36, style.DoglegLength)
	assertEqFloat64(t, 0.18, style.ArrowheadSize)
	assertEqInt(t, 0x11, int(style.TextStyleHandle))
	rgb, ok := style.TextColor.TrueColor()
	assertEqBool(t, true, ok)
	assertEqInt(t, 0x123456, rgb)
	aci, ok := style.BlockContentColor.ACI()
	assertEqBool(t, true, ok)
	assertEqInt(t, 5, int(aci))
	assertEqFloat64(t, 0.25, style.TextHeight)
	assertEqVector(t, Vector{1.5, 2.0, 1.0}, style.BlockContentScale)
	assertEqFloat64(t, 50.0, style.Scale)
	assertEqInt(t, 9, int(style.BottomTextAttachment))

	// a MULTILEADER finds its style through code 340 after the context data
	leader := parseEntity(t, "MULTILEADER", mleaderPairs()...).(*MLeader)
	assertEqInt(t, 0x1A, int(leader.StyleHandle))
	assert(t, drawing.MLeaderStyle(leader) == &drawing.MLeaderStyles[0], "expected the leader's style")
	leader.StyleHandle = 0x99
	assert(t, drawing.MLeaderStyle(leader) == nil, "expected no style")
}

func tableCellStylePairs(name string, height float64, alignment int16) []CodePair {
	pairs := []CodePair{
		NewStringCodePair(7, name), NewDoubleCodePair(140, height), NewShortCodePair(170, alignment),
		NewShortCodePair(62, 0), NewShortCodePair(63, 7), NewShortCodePair(283, 1),
		NewIntCodePair(90, 4), NewIntCodePair(91, 0), NewStringCodePair(1, ""),
	}
	for i := int16(0); i < 6; i++ {
		pairs = append(pairs,
			NewShortCodePair(274+int(i), -2+i), NewShortCodePair(284+int(i), i%2), NewShortCodePair(64+int(i), i+1))
	}
	return pairs
}

func TestReadTableStyle(t *testing.T) {
	pairs := []CodePair{
		NewStringCodePair(0, "TABLESTYLE"), NewStringCodePair(5, "7A"),
		NewStringCodePair(102, "{ACAD_XDICTIONARY"), NewStringCodePair(360, "80"), NewStringCodePair(102, "}"),
		NewStringCodePair(100, "AcDbTableStyle"),
		NewShortCodePair(280, 0),
		NewStringCodePair(3, "Standard"),
		NewShortCodePair(70, 1),
		NewShortCodePair(71, 0),
		NewDoubleCodePair(40, 0.06),
		NewDoubleCodePair(41, 0.07),
		NewShortCodePair(280, 1),
		NewShortCodePair(281, 0),
	}
	pairs = append(pairs, tableCellStylePairs("Data", 0.18, 2)...)
	pairs = append(pairs, tableCellStylePairs("Header", 0.25, 5)...)
	pairs = append(pairs, tableCellStylePairs("Title", 0.35, 5)...)
	drawing := objectsDrawing(t, pairs...)

	assertEqInt(t, 1, len(drawing.TableStyles))
	style := drawing.TableStyles[0]
	assertEqInt(t, 0x7A, int(style.Handle))
	assertEqInt(t, 0, int(style.Version))
	assertEqString(t, "Standard", style.Description)
	assertEqInt(t, 1, int(style.FlowDirection))
	assertEqFloat64(t, 0.06, style.HorizontalCellMargin)
	assertEqFloat64(t, 0.07, style.VerticalCellMargin)
	assertEqBool(t, true, style.IsTitleSuppressed)
	assertEqBool(t, false, style.IsColumnHeadingSuppressed)

	assertEqInt(t, 3, len(style.CellStyles))
	data, header, title := style.CellStyles[0], style.CellStyles[1], style.CellStyles[2]
	assertEqString(t, "Data", data.TextStyleName)
	assertEqFloat64(t, 0.18, data.TextHeight)
	assertEqInt(t, 2, int(data.Alignment))
	assertEqInt(t, 7, int(data.FillColor))
	assertEqBool(t, true, data.IsFillEnabled)
	assertEqInt(t, 4, data.DataType)
	assertEqInt(t, -2, int(data.BorderWeights[0]))
	assertEqInt(t, 3, int(data.BorderWeights[5]))
	assertEqBool(t, false, data.BorderVisible[0])
	assertEqBool(t, true, data.BorderVisible[1])
	assertEqInt(t, 6, int(data.BorderColors[5]))
	assertEqString(t, "Header", header.TextStyleName)
	assertEqFloat64(t, 0.25, header.TextHeight)
	assertEqString(t, "Title", title.TextStyleName)
	assertEqFloat64(t, 0.35, title.TextHeight)

	// an ACAD_TABLE finds its style through code 342
	table := parseEntity(t, "ACAD_TABLE", tablePairs()...).(*Table)
	assertEqInt(t, 0x7A, int(table.StyleHandle))
	assert(t, drawing.TableStyle(table) == &drawing.TableStyles[0], "expected the table's style")
}

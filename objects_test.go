package dxf

import (
	"testing"
)

func layoutDrawing(t *testing.T) Drawing {
	return parseFromCodePairs(t,
		NewStringCodePair(0, "SECTION"), NewStringCodePair(2, "TABLES"),
		NewStringCodePair(0, "TABLE"), NewStringCodePair(2, "BLOCK_RECORD"),
		NewStringCodePair(0, "BLOCK_RECORD"), NewStringCodePair(5, "1F"), NewStringCodePair(2, "*Model_Space"), NewStringCodePair(340, "22"),
		NewStringCodePair(0, "BLOCK_RECORD"), NewStringCodePair(5, "1B"), NewStringCodePair(2, "*Paper_Space"), NewStringCodePair(340, "1E"),
		NewStringCodePair(0, "BLOCK_RECORD"), NewStringCodePair(5, "1C"), NewStringCodePair(2, "*Paper_Space0"),
		NewStringCodePair(0, "ENDTAB"),
		NewStringCodePair(0, "ENDSEC"),

		NewStringCodePair(0, "SECTION"), NewStringCodePair(2, "BLOCKS"),
		NewStringCodePair(0, "BLOCK"), NewStringCodePair(2, "*Paper_Space0"), NewShortCodePair(70, 0),
		NewDoubleCodePair(10, 0), NewDoubleCodePair(20, 0), NewDoubleCodePair(30, 0),
		NewStringCodePair(0, "LINE"), NewStringCodePair(8, "on-sheet-b"), NewShortCodePair(67, 1),
		NewStringCodePair(0, "ENDBLK"),
		NewStringCodePair(0, "ENDSEC"),

		NewStringCodePair(0, "SECTION"), NewStringCodePair(2, "ENTITIES"),
		NewStringCodePair(0, "LINE"), NewStringCodePair(8, "model"),
		NewStringCodePair(0, "LINE"), NewStringCodePair(8, "on-sheet-a"), NewShortCodePair(67, 1),
		NewStringCodePair(0, "IMAGE"), NewShortCodePair(67, 1), NewStringCodePair(340, "30"),
		NewStringCodePair(0, "ENDSEC"),

		NewStringCodePair(0, "SECTION"), NewStringCodePair(2, "OBJECTS"),
		NewStringCodePair(0, "DICTIONARY"), NewStringCodePair(5, "C"), NewStringCodePair(3, "ACAD_LAYOUT"), NewStringCodePair(350, "1A"),
		NewStringCodePair(0, "LAYOUT"), NewStringCodePair(5, "22"),
		NewStringCodePair(102, "{ACAD_REACTORS"), NewStringCodePair(330, "1A"), NewStringCodePair(102, "}"),
		NewStringCodePair(330, "1A"),
		NewStringCodePair(100, "AcDbPlotSettings"), NewStringCodePair(1, ""), NewDoubleCodePair(44, 215.9), NewDoubleCodePair(45, 279.4), NewShortCodePair(72, 0),
		NewStringCodePair(100, "AcDbLayout"), NewStringCodePair(1, "Model"), NewShortCodePair(70, 1), NewShortCodePair(71, 0), NewStringCodePair(330, "1F"),
		NewStringCodePair(0, "LAYOUT"), NewStringCodePair(5, "1E"),
		NewStringCodePair(330, "1A"),
		NewStringCodePair(100, "AcDbPlotSettings"), NewStringCodePair(1, "setup"), NewStringCodePair(2, "none_device"),
		NewDoubleCodePair(40, 5), NewDoubleCodePair(41, 10), NewDoubleCodePair(42, 15), NewDoubleCodePair(43, 20),
		NewDoubleCodePair(44, 420), NewDoubleCodePair(45, 297), NewDoubleCodePair(46, 1), NewDoubleCodePair(47, 2),
		NewShortCodePair(70, 688), NewShortCodePair(72, 1), NewShortCodePair(73, 0), NewShortCodePair(74, 5),
		NewStringCodePair(100, "AcDbLayout"), NewStringCodePair(1, "Sheet A"), NewShortCodePair(70, 0), NewShortCodePair(71, 1),
		NewDoubleCodePair(14, 1), NewDoubleCodePair(24, 2), NewDoubleCodePair(34, 0),
		NewStringCodePair(330, "1B"), NewStringCodePair(331, "5E"),
		NewStringCodePair(0, "LAYOUT"), NewStringCodePair(5, "1D"),
		NewStringCodePair(100, "AcDbPlotSettings"), NewDoubleCodePair(44, 297), NewDoubleCodePair(45, 210), NewShortCodePair(72, 1),
		NewStringCodePair(100, "AcDbLayout"), NewStringCodePair(1, "Sheet B"), NewShortCodePair(71, 2), NewStringCodePair(330, "1C"),
		NewStringCodePair(0, "IMAGEDEF"), NewStringCodePair(5, "30"),
		NewStringCodePair(102, "{ACAD_REACTORS"), NewStringCodePair(330, "31"), NewStringCodePair(102, "}"),
		NewStringCodePair(330, "31"),
		NewStringCodePair(100, "AcDbRasterImageDef"), NewIntCodePair(90, 0), NewStringCodePair(1, "images/photo.jpg"),
		NewDoubleCodePair(10, 847), NewDoubleCodePair(20, 336), NewDoubleCodePair(11, 0.5), NewDoubleCodePair(21, 0.25),
		NewShortCodePair(280, 1), NewShortCodePair(281, 2),
		NewStringCodePair(0, "XRECORD"), NewStringCodePair(5, "40"), NewStringCodePair(1, "not a layout"),
		NewStringCodePair(0, "ENDSEC"),
		NewStringCodePair(0, "EOF"),
	)
}

func TestReadLayouts(t *testing.T) {
	d := layoutDrawing(t)
	assertEqInt(t, 3, len(d.Layouts))

	model := d.LayoutByName("model")
	assertEqBool(t, true, model.IsModel())
	assertEqUInt64(t, 0x22, uint64(model.Handle))
	assertEqUInt64(t, 0x1F, uint64(model.BlockRecordHandle))

	sheet := d.LayoutByName("Sheet A")
	assertEqBool(t, false, sheet.IsModel())
	assertEqString(t, "setup", sheet.PageSetupName)
	assertEqString(t, "none_device", sheet.PlotConfigurationName)
	assertEqFloat64(t, 420, sheet.PaperWidth)
	assertEqFloat64(t, 297, sheet.PaperHeight)
	assertEqFloat64(t, 15, sheet.MarginRight)
	assertEqInt(t, 688, sheet.PlotLayoutFlags)
	assertEqInt(t, 0, sheet.Flags)
	assertEqInt(t, 1, sheet.TabOrder)
	assertEqInt(t, 5, int(sheet.PlotType))
	assertEqPoint(t, Point{1, 2, 0}, sheet.ExtentsMin)
	assertEqUInt64(t, 0x1B, uint64(sheet.BlockRecordHandle))
	assertEqUInt64(t, 0x5E, uint64(sheet.LastActiveViewportHandle))
}

func TestLayoutSheetRectangle(t *testing.T) {
	d := layoutDrawing(t)
	minimum, maximum, ok := d.LayoutByName("Sheet A").SheetRectangle()
	assertEqBool(t, true, ok)
	// paper space (0,0) is inside the left/bottom margins, moved by the plot origin
	assertNearPoint(t, Point{-6, -12, 0}, minimum)
	assertNearPoint(t, Point{414, 285, 0}, maximum)

	rotated := Layout{PaperWidth: 297, PaperHeight: 210, PaperUnits: 1, PlotRotation: 1, MarginLeft: 1, MarginBottom: 2, MarginRight: 3, MarginTop: 4}
	minimum, maximum, _ = rotated.SheetRectangle()
	assertNearPoint(t, Point{-2, -3, 0}, minimum)
	assertNearPoint(t, Point{208, 294, 0}, maximum)

	inches := Layout{PaperWidth: 254, PaperHeight: 127}
	_, maximum, _ = inches.SheetRectangle()
	assertNearPoint(t, Point{10, 5, 0}, maximum)

	_, _, ok = (&Layout{}).SheetRectangle()
	assertEqBool(t, false, ok)
}

func TestLayoutEntities(t *testing.T) {
	d := layoutDrawing(t)
	layers := func(entities []Entity) (names []string) {
		for _, e := range entities {
			names = append(names, e.Layer())
		}
		return
	}

	model := layers(d.LayoutEntities(d.LayoutByName("Model")))
	assertEqInt(t, 1, len(model))
	assertEqString(t, "model", model[0])

	// the active layout: the paper space entities of Entities
	active := d.LayoutEntities(d.LayoutByName("Sheet A"))
	assertEqInt(t, 2, len(active))
	assertEqString(t, "on-sheet-a", active[0].Layer())

	// another layout: its *Paper_Space0 block
	other := layers(d.LayoutEntities(d.LayoutByName("Sheet B")))
	assertEqInt(t, 1, len(other))
	assertEqString(t, "on-sheet-b", other[0])

	assertEqInt(t, 0, len(d.LayoutEntities(&Layout{Name: "missing", TabOrder: 3})))
}

func TestReadImageDefinition(t *testing.T) {
	d := layoutDrawing(t)
	assertEqInt(t, 1, len(d.ImageDefinitions))
	image := d.Entities[2].(*Image)
	definition := d.ImageDefinition(image)
	if definition == nil {
		t.Fatal("the image definition was not found")
	}
	assertEqString(t, "images/photo.jpg", definition.FileName)
	assertEqVector(t, Vector{847, 336, 0}, definition.ImageSize)
	assertEqVector(t, Vector{0.5, 0.25, 0}, definition.PixelSize)
	assertEqBool(t, true, definition.IsLoaded)
	assertEqInt(t, 2, int(definition.ResolutionUnits))
	assertEqUInt64(t, 0x30, uint64(definition.Handle))

	assertEqBool(t, true, d.ImageDefinition(NewImage()) == nil)
}

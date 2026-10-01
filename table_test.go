package dxf

import (
	"reflect"
	"testing"
)

func tablePairs(cells ...CodePair) []CodePair {
	pairs := []CodePair{
		NewStringCodePair(100, "AcDbEntity"),
		NewStringCodePair(8, "TABLES"),
		NewStringCodePair(100, "AcDbBlockReference"),
		NewStringCodePair(2, "*T1"),
		NewDoubleCodePair(10, 10.0),
		NewDoubleCodePair(20, 20.0),
		NewDoubleCodePair(30, 0.0),
		NewStringCodePair(100, "AcDbTable"),
		NewShortCodePair(280, 0),
		NewStringCodePair(342, "7A"),
		NewStringCodePair(343, "7B"),
		NewDoubleCodePair(11, 1.0),
		NewDoubleCodePair(21, 0.0),
		NewDoubleCodePair(31, 0.0),
		NewIntCodePair(90, 22),
		NewIntCodePair(91, 2),
		NewIntCodePair(92, 2),
		NewDoubleCodePair(141, 5.0),
		NewDoubleCodePair(141, 6.0),
		NewDoubleCodePair(142, 30.0),
		NewDoubleCodePair(142, 40.0),
	}
	return append(pairs, cells...)
}

func TestReadR2004Table(t *testing.T) {
	table := parseEntity(t, "ACAD_TABLE", tablePairs(
		NewShortCodePair(171, 1),
		NewIntCodePair(91, 0), // cell data; not the row count
		NewStringCodePair(1, "Előtér"),
		NewShortCodePair(171, 1),
		NewStringCodePair(2, "long te"),
		NewStringCodePair(1, "xt"),
		NewShortCodePair(171, 1),
		NewShortCodePair(171, 1),
		NewStringCodePair(1, "B2"),
	)...).(*Table)
	assertEqString(t, "TABLES", table.Layer())
	assertEqString(t, "*T1", table.BlockName)
	assertEqPoint(t, Point{10, 20, 0}, table.InsertionPoint)
	assertEqVector(t, Vector{1, 0, 0}, table.HorizontalDirection)
	assertEqInt(t, 2, table.RowCount)
	assertEqInt(t, 2, table.ColumnCount)
	assert(t, reflect.DeepEqual([]float64{5, 6}, table.RowHeights), "unexpected row heights")
	assert(t, reflect.DeepEqual([]float64{30, 40}, table.ColumnWidths), "unexpected column widths")
	assert(t, reflect.DeepEqual([]string{"Előtér", "long text", "", "B2"}, table.CellTexts), "unexpected cell texts")
	assertEqString(t, "B2", table.CellText(1, 1))
	assertEqString(t, "", table.CellText(2, 0))
	assertEqString(t, "", table.CellText(0, 2))
}

func TestReadR2007Table(t *testing.T) {
	table := parseEntity(t, "ACAD_TABLE", tablePairs(
		NewShortCodePair(171, 1),
		NewStringCodePair(301, "CELL_VALUE"),
		NewIntCodePair(93, 0),
		NewStringCodePair(302, "first"),
		NewStringCodePair(304, "ACVALUE_END"),
		NewShortCodePair(171, 1),
		NewStringCodePair(301, "CELL_VALUE"),
		NewStringCodePair(302, "second"),
	)...).(*Table)
	assert(t, reflect.DeepEqual([]string{"first", "second"}, table.CellTexts), "unexpected cell texts")
}

func TestWriteTableAsRead(t *testing.T) {
	pairs := tablePairs(NewShortCodePair(171, 1), NewStringCodePair(1, "A1"))
	table := parseEntity(t, "ACAD_TABLE", pairs...).(*Table)
	written := allCodePairs(table, R2018)
	// everything after the common entity data is written as it was read
	assertContainsCodePairs(t, pairs[2:], written)
	assertContainsCodePairs(t, []CodePair{NewStringCodePair(0, "ACAD_TABLE")}, written)

	// ACAD_TABLE exists since AutoCAD 2004
	assertNotContainsCodePairs(t, []CodePair{NewStringCodePair(0, "ACAD_TABLE")}, drawingCodePairsFromEntity(t, table, R2000))
	// a table without read data is never written
	assertNotContainsCodePairs(t, []CodePair{NewStringCodePair(0, "ACAD_TABLE")}, drawingCodePairsFromEntity(t, newTable(), R2018))
}

func TestExplodeTable(t *testing.T) {
	drawing := *NewDrawing()
	line := lineEntity(Point{0, 0, 0}, Point{30, 0, 0})
	drawing.Blocks = append(drawing.Blocks, Block{Name: "*T1", Flags: 1, Entities: []Entity{line}})
	table := parseEntity(t, "ACAD_TABLE", tablePairs()...).(*Table)
	drawing.Entities = append(drawing.Entities, table)

	visits, issues := walkAll(t, &drawing, WalkOptions{})
	assertEqInt(t, 0, len(issues))
	lines := visitsOf[*Line](visits)
	assertEqInt(t, 1, len(lines))
	assertNearPoint(t, Point{10, 20, 0}, lines[0].matrix.TransformPoint(line.P1))

	result := drawing.Explode(ExplodeOptions{InheritProperties: true})
	assertEqInt(t, 1, len(result.Entities))
	exploded := result.Entities[0].(*Line)
	assertNearPoint(t, Point{40, 20, 0}, exploded.P2)
	assertEqString(t, "TABLES", exploded.Layer())
}

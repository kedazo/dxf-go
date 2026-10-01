package dxf

import (
	"reflect"
	"testing"
)

func TestReadXDataValues(t *testing.T) {
	line := parseEntity(t, "LINE",
		NewDoubleCodePair(10, 1.0),
		NewStringCodePair(1001, "APP_ONE"),
		NewStringCodePair(1000, "text"),
		NewStringCodePair(1003, "WALLS"),
		NewStringCodePair(1004, "DEAD01"),
		NewStringCodePair(1005, "1F"),
		NewDoubleCodePair(1010, 1.0), NewDoubleCodePair(1020, 2.0), NewDoubleCodePair(1030, 3.0),
		NewDoubleCodePair(1013, 0.0), NewDoubleCodePair(1023, 1.0), NewDoubleCodePair(1033, 0.0),
		NewDoubleCodePair(1040, 1.5),
		NewDoubleCodePair(1041, 2.5),
		NewDoubleCodePair(1042, 3.5),
		NewShortCodePair(1070, 7),
		NewIntCodePair(1071, 70000),
		NewStringCodePair(1002, "{"),
		NewStringCodePair(1000, "in list"),
		NewStringCodePair(1002, "{"),
		NewShortCodePair(1070, 1),
		NewStringCodePair(1002, "}"),
		NewStringCodePair(1002, "}"),
		NewStringCodePair(1001, "APP_TWO"),
		NewStringCodePair(1002, "{"), // never closed
		NewStringCodePair(1000, "last"),
	).(*Line)
	// the entity's own data is unaffected
	assertEqFloat64(t, 1.0, line.P1.X)

	expected := XData{
		{Name: "APP_ONE", Items: []XDataItem{
			{Code: 1000, String: "text"},
			{Code: 1003, String: "WALLS"},
			{Code: 1004, String: "DEAD01"},
			{Code: 1005, String: "1F"},
			{Code: 1010, Point: Point{1, 2, 3}},
			{Code: 1013, Point: Point{0, 1, 0}},
			{Code: 1040, Real: 1.5},
			{Code: 1041, Real: 2.5},
			{Code: 1042, Real: 3.5},
			{Code: 1070, Int: 7},
			{Code: 1071, Int: 70000},
			{Code: 1002, Items: []XDataItem{
				{Code: 1000, String: "in list"},
				{Code: 1002, Items: []XDataItem{{Code: 1070, Int: 1}}},
			}},
		}},
		{Name: "APP_TWO", Items: []XDataItem{
			{Code: 1002, Items: []XDataItem{{Code: 1000, String: "last"}}},
		}},
	}
	assert(t, reflect.DeepEqual(expected, line.XData()), "unexpected xdata")
	assertEqString(t, "APP_TWO", line.XData().Application("app_two").Name)
	assert(t, line.XData().Application("NONE") == nil, "expected no application")

	// copies don't share the extended data
	clone := CloneEntity(line).(*Line)
	clone.XData()[0].Items[11].Items[0].String = "changed"
	assertEqString(t, "in list", line.XData()[0].Items[11].Items[0].String)

	// entities without extended data have none
	assert(t, parseEntity(t, "LINE", NewDoubleCodePair(10, 1.0)).XData() == nil, "expected no xdata")
}

func TestReadXDataOfOtherItems(t *testing.T) {
	drawing := parse(t, join(
		"  0", "SECTION", "  2", "TABLES",
		"  0", "TABLE", "  2", "STYLE",
		"  0", "STYLE", "  5", "11", "  2", "NARROW", "  3", "ARIALN.TTF",
		"1001", "ACAD", "1000", "Arial Narrow", "1071", "33554466",
		"  0", "STYLE", "  5", "12", "  2", "PLAIN", "  3", "txt",
		"  0", "ENDTAB",
		"  0", "ENDSEC",
		"  0", "SECTION", "  2", "BLOCKS",
		"  0", "BLOCK", "  2", "B", " 70", "0", " 10", "0.0", " 20", "0.0", " 30", "0.0",
		"1001", "BLOCK_APP", "1000", "on the block",
		"  0", "ENDBLK",
		"  0", "ENDSEC",
		"  0", "SECTION", "  2", "OBJECTS",
		"  0", "MLEADERSTYLE", "  5", "5C", "  3", "Standard",
		"1001", "ACAD_MLEADERVER", "1070", "2",
		"  0", "ENDSEC",
		"  0", "EOF",
	))

	narrow := drawing.Styles[0]
	family, bold, italic, ok := narrow.TrueTypeFont()
	assertEqBool(t, true, ok)
	assertEqString(t, "Arial Narrow", family)
	assertEqBool(t, true, bold)
	assertEqBool(t, false, italic)
	// FontFlags still reads the same 1071
	assertEqInt(t, 33554466, narrow.FontFlags)
	_, _, _, ok = drawing.Styles[1].TrueTypeFont()
	assertEqBool(t, false, ok)

	assertEqString(t, "on the block", drawing.Blocks[0].XData.Application("BLOCK_APP").Items[0].String)
	assertEqInt(t, 2, drawing.MLeaderStyles[0].XData.Application("ACAD_MLEADERVER").Items[0].Int)
}

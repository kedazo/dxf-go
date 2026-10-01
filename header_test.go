package dxf

import (
	"fmt"
	"testing"
)

func TestReadNonDefaultHeaderVersion(t *testing.T) {
	header := parseHeader(t,
		NewStringCodePair(9, "$UNSUPPORTED_HEADER_VARIABLE"),
		NewStringCodePair(1, "UNSUPPORTED_VALUE"),
		NewStringCodePair(9, "$ACADVER"),
		NewStringCodePair(1, "AC1014"),
		NewStringCodePair(9, "$ACADMAINTVER"),
		NewShortCodePair(70, 6),
		NewStringCodePair(9, "$UNSUPPORTED_HEADER_VARIABLE"),
		NewStringCodePair(1, "UNSUPPORTED_VALUE"),
	)
	assertEqInt(t, int(R14), int(header.Version))
	assertEqInt(t, 6, int(header.MaintenanceVersion))
}

func TestMalformedHeaderVariablesAreReportedAsWarnings(t *testing.T) {
	drawing := parseFromCodePairs(t,
		NewStringCodePair(0, "SECTION"),
		NewStringCodePair(2, "HEADER"),
		NewStringCodePair(9, "$ACADMAINTVER"),
		NewStringCodePair(1, "GARBAGE"),
		NewStringCodePair(9, "$INSBASE"),
		NewDoubleCodePair(10, 1.0),
		NewDoubleCodePair(40, 2.0),
		NewStringCodePair(9, "$ACADVER"),
		NewStringCodePair(1, "AC1015"),
		NewStringCodePair(9, "$UNKNOWNVARIABLE"),
		NewStringCodePair(1, "fine"),
		NewStringCodePair(0, "ENDSEC"),
		NewStringCodePair(0, "EOF"),
	)
	assertEqInt(t, int(R2000), int(drawing.Header.Version))
	assertEqFloat64(t, 1.0, drawing.Header.InsertionBase.X)
	assertEqInt(t, 2, len(drawing.Warnings))
	assertEqString(t, "header variable $ACADMAINTVER: skipped a value with unexpected group code 1", drawing.Warnings[0])
	assertEqString(t, "header variable $INSBASE: skipped a value with unexpected group code 40", drawing.Warnings[1])
}

func TestLineWeightValues(t *testing.T) {
	// DXF line weight codes: -1 = BYLAYER, -2 = BYBLOCK, -3 = DEFAULT
	assertEqInt(t, -1, int(NewLineWeightByLayer()))
	assertEqInt(t, -2, int(NewLineWeightByBlock()))
	assertEqInt(t, -3, int(NewLineWeightStandard()))

	// AutoCAD's defaults
	header := *NewHeader()
	assertEqInt(t, -1, int(header.NewObjectLineWeight))
	assertEqInt(t, -2, int(header.DimensionLineWeight))
	assertEqInt(t, -2, int(header.DimensionExtensionLineWeight))

	line := parseEntity(t, "LINE", NewShortCodePair(370, -2)).(*Line)
	lineWeight := line.LineWeight()
	assertEqBool(t, true, lineWeight.ByBlock())
}

func TestTolerateMalformedHeaderVariable(t *testing.T) {
	// $ACADMAINTVER expects a code-70 (short) value, but here it is followed by
	// a code-1 (string) value — the kind of degraded HEADER that LibreDWG emits
	// for some AutoCAD 2018/AC1032 DWGs ("Template section not found"). Strict
	// parsing used to abort with "expected code 70"; tolerant parsing must skip
	// the bad variable and still read the well-formed $ACADVER that follows.
	header := parseHeader(t,
		NewStringCodePair(9, "$ACADMAINTVER"),
		NewStringCodePair(1, "GARBAGE"),
		NewStringCodePair(9, "$ACADVER"),
		NewStringCodePair(1, "AC1014"),
	)
	assertEqInt(t, int(R14), int(header.Version))
}

func TestReadMaintenanceVersionWithCode90(t *testing.T) {
	header := parseHeader(t,
		NewStringCodePair(9, "$ACADMAINTVER"),
		NewIntCodePair(90, 105),
	)
	assertEqInt(t, 105, int(header.MaintenanceVersion))
}

func TestReadOutOfRangeDates(t *testing.T) {
	drawing := parse(t, join(
		"  0", "SECTION",
		"  2", "HEADER",
		"  9", "$TDCREATE",
		" 40", "0.0",
		"  9", "$TDUPDATE",
		" 40", "1.0e300",
		"  9", "$TDINDWG",
		" 40", "1.0e300",
		"  9", "$TDUCREATE",
		" 40", "2634300.5",
		"  0", "ENDSEC",
		"  0", "EOF",
	))
	// unset or bogus dates are the zero time, and are written back as 0
	assert(t, drawing.Header.CreationDate.IsZero(), fmt.Sprintf("expected the zero time, got %v", drawing.Header.CreationDate))
	assert(t, drawing.Header.UpdateDate.IsZero(), fmt.Sprintf("expected the zero time, got %v", drawing.Header.UpdateDate))
	assertEqInt(t, 0, int(drawing.Header.TimeInDrawing))
	assertContainsCodePairs(t, []CodePair{
		NewStringCodePair(9, "$TDCREATE"),
		NewDoubleCodePair(40, 0.0),
	}, fileCodePairsFromHeader(t, drawing.Header))
	// more than 292 years after 1899 no longer overflows a time.Duration
	assertEqInt(t, 2500, drawing.Header.CreationDateUniversal.Year())
}

func TestWriteVersionSpecificVariables(t *testing.T) {
	header := *NewHeader()

	// value is present >= R14
	header.Version = R14
	assertContainsCodePairs(t, []CodePair{
		NewStringCodePair(9, "$ACADMAINTVER"),
	}, fileCodePairsFromHeader(t, header))

	// value is missing < R14
	header.Version = R13
	assertNotContainsCodePairs(t, []CodePair{
		NewStringCodePair(9, "$ACADMAINTVER"),
	}, fileCodePairsFromHeader(t, header))
}

func TestReadHeaderFlag(t *testing.T) {
	header := parseHeader(t,
		NewStringCodePair(9, "$OSMODE"),
		NewShortCodePair(70, 1),
	)
	assert(t, header.EndPointSnap(), "expected OSMODE.EndPointSnap")
	assert(t, !header.MidPointSnap(), "expected !OSMODE.MidPointSnap")
	assert(t, !header.CenterSnap(), "expected !OSMODE.CenterSnap")
	assert(t, !header.NodeSnap(), "expected !OSMODE.NodeSnap")
	assert(t, !header.QuadrantSnap(), "expected !OSMODE.QuadrantSnap")
	assert(t, !header.IntersectionSnap(), "expected !OSMODE.IntersectionSnap")
	assert(t, !header.InsertionSnap(), "expected !OSMODE.InsertionSnap")
	assert(t, !header.PerpendicularSnap(), "expected !OSMODE.PerpendicularSnap")
	assert(t, !header.TangentSnap(), "expected !OSMODE.TangentSnap")
	assert(t, !header.NearestSnap(), "expected !OSMODE.NearestSnap")
	assert(t, !header.ApparentIntersectionSnap(), "expected !OSMODE.ApparentIntersectionSnap")
	assert(t, !header.ExtensionSnap(), "expected !OSMODE.ExtensionSnap")
	assert(t, !header.ParallelSnap(), "expected !OSMODE.ParallelSnap")
}

func TestWriteHeaderFlag(t *testing.T) {
	header := *NewHeader()
	header.SetEndPointSnap(true)
	header.SetMidPointSnap(false)
	header.SetCenterSnap(false)
	header.SetNodeSnap(false)
	header.SetQuadrantSnap(false)
	header.SetIntersectionSnap(false)
	header.SetInsertionSnap(false)
	header.SetPerpendicularSnap(false)
	header.SetTangentSnap(false)
	header.SetNearestSnap(false)
	header.SetApparentIntersectionSnap(false)
	header.SetExtensionSnap(false)
	header.SetParallelSnap(false)
	assertContainsCodePairs(t, []CodePair{
		NewStringCodePair(9, "$OSMODE"),
		NewShortCodePair(70, 1),
	}, fileCodePairsFromHeader(t, header))
}

func TestReadPoint(t *testing.T) {
	header := parseHeader(t,
		NewStringCodePair(9, "$INSBASE"),
		NewDoubleCodePair(10, 1.0),
		NewDoubleCodePair(20, 2.0),
		NewDoubleCodePair(30, 3.0),
	)
	expected := Point{1.0, 2.0, 3.0}
	assert(t, header.InsertionBase == expected, fmt.Sprintf("expected %s, got %s", expected.String(), header.InsertionBase.String()))
}

func TestWritePoint(t *testing.T) {
	header := *NewHeader()
	header.InsertionBase = Point{1.0, 2.0, 3.0}
	assertContainsCodePairs(t, []CodePair{
		NewStringCodePair(9, "$INSBASE"),
		NewDoubleCodePair(10, 1.0),
		NewDoubleCodePair(20, 2.0),
		NewDoubleCodePair(30, 3.0),
	}, fileCodePairsFromHeader(t, header))
}

func TestReadEnumValue(t *testing.T) {
	header := parseHeader(t,
		NewStringCodePair(9, "$DRAGMODE"),
		NewShortCodePair(70, 2),
	)
	assertEqShort(t, int16(2), int16(header.DragMode))
}

func TestWriteEnumValue(t *testing.T) {
	header := *NewHeader()
	header.DragMode = DragModeAuto
	assertContainsCodePairs(t, []CodePair{
		NewStringCodePair(9, "$DRAGMODE"),
		NewShortCodePair(70, 2),
	}, fileCodePairsFromHeader(t, header))
}

func TestReadHandleValue(t *testing.T) {
	header := parseHeader(t,
		NewStringCodePair(9, "$DRAGVS"),
		NewStringCodePair(349, "FF"),
	)
	assertEqUInt64(t, uint64(255), uint64(header.SolidVisualStylePointer))
}

func TestWriteHandleValue(t *testing.T) {
	header := *NewHeader()
	header.Version = R2007 // min version R2007
	header.SolidVisualStylePointer = Handle(255)
	assertContainsCodePairs(t, []CodePair{
		NewStringCodePair(9, "$DRAGVS"),
		NewStringCodePair(349, "FF"),
	}, fileCodePairsFromHeader(t, header))
}

func parseHeader(t *testing.T, codePairs ...CodePair) Header {
	allPairs := []CodePair{
		NewStringCodePair(0, "SECTION"),
		NewStringCodePair(2, "HEADER"),
	}
	allPairs = append(allPairs, codePairs...)
	allPairs = append(allPairs,
		NewStringCodePair(0, "ENDSEC"),
		NewStringCodePair(0, "EOF"),
	)
	drawing := parseFromCodePairs(t, allPairs...)
	return drawing.Header
}

func fileCodePairsFromHeader(t *testing.T, h Header) []CodePair {
	drawing := *NewDrawing()
	drawing.Header = h
	codePairs, err := drawing.CodePairs()
	if err != nil {
		t.Error(err)
	}

	return codePairs
}

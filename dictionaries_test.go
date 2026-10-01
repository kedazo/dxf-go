package dxf

import (
	"reflect"
	"testing"
)

func dictionaryDrawing(t *testing.T) Drawing {
	return parse(t, join(
		"  0", "SECTION", "  2", "OBJECTS",
		// the named object dictionary
		"  0", "DICTIONARY", "  5", "C", "330", "0", "100", "AcDbDictionary", "281", "1",
		"  3", "ACAD_MLEADERSTYLE", "350", "5B",
		"  3", "ACAD_PLOTSTYLENAME", "350", "E",
		"  3", "ACAD_SCALELIST", "350", "70",
		"  3", "AcDbVariableDictionary", "350", "80",
		"  3", "APP_DATA", "360", "90",
		"  0", "DICTIONARY", "  5", "5B",
		"102", "{ACAD_REACTORS", "330", "C", "102", "}",
		"330", "C", "100", "AcDbDictionary", "281", "1",
		"  3", "Standard", "350", "5C",
		"  0", "MLEADERSTYLE", "  5", "5C", "330", "5B", "100", "AcDbMLeaderStyle", " 45", "0.18",
		"  0", "ACDBDICTIONARYWDFLT", "  5", "E", "330", "C", "100", "AcDbDictionary", "281", "1",
		"  3", "Normal", "350", "F",
		"100", "AcDbDictionaryWithDefault", "340", "F",
		"  0", "DICTIONARY", "  5", "70", "330", "C", "100", "AcDbDictionary", "281", "1",
		"  3", "A0", "350", "71",
		"  0", "SCALE", "  5", "71", "330", "70", "100", "AcDbScale", " 70", "0",
		"300", "1:100", "140", "1.0", "141", "100.0", "290", "0",
		"  0", "DICTIONARY", "  5", "80", "330", "C", "100", "AcDbDictionary", "281", "1",
		"  3", "XCLIPFRAME", "350", "81",
		"  0", "DICTIONARYVAR", "  5", "81", "330", "80", "100", "DictionaryVariables", "280", "0", "  1", "2",
		"  0", "DICTIONARY", "  5", "90", "330", "C", "100", "AcDbDictionary", "280", "1", "281", "1",
		"  3", "SETTINGS", "360", "91",
		"  0", "XRECORD", "  5", "91", "330", "90", "100", "AcDbXrecord", "280", "1",
		"  1", "text", " 40", "2.5", " 70", "3",
		"1001", "APP", "1000", "extended",
		"  0", "ENDSEC",
		"  0", "EOF",
	))
}

func TestReadDictionaries(t *testing.T) {
	drawing := dictionaryDrawing(t)
	assertEqInt(t, 6, len(drawing.Dictionaries))

	root := drawing.NamedObjectDictionary()
	assert(t, root != nil, "expected the named object dictionary")
	assertEqInt(t, 0xC, int(root.Handle))
	assertEqInt(t, 5, len(root.Entries))
	assertEqInt(t, 0x5B, int(root.Lookup("acad_mleaderstyle")))
	assertEqBool(t, true, root.Entries[4].IsHardOwner)
	assertEqInt(t, 0, int(root.Lookup("missing")))

	// the owner after a reactor group is the dictionary's own
	assertEqInt(t, 0xC, int(drawing.dictionaryByHandle(0x5B).OwnerHandle))

	// names lead to objects, and objects to their names
	handle := drawing.NamedObject("ACAD_MLEADERSTYLE", "Standard")
	assertEqInt(t, 0x5C, int(handle))
	style, ok := drawing.ItemByHandle(handle).(*MLeaderStyle)
	assert(t, ok && style.TextHeight == 0.18, "expected the multileader style")
	assertEqString(t, "Standard", drawing.ObjectName(style.Handle))
	assertEqInt(t, 0, int(drawing.NamedObject("ACAD_MLEADERSTYLE", "missing")))
	assertEqInt(t, 0, int(drawing.NamedObject("missing", "Standard")))

	plotStyles := drawing.ItemByHandle(0xE).(*Dictionary)
	assertEqInt(t, 0xF, int(plotStyles.DefaultHandle))
	assertEqInt(t, 0xF, int(plotStyles.Lookup("Normal")))
}

func TestReadDictionaryObjects(t *testing.T) {
	drawing := dictionaryDrawing(t)

	assertEqInt(t, 1, len(drawing.Scales))
	scale := drawing.ItemByHandle(drawing.NamedObject("ACAD_SCALELIST", "A0")).(*Scale)
	assertEqString(t, "1:100", scale.Name)
	assertEqFloat64(t, 1.0, scale.PaperUnits)
	assertEqFloat64(t, 100.0, scale.DrawingUnits)
	assertEqBool(t, false, scale.IsUnitScale)

	value, ok := drawing.DictionaryVariable("XCLIPFRAME")
	assertEqBool(t, true, ok)
	assertEqString(t, "2", value)
	_, ok = drawing.DictionaryVariable("MISSING")
	assertEqBool(t, false, ok)

	record := drawing.ItemByHandle(drawing.NamedObject("APP_DATA", "SETTINGS")).(*XRecord)
	assertEqInt(t, 0x90, int(record.OwnerHandle))
	assertEqInt(t, 1, int(record.DuplicateRecordCloning))
	// the data stops where the extended data starts
	assert(t, reflect.DeepEqual([]CodePair{
		NewStringCodePair(1, "text"), NewDoubleCodePair(40, 2.5), NewShortCodePair(70, 3),
	}, record.Data), "unexpected xrecord data")
	assertEqString(t, "extended", record.XData.Application("APP").Items[0].String)
}

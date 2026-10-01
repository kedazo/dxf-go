package dxf

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"golang.org/x/text/encoding/charmap"
)

// codePageDrawing is a drawing whose header names a code page and whose TEXT value is raw (undecoded) bytes.
func codePageDrawing(version, codePage, rawText string) string {
	return join(
		"  0", "SECTION",
		"  2", "HEADER",
		"  9", "$ACADVER",
		"  1", version,
		"  9", "$DWGCODEPAGE",
		"  3", codePage,
		"  9", "$LASTSAVEDBY",
		"  1", rawText,
		"  0", "ENDSEC",
		"  0", "SECTION",
		"  2", "ENTITIES",
		"  0", "TEXT",
		"  1", rawText,
		"  0", "ENDSEC",
		"  0", "EOF",
	)
}

func readCodePageDrawing(t *testing.T, version, codePage, rawText string) Drawing {
	drawing, err := ReadFromReader(strings.NewReader(codePageDrawing(version, codePage, rawText)))
	if err != nil {
		t.Fatal(err)
	}
	return drawing
}

func TestReadPre2007TextInDrawingCodePage(t *testing.T) {
	// "Előtér" in Windows-1250
	drawing := readCodePageDrawing(t, "AC1015", "ANSI_1250", "El\xF5t\xE9r")
	assertEqString(t, "Előtér", drawing.Entities[0].(*Text).Value)
	assertEqString(t, "Előtér", drawing.Header.LastSavedBy)

	// Cyrillic in Windows-1251, code page name in lower case
	drawing = readCodePageDrawing(t, "AC1018", "ansi_1251", "\xCF\xF0\xE8\xE2\xE5\xF2")
	assertEqString(t, "Привет", drawing.Entities[0].(*Text).Value)
}

func TestReadPre2007EscapesInDrawingCodePage(t *testing.T) {
	drawing := readCodePageDrawing(t, "AC1015", "ANSI_1250", "\\U+0150r \xE9s")
	assertEqString(t, "Őr és", drawing.Entities[0].(*Text).Value)
}

func TestReadPre2007Utf8TextWithCodePage(t *testing.T) {
	// some writers put UTF-8 into pre-2007 files; valid UTF-8 is kept as it is
	drawing := readCodePageDrawing(t, "AC1015", "ANSI_1252", "Előtér")
	assertEqString(t, "Előtér", drawing.Entities[0].(*Text).Value)
}

func TestReadR2007TextIgnoresCodePage(t *testing.T) {
	// ArchiCAD writes the Windows code page even into UTF-8 files
	drawing := readCodePageDrawing(t, "AC1021", "ANSI_1250", "Előtér")
	assertEqString(t, "Előtér", drawing.Entities[0].(*Text).Value)
}

func TestReadExplicitEncodingWinsOverCodePage(t *testing.T) {
	content := codePageDrawing("AC1015", "ANSI_1250", "El\xF5t\xE9r")
	drawing, err := ReadFromReaderWithEncoding(strings.NewReader(content), charmap.Windows1252)
	if err != nil {
		t.Fatal(err)
	}
	assertEqString(t, "Elõtér", drawing.Entities[0].(*Text).Value)
}

func TestReadPre2007TextWithUnknownCodePage(t *testing.T) {
	drawing := readCodePageDrawing(t, "AC1015", "ANSI_9999", "El\xF5t\xE9r")
	assertEqString(t, "El�t�r", drawing.Entities[0].(*Text).Value)
}

// binaryStringDrawing builds a post-R13 binary DXF from string-valued code pairs, keeping the value bytes as they are.
func binaryStringDrawing(pairs ...string) []byte {
	data := []byte("AutoCAD Binary DXF\r\n\x1A\x00")
	for i := 0; i < len(pairs); i += 2 {
		var code int
		fmt.Sscan(pairs[i], &code)
		data = append(data, byte(code), byte(code>>8))
		data = append(data, pairs[i+1]...)
		data = append(data, 0x00)
	}
	return data
}

func binaryCodePageDrawing(version, codePage, rawText string) []byte {
	return binaryStringDrawing(
		"0", "SECTION",
		"2", "HEADER",
		"9", "$ACADVER",
		"1", version,
		"9", "$DWGCODEPAGE",
		"3", codePage,
		"0", "ENDSEC",
		"0", "SECTION",
		"2", "ENTITIES",
		"0", "TEXT",
		"1", rawText,
		"0", "ENDSEC",
		"0", "EOF",
	)
}

func TestReadBinaryPre2007TextInDrawingCodePage(t *testing.T) {
	drawing, err := ReadFromReader(bytes.NewReader(binaryCodePageDrawing("AC1015", "ANSI_1250", "El\xF5t\xE9r \\U+0150r")))
	if err != nil {
		t.Fatal(err)
	}
	assertEqString(t, "Előtér Őr", drawing.Entities[0].(*Text).Value)
}

func TestReadBinaryR2007TextAsUtf8(t *testing.T) {
	drawing, err := ReadFromReader(bytes.NewReader(binaryCodePageDrawing("AC1021", "ANSI_1250", "Előtér")))
	if err != nil {
		t.Fatal(err)
	}
	assertEqString(t, "Előtér", drawing.Entities[0].(*Text).Value)
}

func TestReadBinaryWithExplicitEncoding(t *testing.T) {
	data := binaryCodePageDrawing("AC1015", "ANSI_1250", "El\xF5t\xE9r")
	drawing, err := ReadFromReaderWithEncoding(bytes.NewReader(data), charmap.Windows1252)
	if err != nil {
		t.Fatal(err)
	}
	assertEqString(t, "Elõtér", drawing.Entities[0].(*Text).Value)
}

func TestEncodingFromCodePage(t *testing.T) {
	assert(t, encodingFromCodePage("ANSI_1250") == charmap.Windows1250, "expected Windows-1250")
	assert(t, encodingFromCodePage(" ansi_1252 ") == charmap.Windows1252, "expected Windows-1252")
	assert(t, encodingFromCodePage("DOS852") == charmap.CodePage852, "expected code page 852")
	assert(t, encodingFromCodePage("unknown") == nil, "expected nil for an unknown code page")
}

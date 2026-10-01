package dxf

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/unicode"
)

func TestReadBoolAsText(t *testing.T) {
	assertReadBoolText(t, true, "1")
	assertReadBoolText(t, true, "   1   ")
	assertReadBoolText(t, true, "3")
	assertReadBoolText(t, false, "0")
	assertReadBoolText(t, false, "   0   ")
}

func TestReadShortAsText(t *testing.T) {
	assertReadShortText(t, 2, "2")
	assertReadShortText(t, -2, "-2")
	assertReadShortText(t, 2, "        2        ")
}

func TestReadIntAsText(t *testing.T) {
	assertReadIntText(t, 2, "2")
	assertReadIntText(t, -2, "-2")
	assertReadIntText(t, 2, "          2          ")
}

func TestReadIntegersWrittenAsDecimals(t *testing.T) {
	assertReadShortText(t, 1, "1.0")
	assertReadShortText(t, -2, "  -2.7  ")
	assertReadIntText(t, 3, "3.0")
	assertReadLongText(t, 4, "4e0")
	assertReadBoolText(t, true, "1.0")
	if _, err := readShortText("abc"); err == nil {
		t.Error("expected an error for a non-number")
	}
	if _, err := readLongText("1e30"); err == nil {
		t.Error("expected an error for a number out of range")
	}
}

func TestReadLongAsText(t *testing.T) {
	assertReadLongText(t, 2, "2")
	assertReadLongText(t, -2, "-2")
	assertReadLongText(t, 2, "          2          ")
}

func TestReadDoubleAsText(t *testing.T) {
	assertReadDoubleText(t, 11.0, "1.100000E+001")
	assertReadDoubleText(t, 55.0, "5.5e1")
	assertReadDoubleText(t, 2.0, "2")
}

func TestReadStringAsText(t *testing.T) {
	assertReadStringText(t, "Repère pièce", "Repère pièce", true)
	assertReadStringText(t, "Repère pièce", "Rep\\U+00E8re pi\\U+00E8ce", false)
}

func TestReadEscapedUnicodeNextToOtherEscapes(t *testing.T) {
	// MTEXT paragraph starting with a non-ASCII character
	assertReadStringText(t, "Előtér\\PŐr", "El\\U+0151t\\U+00E9r\\P\\U+0150r", false)
	assertReadStringText(t, "{\\fArial|b0;Á}", "{\\fArial|b0;\\U+00C1}", false)
	// escape sequences after multi-byte characters
	assertReadStringText(t, "éé", "é\\U+00E9", false)
	// incomplete or invalid escapes are kept as they are
	assertReadStringText(t, "a\\U+00", "a\\U+00", false)
	assertReadStringText(t, "\\U+00G1", "\\U+00G1", false)
	assertReadStringText(t, "\\P", "\\P", false)
	// invalid UTF-8 bytes become the replacement character
	assertReadStringText(t, "a�b", "a\xF5b", false)
}

func TestReadCodePairAsText(t *testing.T) {
	// \r\n
	assertReadCodePairText(t, NewBoolCodePair(290, true), "290\r\n1")
	assertReadCodePairText(t, NewShortCodePair(70, 1), "70\r\n1")
	assertReadCodePairText(t, NewIntCodePair(90, 1), "90\r\n1")
	assertReadCodePairText(t, NewLongCodePair(160, 1), "160\r\n1")
	assertReadCodePairText(t, NewDoubleCodePair(10, 1.0), "10\r\n1")
	assertReadCodePairText(t, NewStringCodePair(1, "a"), "1\r\na")

	// \n
	assertReadCodePairText(t, NewIntCodePair(90, 1), "90\n1")
}

func TestReadCodePairsText(t *testing.T) {
	actual := readCodePairsText(t, "1\r\na\r\n90\r\n4\r\n10\r\n1")
	expected := []CodePair{
		NewStringCodePair(1, "a"),
		NewIntCodePair(90, 4),
		NewDoubleCodePair(10, 1.0),
	}

	assertEqCodePairs(t, expected, actual)
}

func TestAutoDetectCodePairsFromText(t *testing.T) {
	actual := readCodePairsFromReader(t, strings.NewReader("1\r\na\r\n90\r\n4\r\n10\r\n1\r\n"))
	expected := []CodePair{
		NewStringCodePair(1, "a"),
		NewIntCodePair(90, 4),
		NewDoubleCodePair(10, 1.0),
	}

	assertEqCodePairs(t, expected, actual)
}

func TestReadBoolAsBinary(t *testing.T) {
	assertReadBoolBinary(t, true, []byte{0x01, 0x00}, false)
	assertReadBoolBinary(t, false, []byte{0x00, 0x00}, false)

	assertReadBoolBinary(t, true, []byte{0x01}, true)
	assertReadBoolBinary(t, false, []byte{0x00}, true)
}

func TestReadShortAsBinary(t *testing.T) {
	assertReadShortBinary(t, 1, []byte{0x01, 0x00})
}

func TestReadIntAsBinary(t *testing.T) {
	assertReadIntBinary(t, 1, []byte{0x01, 0x00, 0x00, 0x00})
}

func TestReadLongAsBinary(t *testing.T) {
	assertReadLongBinary(t, 1, []byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
}

func TestReadDoubleAsBinary(t *testing.T) {
	assertReadDoubleBinary(t, 1.0, []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xF0, 0x3F})
}

func TestReadStringAsBinary(t *testing.T) {
	assertReadStringBinary(t, "a", []byte{0x61, 0x00})
}

func TestReadCodePairAsBinary(t *testing.T) {
	// post R13
	assertReadCodePairBinary(t, NewBoolCodePair(290, true), []byte{0x22, 0x01, 0x01}, true)
	assertReadCodePairBinary(t, NewShortCodePair(70, 1), []byte{0x46, 0x00, 0x01, 0x00}, true)
	assertReadCodePairBinary(t, NewIntCodePair(90, 1), []byte{0x5A, 0x00, 0x01, 0x00, 0x00, 0x00}, true)
	assertReadCodePairBinary(t, NewLongCodePair(160, 1), []byte{0xA0, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, true)
	assertReadCodePairBinary(t, NewDoubleCodePair(10, 1.0), []byte{0x0A, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xF0, 0x3F}, true)
	assertReadCodePairBinary(t, NewStringCodePair(1, "a"), []byte{0x01, 0x00, 0x61, 0x00}, true)

	// pre R13
	assertReadCodePairBinary(t, NewBoolCodePair(290, true), []byte{0xFF, 0x22, 0x01, 0x01, 0x00}, false)
}

func TestReadCodePairsBinaryPreR13(t *testing.T) {
	data := []byte{
		0x01, 0x61, 0x00, // 1/a
		0x5A, 0x04, 0x00, 0x00, 0x00, // 90/4
		0x0A, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xF0, 0x3F, // 10/1.0
	}

	actual := readCodePairsBinary(t, data, false)
	expected := []CodePair{
		NewStringCodePair(1, "a"),
		NewIntCodePair(90, 4),
		NewDoubleCodePair(10, 1.0),
	}

	assertEqCodePairs(t, expected, actual)
}

func TestReadCodePairsBinaryPostR13(t *testing.T) {
	data := []byte{
		0x01, 0x00, 0x61, 0x00, // 1/a
		0x5A, 0x00, 0x04, 0x00, 0x00, 0x00, // 90/4
		0x0A, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xF0, 0x3F, // 10/1.0
	}

	actual := readCodePairsBinary(t, data, true)
	expected := []CodePair{
		NewStringCodePair(1, "a"),
		NewIntCodePair(90, 4),
		NewDoubleCodePair(10, 1.0),
	}

	assertEqCodePairs(t, expected, actual)
}

func TestAutoDetectCodePairsFromBinaryPreR13(t *testing.T) {
	data := make([]byte, 0)
	// binary header
	data = append(data, []byte("AutoCAD Binary DXF\r\n")...)
	data = append(data, 0x1A, 0x00)
	// code pair data
	data = append(data,
		0x00, 0x61, 0x00, // 0/a
		0x5A, 0x04, 0x00, 0x00, 0x00, // 90/4
		0x0A, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xF0, 0x3F, // 10/1.0
	)

	buf := bytes.NewReader(data)
	actual := readCodePairsFromReader(t, buf)
	expected := []CodePair{
		NewStringCodePair(0, "a"),
		NewIntCodePair(90, 4),
		NewDoubleCodePair(10, 1.0),
	}

	assertEqCodePairs(t, expected, actual)
}

func TestAutoDetectCodePairsFromBinaryPostR13(t *testing.T) {
	data := make([]byte, 0)
	// binary header
	data = append(data, []byte("AutoCAD Binary DXF\r\n")...)
	data = append(data, 0x1A, 0x00)
	// code pair data
	data = append(data,
		0x00, 0x00, 0x61, 0x00, // 0/a
		0x5A, 0x00, 0x04, 0x00, 0x00, 0x00, // 90/4
		0x0A, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xF0, 0x3F, // 10/1.0
	)

	buf := bytes.NewReader(data)
	actual := readCodePairsFromReader(t, buf)
	expected := []CodePair{
		NewStringCodePair(0, "a"),
		NewIntCodePair(90, 4),
		NewDoubleCodePair(10, 1.0),
	}

	assertEqCodePairs(t, expected, actual)
}

func TestReadBinaryChunksAsBinary(t *testing.T) {
	data := []byte{
		0x36, 0x01, 0x03, 0xDE, 0xAD, 0x01, // 310/DEAD01
		0x36, 0x01, 0x00, // 310/(empty)
		0xEC, 0x03, 0x02, 0x00, 0x0A, // 1004/000A
		0x01, 0x00, 0x61, 0x00, // 1/a
	}

	actual := readCodePairsBinary(t, data, true)
	expected := []CodePair{
		NewStringCodePair(310, "DEAD01"),
		NewStringCodePair(310, ""),
		NewStringCodePair(1004, "000A"),
		NewStringCodePair(1, "a"),
	}

	assertEqCodePairs(t, expected, actual)
}

func TestReadUnknownCodeAsBinaryReturnsError(t *testing.T) {
	reader := binaryCodePairReader{
		reader:          bufio.NewReader(bytes.NewReader([]byte{0x96, 0x00, 0x61, 0x00})), // 150/?
		hasReturnedPair: true,
		isPostR13:       true,
	}
	_, err := reader.readCodePair()
	if err == nil {
		t.Error("expected an error for an unknown group code")
	}
}

func TestReadCaretEscapes(t *testing.T) {
	assertEqString(t, "a\nb\tc", unescapeCarets("a^Jb^Ic"))
	assertEqString(t, "x^2", unescapeCarets("x^ 2"))
	assertEqString(t, "\x00\x1F", unescapeCarets("^@^_"))
	// a caret before anything else, or at the end, is text
	assertEqString(t, "x^2^", unescapeCarets("x^2^"))

	actual := readCodePairsBinary(t, []byte{0x01, 0x00, 0x61, 0x5E, 0x4A, 0x62, 0x00}, true) // 1/a^Jb
	assertEqCodePairs(t, []CodePair{NewStringCodePair(1, "a\nb")}, actual)
}

func TestReadUnknownCodeAsText(t *testing.T) {
	actual := readCodePairsText(t, join(
		"150", "abc",
		"1080", "1.5",
		"  1", "a",
	))
	expected := []CodePair{
		NewStringCodePair(150, "abc"),
		NewStringCodePair(1080, "1.5"),
		NewStringCodePair(1, "a"),
	}

	assertEqCodePairs(t, expected, actual)
}

func TestReadEntityWithUnknownCodes(t *testing.T) {
	drawing := parse(t, join(
		"  0", "SECTION",
		"  2", "ENTITIES",
		"  0", "LINE",
		"150", "abc",
		" 10", "1.0",
		"1080", "xyz",
		" 11", "2.0",
		"  0", "ENDSEC",
		"  0", "EOF",
	))
	assertEqInt(t, 1, len(drawing.Entities))
	line := drawing.Entities[0].(*Line)
	assertEqPoint(t, Point{1.0, 0.0, 0.0}, line.P1)
	assertEqPoint(t, Point{2.0, 0.0, 0.0}, line.P2)
}

func TestReadErrorHasTheLine(t *testing.T) {
	_, err := ParseDrawing(join(
		"  0", "SECTION",
		"  2", "ENTITIES",
		"  0", "LINE",
		" 10", "not a number",
		"  0", "ENDSEC",
		"  0", "EOF",
	))
	var readError *ReadError
	if !errors.As(err, &readError) {
		t.Fatalf("expected a ReadError, got %v", err)
	}
	assertEqInt(t, 8, readError.Line)
	assertEqInt(t, -1, int(readError.Offset))
	assert(t, strings.HasPrefix(err.Error(), "line 8: "), "unexpected message "+err.Error())
	var numError *strconv.NumError
	assert(t, errors.As(err, &numError), "expected the parse error to be wrapped")

	// a structural error: the last value read is the stray 1/x on line 2
	_, err = ParseDrawing(join("  1", "x"))
	assert(t, errors.As(err, &readError), "expected a ReadError")
	assertEqInt(t, 2, readError.Line)
}

func TestReadErrorHasTheByteOffset(t *testing.T) {
	data := []byte("AutoCAD Binary DXF\r\n\x1A\x00")
	data = append(data,
		0x00, 0x00, 'S', 'E', 'C', 'T', 'I', 'O', 'N', 0x00, // 0/SECTION at 22
		0x02, 0x00, 'E', 'N', 'T', 'I', 'T', 'I', 'E', 'S', 0x00, // 2/ENTITIES at 32
		0x00, 0x00, 'L', 'I', 'N', 'E', 0x00, // 0/LINE at 43
		0x96, 0x00, 0x01, 0x02, // 150 at 50: a code of unknown type
	)
	_, err := ReadFromReader(bytes.NewReader(data))
	var readError *ReadError
	if !errors.As(err, &readError) {
		t.Fatalf("expected a ReadError, got %v", err)
	}
	assertEqInt(t, 0, readError.Line)
	assertEqInt(t, 50, int(readError.Offset))
	assert(t, strings.HasPrefix(err.Error(), "byte offset 50: "), "unexpected message "+err.Error())
}

func TestReadCarriageReturnLineEndings(t *testing.T) {
	content := strings.Join([]string{
		"  0", "SECTION", "  2", "ENTITIES",
		"  0", "TEXT", "  1", "a^Mb", " 10", "1.5",
		"  0", "ENDSEC", "  0", "EOF",
	}, "\r")
	drawing, err := ReadFromReader(strings.NewReader(content))
	if err != nil {
		t.Fatal(err)
	}
	assertEqInt(t, 1, len(drawing.Entities))
	text := drawing.Entities[0].(*Text)
	assertEqString(t, "a\rb", text.Value)
	assertEqFloat64(t, 1.5, text.Location.X)
}

func TestReadUtf16File(t *testing.T) {
	content := join(
		"  0", "SECTION", "  2", "HEADER",
		"  9", "$ACADVER", "  1", "AC1018",
		"  9", "$DWGCODEPAGE", "  3", "ANSI_1250",
		"  0", "ENDSEC",
		"  0", "SECTION", "  2", "ENTITIES",
		"  0", "TEXT", "  1", "Előtér \\U+00E9",
		"  0", "ENDSEC", "  0", "EOF",
	)
	for _, endianness := range []unicode.Endianness{unicode.LittleEndian, unicode.BigEndian} {
		encoded, err := unicode.UTF16(endianness, unicode.UseBOM).NewEncoder().String(content)
		if err != nil {
			t.Fatal(err)
		}
		// the UTF-16 wins over the code page and over the encoding the caller passes
		drawing, err := ReadFromReaderWithEncoding(strings.NewReader(encoded), charmap.Windows1252)
		if err != nil {
			t.Fatal(err)
		}
		assertEqString(t, "Előtér é", drawing.Entities[0].(*Text).Value)
	}
}

func TestReadEmptyFile(t *testing.T) {
	_ = parse(t, "")
}

func TestReadTextAsAscii(t *testing.T) {
	// if version <= R2004 (AC1018) stream is ASCII

	// unicode values in the middle of the string
	drawing := parse(t, join(
		"  0", "SECTION",
		"  2", "HEADER",
		"  9", "$ACADVER",
		"  1", "AC1018",
		"  9", "$PROJECTNAME",
		"  1", "Rep\\U+00E8re pi\\U+00E8ce",
		"  0", "ENDSEC",
		"  0", "EOF",
	))
	assertEqString(t, "Repère pièce", drawing.Header.ProjectName)

	// unicode values for the entire string
	drawing = parse(t, join(
		"  0", "SECTION",
		"  2", "HEADER",
		"  9", "$ACADVER",
		"  1", "AC1018",
		"  9", "$PROJECTNAME",
		"  1", "\\U+4F60\\U+597D",
		"  0", "ENDSEC",
		"  0", "EOF",
	))
	assertEqString(t, "你好", drawing.Header.ProjectName)
}

func TestReadTextAsUtf8(t *testing.T) {
	// if version >= R2007 (AC1021) stream is UTF-8
	drawing := parse(t, join(
		"  0", "SECTION",
		"  2", "HEADER",
		"  9", "$ACADVER",
		"  1", "AC1021",
		"  9", "$PROJECTNAME",
		"  1", "Repère pièce",
		"  0", "ENDSEC",
		"  0", "EOF",
	))
	assertEqString(t, "Repère pièce", drawing.Header.ProjectName)
}

func TestReadBinaryFile(t *testing.T) {
	// pre R13 binary file
	drawing, err := ReadFile("res/diamond-bin.dxf")
	if err != nil {
		t.Error(err)
	}
	assertEqInt(t, 12, len(drawing.Entities))
	switch line := drawing.Entities[0].(type) {
	case *Line:
		assertEqPoint(t, Point{45.0, 45.0, 0.0}, line.P1)
		assertEqPoint(t, Point{45.0, -45.0, 0.0}, line.P2)
	default:
		t.Error("expected LINE")
	}
}

func TestReadBinaryPostR13(t *testing.T) {
	data := make([]byte, 0)
	// binary header
	data = append(data, []byte("AutoCAD Binary DXF\r\n")...)
	data = append(data, []byte{0x1A, 0x00}...)

	// 0/SECTION
	data = append(data, []byte{0x00, 0x00}...)
	data = append(data, []byte("SECTION")...)
	data = append(data, 0x00)

	// 2/HEADER
	data = append(data, []byte{0x02, 0x00}...)
	data = append(data, []byte("HEADER")...)
	data = append(data, 0x00)

	// 9/$LWDISPLAY
	data = append(data, []byte{0x09, 0x00}...)
	data = append(data, []byte("$LWDISPLAY")...)
	data = append(data, 0x00)

	// 290/true
	data = append(data, []byte{0x22, 0x01}...)
	data = append(data, 0x01)

	// 0/ENDSEC
	data = append(data, []byte{0x00, 0x00}...)
	data = append(data, []byte("ENDSEC")...)
	data = append(data, 0x00)

	// 0/EOF
	data = append(data, []byte{0x00, 0x00}...)
	data = append(data, []byte("EOF")...)
	data = append(data, 0x00)

	reader := bytes.NewReader(data)
	drawing, err := ReadFromReader(reader)
	if err != nil {
		t.Error(err)
	}

	assert(t, drawing.Header.DisplayLinewieghtInModelAndLayoutTab, "expected $LWDISPLAY to be true")
}

func TestReadDrawingWithNonStandardEncoding(t *testing.T) {
	contents := join(
		"  0", "SECTION",
		"  2", "HEADER",
		"  9", "$PROJECTNAME",
		"  1", "\xB2\xBB",
		"  0", "ENDSEC",
		"  0", "EOF",
	)
	data := []byte(contents)
	reader := bytes.NewReader(data)
	drawing, err := ReadFromReaderWithEncoding(reader, simplifiedchinese.GB18030)
	if err != nil {
		t.Error(err)
	}

	assertEqString(t, "不", drawing.Header.ProjectName)
}

func assertReadBoolText(t *testing.T, expected bool, line string) {
	actual, err := readBoolText(line)
	if err != nil {
		t.Error(err)
	}

	if actual != expected {
		t.Errorf("Expected %v but found %v", expected, actual)
	}
}

func assertReadShortText(t *testing.T, expected int16, line string) {
	actual, err := readShortText(line)
	if err != nil {
		t.Error(err)
	}

	if actual != expected {
		t.Errorf("Expected %d but found %d", expected, actual)
	}
}

func assertReadIntText(t *testing.T, expected int, line string) {
	actual, err := readIntText(line)
	if err != nil {
		t.Error(err)
	}

	if actual != expected {
		t.Errorf("Expected %d but found %d", expected, actual)
	}
}

func assertReadLongText(t *testing.T, expected int64, line string) {
	actual, err := readLongText(line)
	if err != nil {
		t.Error(err)
	}

	if actual != expected {
		t.Errorf("Expected %d but found %d", expected, actual)
	}
}

func assertReadDoubleText(t *testing.T, expected float64, line string) {
	actual, err := readDoubleText(line)
	if err != nil {
		t.Error(err)
	}

	if actual != expected {
		t.Errorf("Expected %f but found %f", expected, actual)
	}
}

func assertReadStringText(t *testing.T, expected, line string, readAsUtf8 bool) {
	actual, err := readStringText(line, readAsUtf8)
	if err != nil {
		t.Error(err)
	}

	if actual != expected {
		t.Errorf("Expected '%s' but found '%s'", expected, actual)
	}
}

func assertReadBoolBinary(t *testing.T, expected bool, data []byte, isPostR13 bool) {
	actual, err := readBoolBinary(data, isPostR13)
	if err != nil {
		t.Error(err)
	}

	if actual != expected {
		t.Errorf("Expected %v but found %v", expected, actual)
	}
}

func assertReadShortBinary(t *testing.T, expected int16, data []byte) {
	actual, err := readShortBinary(data)
	if err != nil {
		t.Error(err)
	}

	if actual != expected {
		t.Errorf("Expected %v but found %v", expected, actual)
	}
}

func assertReadIntBinary(t *testing.T, expected int, data []byte) {
	actual, err := readIntBinary(data)
	if err != nil {
		t.Error(err)
	}

	if actual != expected {
		t.Errorf("Expected %v but found %v", expected, actual)
	}
}

func assertReadLongBinary(t *testing.T, expected int64, data []byte) {
	actual, err := readLongBinary(data)
	if err != nil {
		t.Error(err)
	}

	if actual != expected {
		t.Errorf("Expected %v but found %v", expected, actual)
	}
}

func assertReadDoubleBinary(t *testing.T, expected float64, data []byte) {
	actual, err := readDoubleBinary(data)
	if err != nil {
		t.Error(err)
	}

	if actual != expected {
		t.Errorf("Expected %v but found %v", expected, actual)
	}
}

func assertReadStringBinary(t *testing.T, expected string, data []byte) {
	buf := bytes.NewBuffer(data)
	reader := bufio.NewReader(buf)
	actual, err := readStringBinary(reader)
	if err != nil {
		t.Error(err)
	}

	if actual != expected {
		t.Errorf("Expected %v but found %v", expected, actual)
	}
}

func readCodePairsFromReader(t *testing.T, reader io.Reader) (codePairs []CodePair) {
	r, err := codePairReaderFromReader(reader, encoding.Nop)
	if err != nil {
		t.Error(err)
	}

	nextPair, err := r.readCodePair()
	for err == nil {
		codePairs = append(codePairs, nextPair)
		nextPair, err = r.readCodePair()
	}

	return
}

func readCodePairsText(t *testing.T, content string) (codePairs []CodePair) {
	stringReader := bufio.NewReader(strings.NewReader(content))
	reader := textCodePairReader{
		reader:        stringReader,
		firstLine:     "",
		firstLineRead: true,
	}

	nextPair, err := reader.readCodePair()
	for err == nil {
		codePairs = append(codePairs, nextPair)
		nextPair, err = reader.readCodePair()
	}

	return
}

func readCodePairsBinary(t *testing.T, data []byte, asPostR13 bool) (codePairs []CodePair) {
	buf := bytes.NewBuffer(data)
	binaryReader := bufio.NewReader(buf)
	reader := binaryCodePairReader{
		reader:          binaryReader,
		hasReturnedPair: true,
		isPostR13:       asPostR13,
	}

	nextPair, err := reader.readCodePair()
	for err == nil {
		codePairs = append(codePairs, nextPair)
		nextPair, err = reader.readCodePair()
	}

	return
}

func assertReadCodePairText(t *testing.T, expected CodePair, content string) {
	codePairs := readCodePairsText(t, content)
	assertEqCodePairs(t, []CodePair{expected}, codePairs)
}

func assertReadCodePairBinary(t *testing.T, expected CodePair, data []byte, asPostR13 bool) {
	codePairs := readCodePairsBinary(t, data, asPostR13)
	assertEqCodePairs(t, []CodePair{expected}, codePairs)
}

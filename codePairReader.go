package dxf

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

type codePairReader interface {
	readCodePair() (CodePair, error)
	setUtf8Reader()
	// setCodePage switches pre-2007 text decoding to the `$DWGCODEPAGE` encoding, unless the caller chose an encoding
	// or the file is UTF-8.
	setCodePage(name string)
}

const readerBufferSize = 64 * 1024

func codePairReaderFromReader(reader io.Reader, e encoding.Encoding) (r codePairReader, err error) {
	// one buffered reader is shared by the format sniffing below and the actual code pair reader; the bytes it reads
	// are counted for the positions in errors
	counter := &byteCountingReader{reader: reader}
	file := bufio.NewReaderSize(counter, readerBufferSize)
	consumed := func() int64 { return counter.count - int64(file.Buffered()) }

	if e == nil {
		// no explicit encoding, like ReadFile
		e = encoding.Nop
	}
	var decoder *encoding.Decoder
	if e != encoding.Nop {
		decoder = e.NewDecoder()
	}

	buffered := file
	start, _ := file.Peek(4096)
	switch {
	case bytes.HasPrefix(start, []byte{0xFF, 0xFE}) || bytes.HasPrefix(start, []byte{0xFE, 0xFF}):
		// UTF-16 with a byte order mark: read as UTF-8, whatever the caller or the header says
		utf16 := unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewDecoder()
		buffered = bufio.NewReaderSize(transform.NewReader(file, utf16), readerBufferSize)
		decoder = unicode.UTF8.NewDecoder()
	case bytes.IndexByte(start, '\r') >= 0 && bytes.IndexByte(start, '\n') < 0:
		// lines end with a bare CR (classic Mac OS); values can't contain one, a CR in a value is written as ^M
		buffered = bufio.NewReaderSize(&carriageReturnReader{reader: file}, readerBufferSize)
	}

	var scratch []byte
	firstLineBytes, err := readLineBytes(buffered, &scratch)
	if err == io.EOF {
		// empty file is valid
		return newDirectCodePairReader(), nil
	}
	if err != nil {
		return newDirectCodePairReader(), err
	}
	firstLine, err := decodeLine(bytes.TrimPrefix(firstLineBytes, utf8ByteOrderMark), decoder, false)
	if err != nil {
		return newDirectCodePairReader(), err
	}

	if firstLine == "AutoCAD Binary DXF" {
		binaryReader, err := newBinaryCodePairReader(buffered, decoder)
		if err != nil {
			return nil, err
		}
		binaryReader.(*binaryCodePairReader).consumed = consumed
		r = binaryReader
	} else {
		r = newTextCodePairReader(buffered, decoder, firstLine)
	}

	return &commentFilteringReader{inner: r}, nil
}

// carriageReturnReader turns the bare CR line endings of a text DXF into LF.
type carriageReturnReader struct {
	reader io.Reader
}

func (c *carriageReturnReader) Read(p []byte) (int, error) {
	n, err := c.reader.Read(p)
	for i := range p[:n] {
		if p[i] == '\r' {
			p[i] = '\n'
		}
	}
	return n, err
}

// byteCountingReader counts the bytes read through it.
type byteCountingReader struct {
	reader io.Reader
	count  int64
}

func (c *byteCountingReader) Read(p []byte) (int, error) {
	n, err := c.reader.Read(p)
	c.count += int64(n)
	return n, err
}

// positionReporter is implemented by the readers that know where in the file they are: the line of the last value
// read (text DXF), or the byte offset of the last code pair read (binary DXF).
type positionReporter interface {
	position() (line int, offset int64)
}

// readerPosition returns the position of a reader, or false if it doesn't know it.
func readerPosition(reader codePairReader) (line int, offset int64, ok bool) {
	if reporter, isReporter := reader.(positionReporter); isReporter {
		line, offset = reporter.position()
		return line, offset, line > 0 || offset >= 0
	}
	return 0, -1, false
}

// commentFilteringReader wraps a codePairReader and silently skips group code 999 (comment) pairs.
type commentFilteringReader struct {
	inner codePairReader
}

func (r *commentFilteringReader) position() (int, int64) {
	line, offset, _ := readerPosition(r.inner)
	return line, offset
}

func (r *commentFilteringReader) readCodePair() (CodePair, error) {
	for {
		pair, err := r.inner.readCodePair()
		if err != nil || pair.Code != 999 {
			return pair, err
		}
	}
}

func (r *commentFilteringReader) setUtf8Reader() {
	r.inner.setUtf8Reader()
}

func (r *commentFilteringReader) setCodePage(name string) {
	r.inner.setCodePage(name)
}

// applicationGroupFilteringReader wraps a codePairReader and skips application groups: 102/{NAME up to 102/}, e.g.
// {ACAD_REACTORS (330s), {ACAD_XDICTIONARY (360) or {BLKREFS (331s). Their handles would otherwise be read as the
// item's own codes, like the owner (330) or a viewport's frozen layers (331).
type applicationGroupFilteringReader struct {
	inner codePairReader
}

func (r *applicationGroupFilteringReader) readCodePair() (CodePair, error) {
	pair, err := r.inner.readCodePair()
	for err == nil && isApplicationGroupStart(pair) {
		// skip to the closing 102/}; a group that isn't closed ends with the item
		for err == nil && pair.Code != 0 && !(pair.Code == 102 && stringValue(pair) == "}") {
			pair, err = r.inner.readCodePair()
		}
		if err == nil && pair.Code == 102 {
			pair, err = r.inner.readCodePair()
		}
	}
	return pair, err
}

func isApplicationGroupStart(pair CodePair) bool {
	return pair.Code == 102 && strings.HasPrefix(stringValue(pair), "{")
}

func (r *applicationGroupFilteringReader) setUtf8Reader() {
	r.inner.setUtf8Reader()
}

func (r *applicationGroupFilteringReader) setCodePage(name string) {
	r.inner.setCodePage(name)
}

// code pairs
type directCodePairReader struct {
	index     int
	codePairs []CodePair
}

func newDirectCodePairReader(codePairs ...CodePair) codePairReader {
	return &directCodePairReader{
		index:     0,
		codePairs: codePairs,
	}
}

func (d *directCodePairReader) readCodePair() (codePair CodePair, err error) {
	if d.index >= len(d.codePairs) {
		err = errors.New("out of data")
	} else {
		codePair = d.codePairs[d.index]
		d.index++
	}

	return codePair, err
}

func (d *directCodePairReader) setUtf8Reader() {
	// noop
}

func (d *directCodePairReader) setCodePage(name string) {
	// noop
}

// text
// stringDecoder turns raw string values into Go strings for both text and binary files: pre-2007 values are in the
// caller's encoding or the `$DWGCODEPAGE` one with `\U+XXXX` escapes, 2007+ values are UTF-8.
type stringDecoder struct {
	decoder          *encoding.Decoder // nil when no decoding is needed
	explicitEncoding bool              // the caller chose the encoding; the header must not override it
	preferUtf8       bool              // keep values that are valid UTF-8 as they are
	readAsUtf8       bool
}

func newStringDecoder(decoder *encoding.Decoder) stringDecoder {
	return stringDecoder{
		decoder:          decoder,
		explicitEncoding: decoder != nil,
	}
}

func (s *stringDecoder) decodeString(raw []byte) (string, error) {
	value, err := decodeLine(raw, s.decoder, s.readAsUtf8 || s.preferUtf8)
	if err != nil {
		return "", err
	}
	return readStringText(unescapeCarets(value), s.readAsUtf8)
}

// unescapeCarets decodes the caret escapes of control characters (^J is a newline, ^I a tab, ^@ to ^_ are 0x00 to
// 0x1F) and of the caret itself (^ followed by a space). A caret before anything else is kept as it is.
func unescapeCarets(s string) string {
	if !strings.Contains(s, "^") {
		return s
	}
	var builder strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '^' && i+1 < len(s) {
			next := s[i+1]
			switch {
			case next >= '@' && next <= '_':
				builder.WriteByte(next - '@')
				i++
				continue
			case next == ' ':
				builder.WriteByte('^')
				i++
				continue
			}
		}
		builder.WriteByte(s[i])
	}
	return builder.String()
}

func (s *stringDecoder) setUtf8Reader() {
	s.decoder = unicode.UTF8.NewDecoder()
	s.readAsUtf8 = true
}

func (s *stringDecoder) setCodePage(name string) {
	if s.readAsUtf8 || s.explicitEncoding {
		return
	}
	if e := encodingFromCodePage(name); e != nil {
		s.decoder = e.NewDecoder()
		// the code page is only a hint: some writers put UTF-8 into pre-2007 files, which no Windows code page text
		// is likely to be valid as
		s.preferUtf8 = true
	}
}

type textCodePairReader struct {
	stringDecoder
	reader        *bufio.Reader
	firstLine     string
	firstLineRead bool
	scratch       []byte
	line          int // the number of the last line read, from 1
}

func (a *textCodePairReader) position() (int, int64) {
	return a.line, -1
}

var utf8ByteOrderMark = []byte{0xEF, 0xBB, 0xBF}

func newTextCodePairReader(reader *bufio.Reader, decoder *encoding.Decoder, firstLine string) codePairReader {
	return &textCodePairReader{
		stringDecoder: newStringDecoder(decoder),
		reader:        reader,
		firstLine:     firstLine,
		firstLineRead: false,
	}
}

// readLineBytes returns the next line without its line ending, or io.EOF when no data is left. The returned slice is
// only valid until the next read.
func readLineBytes(reader *bufio.Reader, scratch *[]byte) ([]byte, error) {
	line, err := reader.ReadSlice('\n')
	if err == bufio.ErrBufferFull {
		// the line is longer than the buffer; collect it in the scratch space
		buffer := append((*scratch)[:0], line...)
		for err == bufio.ErrBufferFull {
			line, err = reader.ReadSlice('\n')
			buffer = append(buffer, line...)
		}
		*scratch = buffer
		line = buffer
	}
	if err == io.EOF && len(line) > 0 {
		// last line without a line ending
		err = nil
	}
	if err != nil {
		return nil, err
	}

	line = bytes.TrimSuffix(line, []byte{'\n'})
	line = bytes.TrimSuffix(line, []byte{'\r'})
	return line, nil
}

// decodeLine converts raw line bytes into a string, only running the decoder when the bytes need it.
func decodeLine(line []byte, decoder *encoding.Decoder, isUtf8 bool) (string, error) {
	if decoder == nil || isASCII(line) || (isUtf8 && utf8.Valid(line)) {
		return string(line), nil
	}

	decoded, err := decoder.Bytes(line)
	return string(decoded), err
}

func isASCII(data []byte) bool {
	for _, b := range data {
		if b >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

func (a *textCodePairReader) readRawLine() ([]byte, error) {
	if !a.firstLineRead {
		a.firstLineRead = true
		a.line = 1
		line := []byte(a.firstLine)
		a.firstLine = ""
		return line, nil
	}

	line, err := readLineBytes(a.reader, &a.scratch)
	if err == nil {
		a.line++
	}
	return line, err
}

func (a *textCodePairReader) readCode() (int, error) {
	line, err := a.readRawLine()
	if err != nil {
		return 0, err
	}

	return parseCode(line)
}

// parseCode parses a group code without allocating; leading and trailing whitespace is ignored.
func parseCode(line []byte) (int, error) {
	trimmed := bytes.TrimSpace(line)
	digits := trimmed
	negative := false
	if len(digits) > 0 && (digits[0] == '-' || digits[0] == '+') {
		negative = digits[0] == '-'
		digits = digits[1:]
	}
	if len(digits) == 0 || len(digits) > 9 {
		return strconv.Atoi(string(trimmed))
	}

	code := 0
	for _, c := range digits {
		if c < '0' || c > '9' {
			return strconv.Atoi(string(trimmed))
		}
		code = code*10 + int(c-'0')
	}
	if negative {
		code = -code
	}
	return code, nil
}

func readBoolText(line string) (bool, error) {
	value, err := readShortText(line)
	result := value != 0
	return result, err
}

func readShortText(line string) (int16, error) {
	value, err := readLongText(line)
	result := int16(value)
	return result, err
}

func readIntText(line string) (int, error) {
	value, err := readLongText(line)
	result := int(value)
	return result, err
}

func readLongText(line string) (int64, error) {
	trimmed := strings.TrimSpace(line)
	value, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil {
		// some writers put integers as decimals ("1.0"); the fraction is dropped
		if f, floatErr := strconv.ParseFloat(trimmed, 64); floatErr == nil && math.Abs(f) < 1<<63 {
			return int64(f), nil
		}
	}
	return value, err
}

func readDoubleText(line string) (float64, error) {
	return strconv.ParseFloat(strings.TrimSpace(line), 64)
}

func readStringText(line string, readAsUtf8 bool) (value string, err error) {
	if !readAsUtf8 {
		line = parseUtf8(line)
	}

	return line, nil
}

func (a *textCodePairReader) readCodePair() (CodePair, error) {
	var codePair CodePair
	code, err := a.readCode()
	if err != nil {
		return codePair, err
	}

	rawValue, err := a.readRawLine()
	if err != nil {
		return codePair, err
	}

	typeName := codeTypeName(code)
	if typeName == "String" || typeName == "Unknown" {
		// codes outside the official ranges are kept as text instead of failing the whole file
		value, err := a.decodeString(rawValue)
		if err != nil {
			return codePair, err
		}
		return NewStringCodePair(code, value), nil
	}

	// numbers are plain ASCII
	stringValue := string(rawValue)
	switch typeName {
	case "Bool":
		value, err := readBoolText(stringValue)
		if err != nil {
			return codePair, err
		}
		codePair = NewBoolCodePair(code, value)
	case "Double":
		value, err := readDoubleText(stringValue)
		if err != nil {
			return codePair, err
		}
		codePair = NewDoubleCodePair(code, value)
	case "Int":
		value, err := readIntText(stringValue)
		if err != nil {
			return codePair, err
		}
		codePair = NewIntCodePair(code, value)
	case "Long":
		value, err := readLongText(stringValue)
		if err != nil {
			return codePair, err
		}
		codePair = NewLongCodePair(code, value)
	case "Short":
		value, err := readShortText(stringValue)
		if err != nil {
			return codePair, err
		}
		codePair = NewShortCodePair(code, value)
	}

	return codePair, nil
}

// binary
type binaryCodePairReader struct {
	stringDecoder
	reader          *bufio.Reader
	hasReturnedPair bool
	isPostR13       bool
	// consumed returns the number of bytes read from the file so far; pairOffset is where the last code pair started
	consumed   func() int64
	pairOffset int64
}

func (b *binaryCodePairReader) position() (int, int64) {
	if b.consumed == nil {
		return 0, -1
	}
	return 0, b.pairOffset
}

func newBinaryCodePairReader(r *bufio.Reader, decoder *encoding.Decoder) (rdr codePairReader, err error) {
	buf, err := readBytes(r, 2)
	if err != nil {
		return
	}
	if buf[0] != 0x1A || buf[1] != 0x00 {
		err = errors.New("expected 0x1A, 0x00")
		return
	}
	rdr = &binaryCodePairReader{
		stringDecoder:   newStringDecoder(decoder),
		reader:          r,
		hasReturnedPair: false,
		isPostR13:       false,
	}
	return
}

// readBytes reads exactly count bytes; a single Read may legally return fewer at buffer boundaries.
func readBytes(reader *bufio.Reader, count int) (buf []byte, err error) {
	buf = make([]byte, count)
	_, err = io.ReadFull(reader, buf)
	if err == io.ErrUnexpectedEOF {
		err = errors.New("not enough bytes")
	}
	return
}

func readBoolBinary(data []byte, isPostR13 bool) (val bool, err error) {
	// after R13 bools are encoded as a single byte
	if isPostR13 {
		if len(data) != 1 {
			err = errors.New("Expected 1 byte to read post R13 bool.")
			return
		}

		val = data[0] != 0
	} else {
		if len(data) != 2 {
			err = errors.New("Expected 2 bytes to read pre R13 bool.")
			return
		}

		s := createShort(data[0], data[1])
		val = s != 0
	}

	return
}

func readShortBinary(data []byte) (val int16, err error) {
	if len(data) != 2 {
		err = errors.New("Expected 2 bytes to read int16.")
		return
	}

	val = createShort(data[0], data[1])
	return
}

func readIntBinary(data []byte) (val int, err error) {
	if len(data) != 4 {
		err = errors.New("Expected 4 bytes to read int.")
		return
	}

	uval := binary.LittleEndian.Uint32(data)
	val = int(uval)
	return
}

func readLongBinary(data []byte) (val int64, err error) {
	if len(data) != 8 {
		err = errors.New("Expected 8 bytes to read int64.")
		return
	}

	uval := binary.LittleEndian.Uint64(data)
	val = int64(uval)
	return
}

func readDoubleBinary(data []byte) (val float64, err error) {
	if len(data) != 8 {
		err = errors.New("Expected 8 bytes to read float64.")
		return
	}

	uval := binary.LittleEndian.Uint64(data)
	val = math.Float64frombits(uval)
	return
}

func readStringBinary(reader *bufio.Reader) (val string, err error) {
	raw, err := readRawStringBinary(reader)
	val = string(raw)
	return
}

// readRawStringBinary reads a NUL-terminated string without decoding it.
func readRawStringBinary(reader *bufio.Reader) ([]byte, error) {
	buf, err := reader.ReadBytes(0x00)
	if err != nil {
		return nil, err
	}

	return buf[:len(buf)-1], nil
}

// isBinaryChunkCode reports whether a code holds binary data: hex text in text files, a length byte followed by that
// many bytes in binary files.
func isBinaryChunkCode(code int) bool {
	return between(code, 310, 319) || code == 1004
}

// readBinaryChunk reads a length-prefixed binary chunk and returns it as hex text, the way text files store it.
func readBinaryChunk(reader *bufio.Reader) (string, error) {
	length, err := reader.ReadByte()
	if err != nil {
		return "", err
	}
	data, err := readBytes(reader, int(length))
	if err != nil {
		return "", err
	}
	return strings.ToUpper(hex.EncodeToString(data)), nil
}

func (b *binaryCodePairReader) readCodePair() (CodePair, error) {
	var pair CodePair
	var err error
	if b.consumed != nil {
		b.pairOffset = b.consumed()
	}
	code, err := b.readCode()
	if err != nil {
		return pair, err
	}

	if isBinaryChunkCode(code) {
		value, err := readBinaryChunk(b.reader)
		if err != nil {
			return pair, err
		}
		return NewStringCodePair(code, value), nil
	}

	switch codeTypeName(code) {
	case "Bool":
		boolByteCount := 2
		if b.isPostR13 {
			boolByteCount = 1
		}
		data, err := readBytes(b.reader, boolByteCount)
		if err != nil {
			return pair, err
		}
		value, err := readBoolBinary(data, b.isPostR13)
		if err != nil {
			return pair, err
		}
		pair = NewBoolCodePair(code, value)
	case "Double":
		data, err := readBytes(b.reader, 8)
		if err != nil {
			return pair, err
		}
		value, err := readDoubleBinary(data)
		if err != nil {
			return pair, err
		}
		pair = NewDoubleCodePair(code, value)
	case "Int":
		data, err := readBytes(b.reader, 4)
		if err != nil {
			return pair, err
		}
		value, err := readIntBinary(data)
		if err != nil {
			return pair, err
		}
		pair = NewIntCodePair(code, value)
	case "Long":
		data, err := readBytes(b.reader, 8)
		if err != nil {
			return pair, err
		}
		value, err := readLongBinary(data)
		if err != nil {
			return pair, err
		}
		pair = NewLongCodePair(code, value)
	case "Short":
		data, err := readBytes(b.reader, 2)
		if err != nil {
			return pair, err
		}
		value, err := readShortBinary(data)
		if err != nil {
			return pair, err
		}
		pair = NewShortCodePair(code, value)
	case "String":
		raw, err := readRawStringBinary(b.reader)
		if err != nil {
			return pair, err
		}
		value, err := b.decodeString(raw)
		if err != nil {
			return pair, err
		}
		pair = NewStringCodePair(code, value)
	default:
		// the size of a value of unknown type isn't known, so nothing after it can be read
		return pair, fmt.Errorf("unknown group code %d in binary DXF", code)
	}

	return pair, err
}

func (b *binaryCodePairReader) readCode() (code int, err error) {
	bt, err := b.readByte()
	if err != nil {
		return
	}
	code = int(bt)

	if !b.hasReturnedPair && code == 0 {
		p := make([]byte, 1)
		p, err = b.reader.Peek(1)
		if err != nil {
			return
		}
		if p[0] == 0 {
			// The first code pair in a binary file must be `0/SECTION`; if we're reading the first pair, the code is
			// `0`, and the next byte is NULL (empty string), then this must be a post R13 file where codes are always
			// encoded with 2 bytes.
			b.isPostR13 = true
		}
	}

	// potentially read the second byte of the code
	if b.isPostR13 {
		var b2 byte
		b2, err = b.readByte()
		if err != nil {
			return
		}
		code = int(createShort(bt, b2))
	} else if code == 255 {
		var data []byte
		data, err = readBytes(b.reader, 2)
		if err != nil {
			return
		}
		var s int16
		s, err = readShortBinary(data)
		if err != nil {
			return
		}
		code = int(s)
	}

	b.hasReturnedPair = true
	return
}

func (b *binaryCodePairReader) readByte() (byte, error) {
	return b.reader.ReadByte()
}

func createShort(b1, b2 byte) int16 {
	return int16(b2)<<8 + int16(b1)
}

// parseUtf8 decodes the `\U+XXXX` escapes that pre-2007 files use for characters outside their code page. Other
// backslash sequences (e.g. MTEXT formatting like `\P`) are kept as they are, and invalid UTF-8 bytes become U+FFFD.
func parseUtf8(v string) string {
	if utf8.ValidString(v) && !strings.Contains(v, `\U+`) {
		return v
	}

	var final strings.Builder
	final.Grow(len(v))
	for i := 0; i < len(v); {
		if v[i] == '\\' && i+7 <= len(v) && v[i+1] == 'U' && v[i+2] == '+' && isHexDigits(v[i+3:i+7]) {
			code, _ := strconv.ParseUint(v[i+3:i+7], 16, 32)
			final.WriteRune(rune(code))
			i += 7
			continue
		}

		r, size := utf8.DecodeRuneInString(v[i:])
		final.WriteRune(r) // utf8.RuneError for an invalid byte
		i += size
	}

	return final.String()
}

func isHexDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return false
		}
	}
	return true
}

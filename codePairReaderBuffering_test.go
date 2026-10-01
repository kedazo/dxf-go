package dxf

import (
	"fmt"
	"io"
	"strings"
	"testing"
	"testing/iotest"
)

// countingReader counts how often the underlying reader is asked for data.
type countingReader struct {
	reader io.Reader
	reads  int
}

func (c *countingReader) Read(p []byte) (int, error) {
	c.reads++
	return c.reader.Read(p)
}

func linesDrawing(lineCount int) string {
	var builder strings.Builder
	builder.WriteString("  0\r\nSECTION\r\n  2\r\nENTITIES\r\n")
	for i := 0; i < lineCount; i++ {
		fmt.Fprintf(&builder, "  0\r\nLINE\r\n  8\r\nWALL\r\n 10\r\n%d.5\r\n 20\r\n2.5\r\n 11\r\n3.25\r\n 21\r\n4.75\r\n", i)
	}
	builder.WriteString("  0\r\nENDSEC\r\n  0\r\nEOF\r\n")
	return builder.String()
}

func TestReadFromReaderIsBuffered(t *testing.T) {
	content := linesDrawing(20000)
	reader := &countingReader{reader: strings.NewReader(content)}
	drawing, err := ReadFromReader(reader)
	if err != nil {
		t.Fatal(err)
	}
	assertEqInt(t, 20000, len(drawing.Entities))
	maxReads := len(content)/4096 + 10
	assert(t, reader.reads <= maxReads, fmt.Sprintf("expected at most %d reads for %d bytes, got %d", maxReads, len(content), reader.reads))
}

func TestReadFromOneByteReader(t *testing.T) {
	drawing, err := ReadFromReader(iotest.OneByteReader(strings.NewReader(linesDrawing(3))))
	if err != nil {
		t.Fatal(err)
	}
	assertEqInt(t, 3, len(drawing.Entities))
	assertEqPoint(t, Point{2.5, 2.5, 0.0}, drawing.Entities[2].(*Line).P1)
}

func TestReadLineLongerThanBuffer(t *testing.T) {
	long := strings.Repeat("x", 3*readerBufferSize+17)
	drawing := parse(t, join(
		"0", "SECTION",
		"2", "ENTITIES",
		"0", "TEXT",
		"1", long,
		"0", "ENDSEC",
		"0", "EOF",
	))
	assertEqString(t, long, drawing.Entities[0].(*Text).Value)
}

func TestReadWithByteOrderMarkAndUnixNewlines(t *testing.T) {
	content := "\xEF\xBB\xBF  0\nSECTION\n  2\nENTITIES\n  0\nLINE\n 10\n1.0\n  0\nENDSEC\n  0\nEOF"
	drawing, err := ParseDrawing(content)
	if err != nil {
		t.Fatal(err)
	}
	assertEqInt(t, 1, len(drawing.Entities))
	assertEqPoint(t, Point{1.0, 0.0, 0.0}, drawing.Entities[0].(*Line).P1)
}

func BenchmarkReadFromReader(b *testing.B) {
	content := linesDrawing(20000)
	b.SetBytes(int64(len(content)))
	for i := 0; i < b.N; i++ {
		if _, err := ReadFromReader(strings.NewReader(content)); err != nil {
			b.Fatal(err)
		}
	}
}

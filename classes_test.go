package dxf

import (
	"encoding/binary"
	"encoding/hex"
	"strings"
	"testing"
)

func TestReadClasses(t *testing.T) {
	drawing := parse(t, join(
		"  0", "SECTION", "  2", "HEADER", "  9", "$ACADVER", "  1", "AC1018", "  0", "ENDSEC",
		"  0", "SECTION", "  2", "CLASSES",
		"  0", "CLASS",
		"  1", "WIPEOUT", "  2", "AcDbWipeout", "  3", "WipeOut|Product Desc: Object Enabler",
		" 90", "2175", " 91", "3", "280", "0", "281", "1",
		"  0", "CLASS",
		"  1", "ACDBDICTIONARYWDFLT", "  2", "AcDbDictionaryWithDefault", "  3", "ObjectDBX Classes",
		" 90", "0", " 91", "1", "280", "1", "281", "0",
		"  0", "ENDSEC",
		"  0", "EOF",
	))
	assertEqInt(t, 2, len(drawing.Classes))
	wipeout := drawing.Classes[0]
	assertEqString(t, "WIPEOUT", wipeout.RecordName)
	assertEqString(t, "AcDbWipeout", wipeout.ClassName)
	assertEqString(t, "WipeOut|Product Desc: Object Enabler", wipeout.ApplicationName)
	assertEqInt(t, 2175, wipeout.ProxyCapabilities)
	assertEqInt(t, 3, wipeout.InstanceCount)
	assertEqBool(t, false, wipeout.WasProxy)
	assertEqBool(t, true, wipeout.IsEntity)
	assertEqBool(t, true, drawing.Classes[1].WasProxy)

	// R13 classes start with their record name and have no code 3
	r13 := parse(t, join(
		"  0", "SECTION", "  2", "HEADER", "  9", "$ACADVER", "  1", "AC1012", "  0", "ENDSEC",
		"  0", "SECTION", "  2", "CLASSES",
		"  0", "DICTIONARYVAR", "  1", "AcDbDictionaryVar", "  2", "ObjectDBX Classes", " 90", "1", "280", "0", "281", "0",
		"  0", "ENDSEC",
		"  0", "EOF",
	))
	assertEqInt(t, 1, len(r13.Classes))
	assertEqString(t, "DICTIONARYVAR", r13.Classes[0].RecordName)
	assertEqString(t, "AcDbDictionaryVar", r13.Classes[0].ClassName)
	assertEqString(t, "ObjectDBX Classes", r13.Classes[0].ApplicationName)
	assertEqInt(t, 1, r13.Classes[0].VersionNumber)
}

// dib returns a BITMAPINFOHEADER image of 2x2 pixels with the given bits per pixel and palette.
func dib(bitCount uint16, paletteCount uint32, palette, pixels []byte) []byte {
	header := make([]byte, 40)
	binary.LittleEndian.PutUint32(header[0:], 40)
	binary.LittleEndian.PutUint32(header[4:], 2)
	binary.LittleEndian.PutUint32(header[8:], 2)
	binary.LittleEndian.PutUint16(header[12:], 1)
	binary.LittleEndian.PutUint16(header[14:], bitCount)
	binary.LittleEndian.PutUint32(header[32:], paletteCount)
	return append(append(header, palette...), pixels...)
}

func TestReadThumbnail(t *testing.T) {
	// 24 bits: rows of 2 BGR pixels padded to 8 bytes
	image := dib(24, 0, nil, []byte{0, 0, 255, 0, 255, 0, 0, 0, 255, 0, 0, 255, 255, 255, 0, 0})
	encoded := strings.ToUpper(hex.EncodeToString(image))
	drawing := parse(t, join(
		"  0", "SECTION", "  2", "THUMBNAILIMAGE",
		" 90", "72",
		"310", encoded[:40],
		"310", encoded[40:],
		"  0", "ENDSEC",
		"  0", "EOF",
	))
	assertEqString(t, hex.EncodeToString(image), hex.EncodeToString(drawing.Thumbnail))

	bmp := drawing.ThumbnailBMP()
	assertEqString(t, "BM", string(bmp[:2]))
	assertEqInt(t, 14+len(image), int(binary.LittleEndian.Uint32(bmp[2:])))
	assertEqInt(t, 54, int(binary.LittleEndian.Uint32(bmp[10:])))
	assertEqString(t, hex.EncodeToString(image), hex.EncodeToString(bmp[14:]))

	// 8 bits with the palette size left at 0: 256 colours
	palette := make([]byte, 256*4)
	indexed := Drawing{Thumbnail: dib(8, 0, palette, make([]byte, 8))}
	assertEqInt(t, 54+1024, int(binary.LittleEndian.Uint32(indexed.ThumbnailBMP()[10:])))
	// 8 bits with 2 colours
	twoColours := Drawing{Thumbnail: dib(8, 2, make([]byte, 8), make([]byte, 8))}
	assertEqInt(t, 54+8, int(binary.LittleEndian.Uint32(twoColours.ThumbnailBMP()[10:])))

	// no or broken image data
	assert(t, (&Drawing{}).ThumbnailBMP() == nil, "expected no thumbnail")
	assert(t, (&Drawing{Thumbnail: []byte{1, 2, 3, 4, 5}}).ThumbnailBMP() == nil, "expected no thumbnail for a stub")
}

package dxf

import (
	"encoding/binary"
	"encoding/hex"
	"strings"
)

// Class is an entry of the CLASSES section: an application-defined object or entity type the drawing uses, e.g.
// WIPEOUT or ACDBDICTIONARYWDFLT. Classes are read, not written.
type Class struct {
	RecordName        string // the type name in the file, e.g. "WIPEOUT"
	ClassName         string // the C++ class, e.g. "AcDbWipeout"
	ApplicationName   string // e.g. "ObjectDBX Classes"
	ProxyCapabilities int    // code 90 (R14+): what may be done to the objects when the application is missing
	VersionNumber     int    // code 90 (R13)
	InstanceCount     int    // code 91 (R2004+)
	WasProxy          bool   // code 280: the class wasn't loaded when the file was saved
	IsEntity          bool   // code 281: the objects are entities
}

// readClassesSection reads the CLASSES section. Before R14 a class starts with 0/<record name> and its codes 1 and 2
// are the class and application names; since R14 it starts with 0/CLASS and its names are codes 1, 2 and 3.
func readClassesSection(drawing *Drawing, np CodePair, reader codePairReader) (nextPair CodePair, err error) {
	isR13 := drawing.Header.Version <= R13
	nextPair = np
	for err == nil && !nextPair.isEndSection() {
		if nextPair.Code != 0 {
			nextPair, err = reader.readCodePair()
			continue
		}

		var class Class
		if isR13 {
			class.RecordName = stringValue(nextPair)
		}
		for nextPair, err = reader.readCodePair(); err == nil && nextPair.Code != 0; nextPair, err = reader.readCodePair() {
			switch nextPair.Code {
			case 1:
				if isR13 {
					class.ClassName = stringValue(nextPair)
				} else {
					class.RecordName = stringValue(nextPair)
				}
			case 2:
				if isR13 {
					class.ApplicationName = stringValue(nextPair)
				} else {
					class.ClassName = stringValue(nextPair)
				}
			case 3:
				class.ApplicationName = stringValue(nextPair)
			case 90:
				if isR13 {
					class.VersionNumber = intValue(nextPair)
				} else {
					class.ProxyCapabilities = intValue(nextPair)
				}
			case 91:
				class.InstanceCount = intValue(nextPair)
			case 280:
				class.WasProxy = shortValue(nextPair) != 0
			case 281:
				class.IsEntity = shortValue(nextPair) != 0
			}
		}
		drawing.Classes = append(drawing.Classes, class)
	}
	return
}

// readThumbnailSection reads the THUMBNAILIMAGE section: the byte count (90) and the image as hex chunks (310).
func readThumbnailSection(drawing *Drawing, np CodePair, reader codePairReader) (nextPair CodePair, err error) {
	var data []byte
	nextPair = np
	for err == nil && !nextPair.isEndSection() && !nextPair.isEOF() {
		if nextPair.Code == 310 {
			chunk, decodeErr := hex.DecodeString(strings.TrimSpace(stringValue(nextPair)))
			if decodeErr == nil {
				data = append(data, chunk...)
			}
		}
		nextPair, err = reader.readCodePair()
	}
	drawing.Thumbnail = data
	return
}

// ThumbnailBMP returns the preview image as a BMP file, or nil if the drawing has none. The file stores the image
// without the BMP file header (a Windows DIB); ThumbnailBMP adds it.
func (d *Drawing) ThumbnailBMP() []byte {
	const fileHeaderSize = 14
	dib := d.Thumbnail
	if len(dib) < 40 {
		return nil
	}
	headerSize := int(binary.LittleEndian.Uint32(dib[0:]))
	if headerSize < 40 || headerSize > len(dib) {
		return nil
	}
	bitCount := int(binary.LittleEndian.Uint16(dib[14:]))
	paletteCount := int(binary.LittleEndian.Uint32(dib[32:]))
	if paletteCount == 0 && bitCount <= 8 {
		paletteCount = 1 << bitCount
	}

	file := make([]byte, fileHeaderSize, fileHeaderSize+len(dib))
	file[0], file[1] = 'B', 'M'
	binary.LittleEndian.PutUint32(file[2:], uint32(fileHeaderSize+len(dib)))
	binary.LittleEndian.PutUint32(file[10:], uint32(fileHeaderSize+headerSize+paletteCount*4))
	return append(file, dib...)
}

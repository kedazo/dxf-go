package dxf

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/text/encoding"
)

// The Drawing struct represents a complete DXF drawing.
type Drawing struct {
	Header Header

	AppIds       []AppId
	BlockRecords []BlockRecord
	DimStyles    []DimStyle
	Layers       []Layer
	LineTypes    []LineType
	Styles       []Style
	Ucss         []Ucs
	Views        []View
	ViewPorts    []ViewPort

	Blocks []Block

	Entities []Entity

	// Layouts, ImageDefinitions and the other objects are read from the OBJECTS section, which is not written.
	Layouts          []Layout
	ImageDefinitions []ImageDefinition
	// RasterVariables and WipeoutVariables are nil when the drawing has none.
	RasterVariables  *RasterVariables
	WipeoutVariables *WipeoutVariables
	MLeaderStyles    []MLeaderStyle
	TableStyles      []TableStyle

	// Classes and Thumbnail are read from the CLASSES and THUMBNAILIMAGE sections, which are not written. Thumbnail is
	// the preview image as stored (a BMP without its file header); see ThumbnailBMP.
	Classes   []Class
	Thumbnail []byte

	// Warnings lists what was skipped while reading because it was malformed.
	Warnings []string

	appIdTableHandle       Handle
	blockRecordTableHandle Handle
	dimStyleTableHandle    Handle
	layerTableHandle       Handle
	lineTypeTableHandle    Handle
	styleTableHandle       Handle
	ucsTableHandle         Handle
	viewTableHandle        Handle
	viewPortTableHandle    Handle
}

// NewDrawing returns a new, fully initialized drawing.
func NewDrawing() *Drawing {
	return &Drawing{
		Header:   *NewHeader(),
		Entities: make([]Entity, 0),
	}
}

// GetItemByHandle gets a `DrawingItem` with the appropriate handle.
func (d *Drawing) GetItemByHandle(h Handle) (item *DrawingItem, err error) {
	item = nil
	err = nil

	d.forEachEntity(func(e *Entity) bool {
		if (*e).Handle() == h {
			di := (*e).(DrawingItem)
			item = &di
			return false
		}
		return true
	})
	if item == nil {
		err = fmt.Errorf("Unable to find item with handle '%d'", h)
	}
	return
}

// forEachEntity calls fn for every top-level and block entity until fn returns false.
func (d *Drawing) forEachEntity(fn func(e *Entity) bool) {
	for i := range d.Entities {
		if !fn(&d.Entities[i]) {
			return
		}
	}
	for b := range d.Blocks {
		for i := range d.Blocks[b].Entities {
			if !fn(&d.Blocks[b].Entities[i]) {
				return
			}
		}
	}
}

func (d *Drawing) Normalize() {
	d.ensureViewPort("*ACTIVE")
	d.ensureBlock("*MODEL_SPACE")
	d.ensureBlock("*PAPER_SPACE")
	d.ensureDimStyle("ANNOTATIVE")
	d.ensureDimStyle("STANDARD")
	d.ensureLayer("0")
	d.ensureLineType("BYLAYER")
	d.ensureLineType("BYBLOCK")
	d.ensureLineType("CONTINUOUS")
	d.ensureStyle("STANDARD")
	d.ensureStyle("ANNOTATIVE")
	d.ensureAppId("ACAD")
	d.ensureAppId("ACADANNOTATIVE")
	d.ensureAppId("ACAD_MLEADERVER")
	d.ensureAppId("ACAD_NAV_VCDISPLAY")
}

// malformedHeaderVariable describes a header variable value that was skipped because of an unexpected group code.
func malformedHeaderVariable(variableName string, pair CodePair) string {
	return fmt.Sprintf("header variable %s: skipped a value with unexpected group code %d", variableName, pair.Code)
}

// BlockByName returns the block with the given name, compared case-insensitively like AutoCAD does, or nil.
func (d *Drawing) BlockByName(name string) *Block {
	for i := range d.Blocks {
		if strings.EqualFold(d.Blocks[i].Name, name) {
			return &d.Blocks[i]
		}
	}
	return nil
}

func (d *Drawing) ensureBlock(name string) {
	if d.BlockByName(name) != nil {
		return
	}

	block := *NewBlock()
	block.Name = name
	d.Blocks = append(d.Blocks, block)
}

func (d *Drawing) ensureDimStyle(name string) {
	for _, dimStyle := range d.DimStyles {
		if dimStyle.Name == name {
			return
		}
	}

	dimStyle := *NewDimStyle()
	dimStyle.Name = name
	d.DimStyles = append(d.DimStyles, dimStyle)
}

func (d *Drawing) ensureLayer(name string) {
	for _, layer := range d.Layers {
		if layer.Name == name {
			return
		}
	}

	layer := *NewLayer()
	layer.Name = name
	d.Layers = append(d.Layers, layer)
}

func (d *Drawing) ensureLineType(name string) {
	for _, lineType := range d.LineTypes {
		if lineType.Name == name {
			return
		}
	}

	lineType := *NewLineType()
	lineType.Name = name
	d.LineTypes = append(d.LineTypes, lineType)
}

func (d *Drawing) ensureStyle(name string) {
	for _, style := range d.Styles {
		if style.Name == name {
			return
		}
	}

	style := *NewStyle()
	style.Name = name
	d.Styles = append(d.Styles, style)
}

func (d *Drawing) ensureUcs(name string) {
	for _, ucs := range d.Ucss {
		if ucs.Name == name {
			return
		}
	}

	ucs := *NewUcs()
	ucs.Name = name
	d.Ucss = append(d.Ucss, ucs)
}

func (d *Drawing) ensureAppId(name string) {
	for _, appId := range d.AppIds {
		if appId.Name == name {
			return
		}
	}

	appId := *NewAppId()
	appId.Name = name
	d.AppIds = append(d.AppIds, appId)
}

func (d *Drawing) ensureViewPort(name string) {
	for _, viewPort := range d.ViewPorts {
		if viewPort.Name == name {
			return
		}
	}

	viewPort := *NewViewPort()
	viewPort.Name = name
	d.ViewPorts = append(d.ViewPorts, viewPort)
}

// SaveFile writes the current drawing to the specified path.
func (d *Drawing) SaveFile(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}

	return d.SaveToWriter(f)
}

// SaveFileBinary writes the current drawing to the specified path as a binary DXF.
func (d *Drawing) SaveFileBinary(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}

	return d.SaveToWriterBinary(f)
}

// SaveToWriter writes the current drawing to the specified io.Writer.
func (d *Drawing) SaveToWriter(writer io.Writer) error {
	codePairWriter := newTextCodePairWriter(writer, d.Header.Version)
	return d.saveToCodePairWriter(codePairWriter)
}

// SaveToWriterBinary writes the current drawing to the specified io.Writer as a binary DXF.
func (d *Drawing) SaveToWriterBinary(writer io.Writer) error {
	codePairWriter := newBinaryCodePairWriter(writer, d.Header.Version)
	return d.saveToCodePairWriter(codePairWriter)
}

func (d *Drawing) String() string {
	buf := new(bytes.Buffer)
	err := d.SaveToWriter(buf)
	if err != nil {
		return err.Error()
	}

	return buf.String()
}

// CodePairs returns the series of `CodePair` that represents the drawing.
func (d *Drawing) CodePairs() (codePairs []CodePair, err error) {
	writer := newDirectCodePairWriter()
	err = d.saveToCodePairWriter(&writer)
	if err != nil {
		return
	}

	codePairs = writer.CodePairs
	return
}

func (d *Drawing) saveToCodePairWriter(writer codePairWriter) error {
	err := writer.init()
	if err != nil {
		return err
	}

	d.Normalize()
	assignHandles(d)
	assignPointers(d)

	err = d.Header.writeHeaderSection(writer)
	if err != nil {
		return err
	}

	err = writeTablesSection(d, writer, d.Header.Version)
	if err != nil {
		return err
	}

	err = writeBlocksSection(d, writer)
	if err != nil {
		return err
	}

	err = writeEntitiesSection(d.Entities, writer, d.Header.Version)
	if err != nil {
		return err
	}

	err = writer.writeCodePair(NewStringCodePair(0, "EOF"))
	return err
}

// ReadFile reads a DXF drawing from the specified path.
func ReadFile(path string) (Drawing, error) {
	return ReadFileWithEncoding(path, encoding.Nop)
}

// ReadFileWithEncoding reads a DXF drawing from the specified path. Pre-2007 text is decoded with the specified
// encoding instead of the drawing's `$DWGCODEPAGE`.
func ReadFileWithEncoding(path string, e encoding.Encoding) (Drawing, error) {
	file, err := os.Open(path)
	if err != nil {
		return Drawing{}, err
	}
	defer file.Close()

	return ReadFromReaderWithEncoding(file, e)
}

// ReadFromReader reads a DXF drawing from the specified io.Reader.
func ReadFromReader(reader io.Reader) (drawing Drawing, err error) {
	return ReadFromReaderWithEncoding(reader, encoding.Nop)
}

// ReadFromReaderWithEncoding reads a DXF drawing from the specified io.Reader with the specified default text encoding.
func ReadFromReaderWithEncoding(reader io.Reader, e encoding.Encoding) (drawing Drawing, err error) {
	r, err := codePairReaderFromReader(reader, e)
	if err != nil {
		return
	}
	drawing, err = readFromCodePairReader(r)
	if line, offset, ok := readerPosition(r); err != nil && ok {
		err = &ReadError{Line: line, Offset: offset, Err: err}
	}
	return
}

// ReadError is an error found while reading a file, with where it was found: the line of the last value read in a
// text DXF, or the byte offset where the last code pair starts in a binary DXF. The drawing read up to there is
// returned with it.
type ReadError struct {
	Line   int   // text DXF, from 1; 0 for binary DXF
	Offset int64 // binary DXF; -1 for text DXF
	Err    error
}

func (e *ReadError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("line %d: %v", e.Line, e.Err)
	}
	return fmt.Sprintf("byte offset %d: %v", e.Offset, e.Err)
}

func (e *ReadError) Unwrap() error { return e.Err }

// ParseDrawing returns a drawing as parsed from a `string`.
func ParseDrawing(content string) (Drawing, error) {
	stringReader := strings.NewReader(content)
	return ReadFromReader(stringReader)
}

// ParseDrawingFromCodePairs returns a drawing as parsed from a sequence of `CodePair`.
func ParseDrawingFromCodePairs(codePairs ...CodePair) (Drawing, error) {
	directReader := newDirectCodePairReader(codePairs...)
	return readFromCodePairReader(directReader)
}

func readFromCodePairReader(reader codePairReader) (Drawing, error) {
	reader = &applicationGroupFilteringReader{inner: reader}
	drawing := *NewDrawing()

	// read sections
	nextPair, err := reader.readCodePair()

	// parse sections
	for err == nil && !nextPair.isEOF() {
		if !nextPair.isStartSection() {
			return drawing, errors.New("expected 0/SECTION code pair")
		}

		// find 2/<section-type>
		nextPair, err = reader.readCodePair()
		if err != nil {
			return drawing, err
		}
		if nextPair.Code != 2 {
			return drawing, errors.New("expected 2/<section-type>")
		}

		sectionType := nextPair.Value.(StringCodePairValue).Value
		nextPair, err = reader.readCodePair()
		for err == nil && !nextPair.isEndSection() {
			switch sectionType {
			case "ENTITIES":
				drawing.Entities, nextPair, err = readEntities(nextPair, reader)
			case "HEADER":
				var warnings []string
				drawing.Header, nextPair, warnings, err = readHeader(nextPair, reader)
				drawing.Warnings = append(drawing.Warnings, warnings...)
			case "TABLES":
				nextPair, err = readTables(&drawing, nextPair, reader)
			case "BLOCKS":
				drawing.Blocks, nextPair, err = readBlocksSection(nextPair, reader)
			case "OBJECTS":
				nextPair, err = readObjectsSection(&drawing, nextPair, reader)
			case "CLASSES":
				nextPair, err = readClassesSection(&drawing, nextPair, reader)
			case "THUMBNAILIMAGE":
				nextPair, err = readThumbnailSection(&drawing, nextPair, reader)
			default:
				// swallow unsupported section
				for err == nil && !nextPair.isEndSection() {
					nextPair, err = reader.readCodePair()
				}
			}
		}

		// find 0/ENDSEC
		if err != nil {
			return drawing, err
		}
		if !nextPair.isEndSection() {
			return drawing, errors.New("expected 0/ENDSEC")
		}

		nextPair, err = reader.readCodePair()
	}

	// find possible 0/EOF
	if err != nil {
		// don't care at this point, the file could be done
		err = nil
	} else if !nextPair.isEOF() {
		return drawing, errors.New("expected 0/EOF")
	}

	bindPointers(&drawing)
	return drawing, nil
}

func readBlocksSection(np CodePair, reader codePairReader) (blocks []Block, nextPair CodePair, err error) {
	nextPair = np
	for err == nil && !nextPair.isEndSection() {
		if nextPair.Code != 0 {
			nextPair, err = reader.readCodePair()
			continue
		}
		entityType := nextPair.Value.(StringCodePairValue).Value
		if entityType == "BLOCK" {
			var block Block
			var xdata xdataReader
			// Read block header fields
			nextPair, err = reader.readCodePair()
			for err == nil && nextPair.Code != 0 {
				xdata.add(nextPair)
				switch nextPair.Code {
				case 2, 3:
					block.Name = nextPair.Value.(StringCodePairValue).Value
				case 5:
					block.handle = handleFromString(nextPair.Value.(StringCodePairValue).Value)
				case 8:
					block.Layer = nextPair.Value.(StringCodePairValue).Value
				case 70:
					block.Flags = nextPair.Value.(ShortCodePairValue).Value
				case 10:
					block.BasePoint.X = nextPair.Value.(DoubleCodePairValue).Value
				case 20:
					block.BasePoint.Y = nextPair.Value.(DoubleCodePairValue).Value
				case 30:
					block.BasePoint.Z = nextPair.Value.(DoubleCodePairValue).Value
				case 1:
					block.XrefName = nextPair.Value.(StringCodePairValue).Value
				case 4:
					block.Description = nextPair.Value.(StringCodePairValue).Value
				case 67:
					block.IsInPaperSpace = nextPair.Value.(ShortCodePairValue).Value != 0
				}
				nextPair, err = reader.readCodePair()
			}
			block.XData = xdata.result()
			// Read block entities until ENDBLK; a missing ENDBLK must not swallow the following sections
			for err == nil && !nextPair.isEndSection() && !nextPair.isEOF() {
				if nextPair.Code == 0 {
					val := nextPair.Value.(StringCodePairValue).Value
					if val == "ENDBLK" {
						// Skip past ENDBLK's pairs
						nextPair, err = reader.readCodePair()
						for err == nil && nextPair.Code != 0 {
							nextPair, err = reader.readCodePair()
						}
						break
					}
				}
				var entity Entity
				var ok bool
				entity, nextPair, ok, err = readEntity(nextPair, reader)
				if err != nil {
					return
				}
				if ok {
					block.Entities = append(block.Entities, entity)
				}
			}
			block.Entities = collectEntities(&entityBufferReader{entities: block.Entities})
			blocks = append(blocks, block)
		} else {
			// Skip non-BLOCK entity type
			nextPair, err = reader.readCodePair()
			for err == nil && nextPair.Code != 0 {
				nextPair, err = reader.readCodePair()
			}
		}
	}
	return
}

func assignHandles(d *Drawing) {
	// table items and top-level entities keep the handles they were read with; new handles start above them (and above
	// $HANDSEED, which is above every handle of the file that was read) so they never collide
	nextHandle := uint32(max(1, d.Header.NextAvailableHandle, maxKeptHandle(d)+1))
	nextHandle = uint32(assignTableHandles(d, Handle(nextHandle)))

	for i := range d.Blocks {
		b := &d.Blocks[i]
		nextHandle = b.assignHandles(nextHandle)
	}

	for i := range d.Entities {
		e := &d.Entities[i]
		if (*e).Handle() == 0 {
			(*e).SetHandle(Handle(nextHandle))
			nextHandle++
		}
	}

	d.Header.NextAvailableHandle = Handle(nextHandle)
}

// maxKeptHandle returns the highest handle of the table items and top-level entities, which keep their handles on save.
func maxKeptHandle(d *Drawing) (highest Handle) {
	keep := func(h Handle) { highest = max(highest, h) }
	for i := range d.AppIds {
		keep(d.AppIds[i].Handle())
	}
	for i := range d.BlockRecords {
		keep(d.BlockRecords[i].Handle())
	}
	for i := range d.DimStyles {
		keep(d.DimStyles[i].Handle())
	}
	for i := range d.Layers {
		keep(d.Layers[i].Handle())
	}
	for i := range d.LineTypes {
		keep(d.LineTypes[i].Handle())
	}
	for i := range d.Styles {
		keep(d.Styles[i].Handle())
	}
	for i := range d.Ucss {
		keep(d.Ucss[i].Handle())
	}
	for i := range d.Views {
		keep(d.Views[i].Handle())
	}
	for i := range d.ViewPorts {
		keep(d.ViewPorts[i].Handle())
	}
	for _, e := range d.Entities {
		keep(e.Handle())
	}
	return
}

func assignPointers(d *Drawing) {
	d.forEachEntity(func(e *Entity) bool {
		for _, p := range (*e).pointers() {
			if p.handle == 0 && p.value != nil {
				p.handle = (*p.value).Handle()
			}
		}
		return true
	})
}

func bindPointers(d *Drawing) {
	itemsByHandle := make(map[Handle]DrawingItem)
	d.forEachEntity(func(e *Entity) bool {
		if h := (*e).Handle(); h != 0 {
			if _, exists := itemsByHandle[h]; !exists {
				itemsByHandle[h] = (*e).(DrawingItem)
			}
		}
		return true
	})
	d.forEachEntity(func(e *Entity) bool {
		for _, p := range (*e).pointers() {
			if p.handle != 0 {
				if item, ok := itemsByHandle[p.handle]; ok {
					p.value = &item
				}
			}
		}
		return true
	})
}

func writeBlocksSection(drawing *Drawing, writer codePairWriter) (err error) {
	err = writeSectionStart(writer, "BLOCKS")
	if err != nil {
		return
	}

	for i := range drawing.Blocks {
		block := &drawing.Blocks[i]
		pairs := block.getBlockPairs(drawing.Header.Version)
		for _, pair := range pairs {
			err = writer.writeCodePair(pair)
			if err != nil {
				return
			}
		}
	}

	err = writeSectionEnd(writer)
	return
}

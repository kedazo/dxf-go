package dxf

import (
	"testing"
)

func TestItemByHandle(t *testing.T) {
	drawing := parse(t, join(
		"  0", "SECTION", "  2", "HEADER", "  9", "$ACADVER", "  1", "AC1027", "  0", "ENDSEC",
		"  0", "SECTION", "  2", "TABLES",
		"  0", "TABLE", "  2", "LAYER",
		"  0", "LAYER", "  5", "10", "  2", "WALLS", "347", "60",
		"  0", "ENDTAB",
		"  0", "TABLE", "  2", "BLOCK_RECORD",
		"  0", "BLOCK_RECORD", "  5", "1F", "  2", "*Model_Space",
		"  0", "BLOCK_RECORD", "  5", "20", "  2", "DOOR",
		"  0", "ENDTAB",
		"  0", "ENDSEC",
		"  0", "SECTION", "  2", "BLOCKS",
		"  0", "BLOCK", "  5", "21", "330", "20", "  2", "DOOR", " 70", "0",
		" 10", "0.0", " 20", "0.0", " 30", "0.0",
		"  0", "LINE", "  5", "22", "330", "20", " 10", "0.0", " 11", "1.0",
		"  0", "ENDBLK", "  5", "23",
		"  0", "ENDSEC",
		"  0", "SECTION", "  2", "ENTITIES",
		"  0", "INSERT", "  5", "30", "330", "1F", " 66", "1", "  2", "DOOR",
		"  0", "ATTRIB", "  5", "31", "  2", "TAG", "  1", "value",
		"  0", "SEQEND", "  5", "32",
		"  0", "POLYLINE", "  5", "40", "330", "1F", " 66", "1",
		"  0", "VERTEX", "  5", "41", " 10", "1.0",
		"  0", "SEQEND", "  5", "42",
		"  0", "ENDSEC",
		"  0", "SECTION", "  2", "OBJECTS",
		"  0", "IMAGEDEF", "  5", "50", "  1", "photo.jpg",
		"  0", "MLEADERSTYLE", "  5", "51", "  3", "Standard",
		"  0", "ENDSEC",
		"  0", "EOF",
	))

	layer, ok := drawing.ItemByHandle(0x10).(*Layer)
	assert(t, ok && layer.Name == "WALLS", "expected the layer")
	assertEqInt(t, 0x60, int(ParseHandle(layer.MaterialHandle)))

	record, ok := drawing.ItemByHandle(0x20).(*BlockRecord)
	assert(t, ok && record.Name == "DOOR", "expected the block record")
	block, ok := drawing.ItemByHandle(0x21).(*Block)
	assert(t, ok && block.Name == "DOOR", "expected the block")
	assertEqInt(t, 0x21, int(block.Handle()))

	// an entity's owner is a BLOCK_RECORD, which its Owner() pointer can't hold
	line, ok := drawing.ItemByHandle(0x22).(*Line)
	assert(t, ok, "expected the line in the block")
	owner, ok := drawing.ItemByHandle(line.OwnerHandle()).(*BlockRecord)
	assert(t, ok && owner.Name == "DOOR", "expected the line's owner")

	insert, ok := drawing.ItemByHandle(0x30).(*Insert)
	assert(t, ok && insert.Name == "DOOR", "expected the insert")
	attribute, ok := drawing.ItemByHandle(0x31).(*Attribute)
	assert(t, ok && attribute.Value == "value", "expected the attribute")
	vertex, ok := drawing.ItemByHandle(0x41).(*Vertex)
	assert(t, ok && vertex.Location.X == 1.0, "expected the vertex")
	polyline := drawing.ItemByHandle(0x40).(*Polyline)
	assertEqInt(t, 0x1F, int(polyline.OwnerHandle()))

	definition, ok := drawing.ItemByHandle(0x50).(*ImageDefinition)
	assert(t, ok && definition.FileName == "photo.jpg", "expected the image definition")
	style, ok := drawing.ItemByHandle(0x51).(*MLeaderStyle)
	assert(t, ok && style.Description == "Standard", "expected the multileader style")

	assert(t, drawing.ItemByHandle(0x99) == nil, "expected nothing for an unknown handle")
	assert(t, drawing.ItemByHandle(0) == nil, "expected nothing for handle 0")
	assertEqInt(t, 0, int(ParseHandle("not a handle")))

	// the index hands out pointers into the drawing
	drawing.HandleIndex()[0x10].(*Layer).Name = "RENAMED"
	assertEqString(t, "RENAMED", drawing.Layers[0].Name)
}

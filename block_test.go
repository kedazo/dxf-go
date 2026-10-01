package dxf

import (
	"testing"
)

func parseBlocks(t *testing.T, body ...CodePair) []Block {
	codePairs := []CodePair{
		NewStringCodePair(0, "SECTION"),
		NewStringCodePair(2, "BLOCKS"),
	}
	codePairs = append(codePairs, body...)
	codePairs = append(codePairs,
		NewStringCodePair(0, "ENDSEC"),
		NewStringCodePair(0, "EOF"),
	)
	drawing := parseFromCodePairs(t, codePairs...)
	return drawing.Blocks
}

func TestReadBlockWithPolyline(t *testing.T) {
	blocks := parseBlocks(t,
		NewStringCodePair(0, "BLOCK"),
		NewStringCodePair(2, "B"),
		NewStringCodePair(0, "POLYLINE"),
		NewShortCodePair(66, 1),
		NewStringCodePair(0, "VERTEX"),
		NewDoubleCodePair(10, 1.0),
		NewDoubleCodePair(20, 2.0),
		NewStringCodePair(0, "VERTEX"),
		NewDoubleCodePair(10, 3.0),
		NewDoubleCodePair(20, 4.0),
		NewStringCodePair(0, "SEQEND"),
		NewStringCodePair(0, "ENDBLK"),
	)
	assertEqInt(t, 1, len(blocks))
	assertEqInt(t, 1, len(blocks[0].Entities))
	polyline := blocks[0].Entities[0].(*Polyline)
	assertEqInt(t, 2, len(polyline.Vertices))
	assertEqPoint(t, Point{1.0, 2.0, 0.0}, polyline.Vertices[0].Location)
	assertEqPoint(t, Point{3.0, 4.0, 0.0}, polyline.Vertices[1].Location)
}

func TestReadBlockWithInsertAttributes(t *testing.T) {
	blocks := parseBlocks(t,
		NewStringCodePair(0, "BLOCK"),
		NewStringCodePair(2, "B"),
		NewStringCodePair(0, "INSERT"),
		NewStringCodePair(2, "NESTED"),
		NewShortCodePair(66, 1),
		NewStringCodePair(0, "ATTRIB"),
		NewStringCodePair(2, "TAG1"),
		NewStringCodePair(0, "ATTRIB"),
		NewStringCodePair(2, "TAG2"),
		NewStringCodePair(0, "SEQEND"),
		NewStringCodePair(0, "LINE"),
		NewStringCodePair(0, "ENDBLK"),
	)
	assertEqInt(t, 2, len(blocks[0].Entities))
	insert := blocks[0].Entities[0].(*Insert)
	assertEqString(t, "NESTED", insert.Name)
	assertEqInt(t, 2, len(insert.Attributes))
	assertEqString(t, "TAG1", insert.Attributes[0].AttributeTag)
	assertEqString(t, "TAG2", insert.Attributes[1].AttributeTag)
	_ = blocks[0].Entities[1].(*Line)
}

func TestReadBlockFlags(t *testing.T) {
	blocks := parseBlocks(t,
		NewStringCodePair(0, "BLOCK"),
		NewStringCodePair(2, "*D1"),
		NewShortCodePair(70, 1),
		NewStringCodePair(0, "ENDBLK"),
		NewStringCodePair(0, "BLOCK"),
		NewStringCodePair(2, "XREF"),
		NewShortCodePair(70, 4|8|2),
		NewStringCodePair(1, "other.dwg"),
		NewStringCodePair(0, "ENDBLK"),
	)
	assertEqInt(t, 2, len(blocks))
	assertEqBool(t, true, blocks[0].IsAnonymous())
	assertEqBool(t, false, blocks[0].IsXref())
	assertEqBool(t, false, blocks[1].IsAnonymous())
	assertEqBool(t, true, blocks[1].IsXref())
	assertEqBool(t, true, blocks[1].IsXrefOverlay())
	assertEqBool(t, true, blocks[1].HasAttributeDefinitions())
	assertEqString(t, "other.dwg", blocks[1].XrefName)
}

func TestWriteBlockFlags(t *testing.T) {
	block := NewBlock()
	block.Name = "*D1"
	block.Flags = 1
	assertContainsCodePairs(t, []CodePair{
		NewStringCodePair(2, "*D1"),
		NewShortCodePair(70, 1),
	}, block.getBlockPairs(R2000))
}

func TestBlockByName(t *testing.T) {
	drawing := *NewDrawing()
	drawing.Blocks = append(drawing.Blocks, Block{Name: "Door_1"})
	block := drawing.BlockByName("DOOR_1")
	assert(t, block != nil, "expected to find block case-insensitively")
	assertEqString(t, "Door_1", block.Name)
	block.BasePoint = Point{1.0, 2.0, 3.0}
	assertEqPoint(t, Point{1.0, 2.0, 3.0}, drawing.Blocks[0].BasePoint)
	assert(t, drawing.BlockByName("missing") == nil, "expected nil for a missing block")
}

func TestNormalizeKeepsDifferentlyCasedModelSpaceBlock(t *testing.T) {
	drawing := *NewDrawing()
	drawing.Blocks = []Block{{Name: "*Model_Space"}, {Name: "*Paper_Space"}}
	drawing.Normalize()
	assertEqInt(t, 2, len(drawing.Blocks))
}

func TestReadBlockWithoutEndBlock(t *testing.T) {
	drawing := parseFromCodePairs(t,
		NewStringCodePair(0, "SECTION"),
		NewStringCodePair(2, "BLOCKS"),
		NewStringCodePair(0, "BLOCK"),
		NewStringCodePair(2, "B"),
		NewStringCodePair(0, "CIRCLE"),
		NewStringCodePair(0, "ENDSEC"),
		NewStringCodePair(0, "SECTION"),
		NewStringCodePair(2, "ENTITIES"),
		NewStringCodePair(0, "LINE"),
		NewStringCodePair(0, "ENDSEC"),
		NewStringCodePair(0, "EOF"),
	)
	assertEqInt(t, 1, len(drawing.Blocks))
	assertEqInt(t, 1, len(drawing.Blocks[0].Entities))
	_ = drawing.Blocks[0].Entities[0].(*Circle)
	assertEqInt(t, 1, len(drawing.Entities))
	_ = drawing.Entities[0].(*Line)
}

func TestReadBlockWithAttributeDefinitionAndStandaloneMText(t *testing.T) {
	blocks := parseBlocks(t,
		NewStringCodePair(0, "BLOCK"),
		NewStringCodePair(2, "B"),
		NewStringCodePair(0, "ATTDEF"),
		NewStringCodePair(2, "TAG"),
		NewStringCodePair(0, "MTEXT"),
		NewStringCodePair(1, "label"),
		NewStringCodePair(0, "ENDBLK"),
	)
	assertEqInt(t, 2, len(blocks[0].Entities))
	assertEqString(t, "TAG", blocks[0].Entities[0].(*AttributeDefinition).TextTag)
	assertEqString(t, "label", blocks[0].Entities[1].(*MText).Text)
}

package dxf

import (
	"testing"
)

func TestParseSimpleLine(t *testing.T) {
	line := parseEntity(t, "LINE",
		NewDoubleCodePair(10, 1.0),
		NewDoubleCodePair(20, 2.0),
		NewDoubleCodePair(30, 3.0),
		NewDoubleCodePair(11, 4.0),
		NewDoubleCodePair(21, 5.0),
		NewDoubleCodePair(31, 6.0),
	).(*Line)
	assertEqPoint(t, Point{1.0, 2.0, 3.0}, line.P1)
	assertEqPoint(t, Point{4.0, 5.0, 6.0}, line.P2)
}

func TestParseAlternateTypeString(t *testing.T) {
	line := parseEntity(t, "3DLINE",
		NewDoubleCodePair(10, 1.0),
		NewDoubleCodePair(20, 2.0),
		NewDoubleCodePair(30, 3.0),
		NewDoubleCodePair(11, 4.0),
		NewDoubleCodePair(21, 5.0),
		NewDoubleCodePair(31, 6.0),
	).(*Line)
	assertEqPoint(t, Point{1.0, 2.0, 3.0}, line.P1)
	assertEqPoint(t, Point{4.0, 5.0, 6.0}, line.P2)
}

func TestParseUnsupportedEntity(t *testing.T) {
	drawing := parseFromCodePairs(t,
		NewStringCodePair(0, "SECTION"),
		NewStringCodePair(2, "ENTITIES"),
		NewStringCodePair(0, "LINE"),          // supported entity
		NewStringCodePair(0, "NOT_AN_ENTITY"), // unsupported entity
		NewStringCodePair(0, "LINE"),          // supported entity
		NewStringCodePair(0, "ENDSEC"),
		NewStringCodePair(0, "EOF"),
	)
	assertEqInt(t, 3, len(drawing.Entities))
	assertEqString(t, "LINE", drawing.Entities[0].typeString())
	assertEqString(t, "NOT_AN_ENTITY", drawing.Entities[1].(*UnknownEntity).Type)
	assertEqString(t, "LINE", drawing.Entities[2].typeString())
}

func TestReadAndWriteUnknownEntity(t *testing.T) {
	drawing := parseFromCodePairs(t,
		NewStringCodePair(0, "SECTION"),
		NewStringCodePair(2, "ENTITIES"),
		NewStringCodePair(0, "PLANESURFACE"),
		NewStringCodePair(5, "2A"),
		NewStringCodePair(102, "{ACAD_XDICTIONARY"),
		NewStringCodePair(360, "2B"),
		NewStringCodePair(102, "}"),
		NewStringCodePair(330, "1F"),
		NewStringCodePair(100, "AcDbEntity"),
		NewStringCodePair(8, "SURFACES"),
		NewShortCodePair(62, 3),
		NewStringCodePair(100, "AcDbModelerGeometry"),
		NewShortCodePair(70, 1),
		NewStringCodePair(1, "ACIS data"),
		NewStringCodePair(100, "AcDbSurface"),
		NewShortCodePair(71, 6),
		NewStringCodePair(1001, "ACAD"),
		NewStringCodePair(1000, "xdata"),
		NewStringCodePair(0, "LINE"),
		NewStringCodePair(0, "ENDSEC"),
		NewStringCodePair(0, "EOF"),
	)
	assertEqInt(t, 2, len(drawing.Entities))
	unknown := drawing.Entities[0].(*UnknownEntity)
	assertEqString(t, "PLANESURFACE", unknown.Type)
	assertEqString(t, "SURFACES", unknown.Layer())
	assertEqInt(t, 3, int(unknown.Color()))
	assertEqUInt64(t, 0x2A, uint64(unknown.Handle()))
	// the subclass data and extended data are kept, the application group is not
	assertEqCodePairs(t, []CodePair{
		NewStringCodePair(100, "AcDbModelerGeometry"),
		NewShortCodePair(70, 1),
		NewStringCodePair(1, "ACIS data"),
		NewStringCodePair(100, "AcDbSurface"),
		NewShortCodePair(71, 6),
		NewStringCodePair(1001, "ACAD"),
		NewStringCodePair(1000, "xdata"),
	}, unknown.CodePairs)
	assertEqInt(t, 1, drawing.UnsupportedEntities()["PLANESURFACE"])

	written := allCodePairs(unknown, R2018)
	assertContainsCodePairs(t, []CodePair{
		NewStringCodePair(0, "PLANESURFACE"),
	}, written)
	assertContainsCodePairs(t, []CodePair{
		NewStringCodePair(8, "SURFACES"),
	}, written)
	assertContainsCodePairs(t, unknown.CodePairs, written)
	assertNotContainsCodePairs(t, []CodePair{NewStringCodePair(102, "{ACAD_XDICTIONARY")}, written)

	// entities with subclass markers can't be written to R12
	assertNotContainsCodePairs(t, []CodePair{NewStringCodePair(0, "PLANESURFACE")}, drawingCodePairsFromEntity(t, unknown, R12))
}

func TestReadUnknownEntityInBlock(t *testing.T) {
	blocks := parseBlocks(t,
		NewStringCodePair(0, "BLOCK"),
		NewStringCodePair(2, "B"),
		NewStringCodePair(0, "EXTRUDEDSURFACE"),
		NewStringCodePair(8, "NOTES"),
		NewStringCodePair(0, "LINE"),
		NewStringCodePair(0, "ENDBLK"),
	)
	assertEqInt(t, 2, len(blocks[0].Entities))
	assertEqString(t, "NOTES", blocks[0].Entities[0].(*UnknownEntity).Layer())
}

func TestWriteSimpleLine(t *testing.T) {
	/*
		testHelpers.go:55: Unable to find '
		10/{<nil> %!s(float64=1)}
		20/{<nil> %!s(float64=2)}
		30/{<nil> %!s(float64=3)}
		11/{<nil> %!s(float64=4)}
		21/{<nil> %!s(float64=5)}
		31/{<nil> %!s(float64=6)}' in '
		0/{<nil> LINE}
		100/{<nil> AcDbEntity}
		8/{<nil> 0}
		100/{<nil> AcDbLine}
		10/{<nil> %!s(float64=1)}
		20/{<nil> %!s(float64=2)}
		30/{<nil> %!s(float64=3)}
		11/{<nil> %!s(float64=4)}
		21/{<nil> %!s(float64=5)}
		31/{<nil> %!s(float64=6)}'
	*/
	line := NewLine()
	line.P1 = Point{1.0, 2.0, 3.0}
	line.P2 = Point{4.0, 5.0, 6.0}
	actual := allCodePairs(line, R12)
	assertContainsCodePairs(t, []CodePair{
		NewStringCodePair(0, "LINE"),
	}, actual)
	assertContainsCodePairs(t, []CodePair{
		NewDoubleCodePair(10, 1.0),
		NewDoubleCodePair(20, 2.0),
		NewDoubleCodePair(30, 3.0),
		NewDoubleCodePair(11, 4.0),
		NewDoubleCodePair(21, 5.0),
		NewDoubleCodePair(31, 6.0),
	}, actual)
}

func TestConditionalEntityFieldWriting(t *testing.T) {
	line := NewLine()
	line.SetIsInPaperSpace(false)
	actual := allCodePairs(line, R14)
	assertNotContainsCodePairs(t, []CodePair{
		NewShortCodePair(67, 0), // this is only written when Version >= R12 and it's not the default (false)
	}, actual)
}

func TestReadEntityFieldFlag(t *testing.T) {
	face := parseEntity(t, "3DFACE",
		NewShortCodePair(70, 5),
	).(*Face)
	assert(t, face.FirstEdgeInvisible(), "expected first edge to be invisible")
	assert(t, !face.SecondEdgeInvisible(), "expected first edge to be visible")
	assert(t, face.ThirdEdgeInvisible(), "expected first edge to be invisible")
	assert(t, !face.FourthEdgeInvisible(), "expected first edge to be visible")
}

func TestWriteEntityFieldFlag(t *testing.T) {
	face := NewFace()
	face.SetFirstEdgeInvisible(true)
	face.SetSecondEdgeInvisible(false)
	face.SetThirdEdgeInvisible(true)
	face.SetFourthEdgeInvisible(false)
	actual := allCodePairs(face, R12)
	assertContainsCodePairs(t, []CodePair{
		NewShortCodePair(70, 0b0101),
	}, actual)
}

func TestWriteVersionSpecificEntities(t *testing.T) {
	solid := NewSolid3D()
	drawing := *NewDrawing()
	drawing.Entities = append(drawing.Entities, solid)

	// ensure it's present when appropriate
	drawing.Header.Version = R13
	assertContainsCodePairs(t, []CodePair{
		NewStringCodePair(0, "3DSOLID"),
	}, drawingCodePairs(t, drawing))

	// and not otherwise
	drawing.Header.Version = R12
	assertNotContainsCodePairs(t, []CodePair{
		NewStringCodePair(0, "3DSOLID"),
	}, drawingCodePairs(t, drawing))
}

func TestReadMultipleBaseEntityData(t *testing.T) {
	line := parseEntity(t, "LINE",
		NewStringCodePair(310, "line 1"),
		NewStringCodePair(310, "line 2"),
	).(*Line)
	assertEqInt(t, 2, len(line.PreviewImageData()))
	assertEqString(t, "line 1", line.PreviewImageData()[0])
	assertEqString(t, "line 2", line.PreviewImageData()[1])
}

func TestWriteMultipleBaseEntityData(t *testing.T) {
	line := NewLine()
	line.SetPreviewImageData(append(line.PreviewImageData(), "line 1"))
	line.SetPreviewImageData(append(line.PreviewImageData(), "line 2"))
	actual := allCodePairs(line, R2000)
	assertContainsCodePairs(t, []CodePair{
		NewStringCodePair(310, "line 1"),
		NewStringCodePair(310, "line 2"),
		NewStringCodePair(100, "AcDbLine"),
	}, actual)
}

func TestReadMultipleSpecificEntityData(t *testing.T) {
	solid := parseEntity(t, "3DSOLID",
		NewStringCodePair(1, "line 1"),
		NewStringCodePair(1, "line 2"),
	).(*Solid3D)
	assertEqInt(t, 2, len(solid.CustomData))
	assertEqString(t, "line 1", solid.CustomData[0])
	assertEqString(t, "line 2", solid.CustomData[1])
}

func TestWriteMultipleSpecificEntityData(t *testing.T) {
	solid := NewSolid3D()
	solid.AddCustomData("line 1")
	solid.AddCustomData("line 2")
	actual := allCodePairs(solid, R13)
	assertContainsCodePairs(t, []CodePair{
		NewStringCodePair(100, "AcDbModelerGeometry"),
		NewShortCodePair(70, 1),
		NewStringCodePair(1, "line 1"),
		NewStringCodePair(1, "line 2"),
	}, actual)
}

func TestWriteConditionsOnWriteOrderDirectives(t *testing.T) {
	solid := NewSolid3D()
	solid.AddCustomData("custom data")

	assertContainsCodePairs(t, []CodePair{
		NewStringCodePair(100, "AcDb3dSolid"),
	}, drawingCodePairsFromEntity(t, solid, R2007))

	assertNotContainsCodePairs(t, []CodePair{
		NewStringCodePair(100, "AcDb3dSolid"),
	}, drawingCodePairsFromEntity(t, solid, R13))
}

func TestReadEntityWithCustomReader(t *testing.T) {
	proxy := parseEntity(t, "ACAD_PROXY_ENTITY",
		NewIntCodePair(92, 4),
		NewStringCodePair(310, "1234"),
		NewStringCodePair(310, "ABCD"),
		NewIntCodePair(93, 4),
		NewStringCodePair(310, "5678"),
		NewStringCodePair(310, "DCBA"),
	).(*ProxyEntity)
	assertEqByteArray(t, []byte{0x12, 0x34, 0xAB, 0xCD}, proxy.GraphicsData)
	assertEqByteArray(t, []byte{0x56, 0x78, 0xDC, 0xBA}, proxy.EntityData)
}

func TestWriteEntityWithBeforeWrite(t *testing.T) {
	proxy := NewProxyEntity()
	proxy.GraphicsData = []byte{0x12, 0x34, 0xAB, 0xCD}
	proxy.EntityData = []byte{0x56, 0x78, 0xDC, 0xBA}
	actual := allCodePairs(proxy, R14)
	assertContainsCodePairs(t, []CodePair{
		NewIntCodePair(92, 4),
		NewStringCodePair(310, "1234ABCD"),
		NewIntCodePair(93, 4),
		NewStringCodePair(310, "5678DCBA"),
	}, actual)
}

func TestReadAttributeDefinitionFollowedByStandaloneMText(t *testing.T) {
	entities := parseEntities(t,
		NewStringCodePair(0, "ATTDEF"),
		NewStringCodePair(2, "TAG"),
		NewStringCodePair(0, "MTEXT"),
		NewStringCodePair(1, "label"),
	)
	assertEqInt(t, 2, len(entities))
	attdef := entities[0].(*AttributeDefinition)
	assertEqString(t, "TAG", attdef.TextTag)
	assertEqString(t, "", attdef.MText.Text)
	assertEqString(t, "label", entities[1].(*MText).Text)
}

func TestReadInsertAttributeFollowedByStandaloneMText(t *testing.T) {
	entities := parseEntities(t,
		NewStringCodePair(0, "INSERT"),
		NewShortCodePair(66, 1),
		NewStringCodePair(0, "ATTRIB"),
		NewStringCodePair(2, "TAG"),
		NewStringCodePair(0, "SEQEND"),
		NewStringCodePair(0, "MTEXT"),
		NewStringCodePair(1, "label"),
	)
	assertEqInt(t, 2, len(entities))
	insert := entities[0].(*Insert)
	assertEqInt(t, 1, len(insert.Attributes))
	assertEqString(t, "TAG", insert.Attributes[0].AttributeTag)
	assertEqString(t, "label", entities[1].(*MText).Text)
}

func TestReadR2018MultilineAttribute(t *testing.T) {
	att := parseEntity(t, "ATTRIB",
		NewStringCodePair(100, "AcDbText"),
		NewDoubleCodePair(10, 1.0),
		NewDoubleCodePair(20, 2.0),
		NewDoubleCodePair(30, 0.0),
		NewDoubleCodePair(40, 2.5),
		NewStringCodePair(1, "first line"),
		NewShortCodePair(71, 4),
		NewShortCodePair(72, 1),
		NewDoubleCodePair(11, 3.0),
		NewDoubleCodePair(21, 4.0),
		NewDoubleCodePair(31, 0.0),
		NewStringCodePair(100, "AcDbAttribute"),
		NewShortCodePair(280, 0),
		NewStringCodePair(2, "ROOM"),
		NewShortCodePair(70, 0),
		NewShortCodePair(280, 1),
		NewShortCodePair(71, 2),
		NewShortCodePair(72, 0),
		NewDoubleCodePair(11, 9.0),
		NewDoubleCodePair(21, 9.0),
		NewDoubleCodePair(31, 9.0),
		NewStringCodePair(101, "Embedded Object"),
		NewDoubleCodePair(10, 5.0),
		NewDoubleCodePair(20, 6.0),
		NewDoubleCodePair(30, 0.0),
		NewDoubleCodePair(40, 2.5),
		NewStringCodePair(1, "first line\\Psecond line"),
	).(*Attribute)
	assertEqString(t, "ROOM", att.AttributeTag)
	assertEqString(t, "first line", att.Value)
	assertEqPoint(t, Point{1.0, 2.0, 0.0}, att.Location)
	assertEqInt(t, 4, att.TextGenerationFlags)
	assertEqInt(t, int(HorizontalTextJustificationCenter), int(att.HorizontalTextJustification))
	assertEqPoint(t, Point{3.0, 4.0, 0.0}, att.SecondAlignmentPoint)
	assertEqInt(t, int(VersionR2010), int(att.Version))
	assertEqBool(t, true, att.IsLockedInBlock)
	assertEqInt(t, 2, int(att.AttributeType))
	assertEqBool(t, true, att.IsMultiline())
	assertEqPoint(t, Point{5.0, 6.0, 0.0}, att.MText.InsertionPoint)
	assertEqString(t, "first line\\Psecond line", att.MText.Text)
}

func TestReadAttributeDefinitionLockPositionWithoutVersion(t *testing.T) {
	// R2007 has the lock position but no version flag
	attdef := parseEntity(t, "ATTDEF",
		NewStringCodePair(100, "AcDbAttributeDefinition"),
		NewStringCodePair(3, "prompt"),
		NewStringCodePair(2, "TAG"),
		NewShortCodePair(70, 0),
		NewShortCodePair(280, 1),
	).(*AttributeDefinition)
	assertEqInt(t, int(VersionR2010), int(attdef.Version))
	assertEqBool(t, true, attdef.IsLockedInBlock)
}

func TestWriteMultilineAttributeAsEmbeddedObject(t *testing.T) {
	attdef := NewAttributeDefinition()
	attdef.TextTag = "ROOM"
	attdef.AttributeType = 4
	attdef.MText.Text = "first\\Psecond"
	actual := allCodePairs(attdef, R2018)
	assertContainsCodePairs(t, []CodePair{
		NewShortCodePair(280, 0),
		NewShortCodePair(71, 4),
		NewShortCodePair(72, 0),
	}, actual)
	assertContainsCodePairs(t, []CodePair{
		NewStringCodePair(101, "Embedded Object"),
	}, actual)
	assertContainsCodePairs(t, []CodePair{
		NewStringCodePair(1, "first\\Psecond"),
	}, actual)
	assertNotContainsCodePairs(t, []CodePair{
		NewStringCodePair(0, "MTEXT"),
	}, actual)

	actual = allCodePairs(attdef, R2013)
	assertNotContainsCodePairs(t, []CodePair{
		NewStringCodePair(101, "Embedded Object"),
	}, actual)
}

func TestRoundTripMultilineInsertAttribute(t *testing.T) {
	att := NewAttribute()
	att.AttributeTag = "ROOM"
	att.Value = "Konyha"
	att.HorizontalTextJustification = HorizontalTextJustificationCenter
	att.AttributeType = 2
	att.MText.Text = "Konyha\\Pétkező"
	insert := NewInsert()
	insert.Name = "B"
	insert.HasAttributes = true
	insert.AddAttributes(*att)
	drawing := *NewDrawing()
	drawing.Header.Version = R2018
	drawing.Entities = append(drawing.Entities, insert)
	roundTripped := roundTripDrawing(t, &drawing)
	assertEqInt(t, 1, len(roundTripped.Entities))
	actual := roundTripped.Entities[0].(*Insert).Attributes[0]
	assertEqString(t, "ROOM", actual.AttributeTag)
	assertEqInt(t, int(HorizontalTextJustificationCenter), int(actual.HorizontalTextJustification))
	assertEqInt(t, 2, int(actual.AttributeType))
	assertEqString(t, "Konyha\\Pétkező", actual.MText.Text)
}

func TestWriteAttributeDefinitionWithoutTrailingMText(t *testing.T) {
	attdef := NewAttributeDefinition()
	actual := drawingCodePairsFromEntity(t, attdef, R14)
	assertContainsCodePairs(t, []CodePair{
		NewStringCodePair(0, "ATTDEF"),
	}, actual)
	assertNotContainsCodePairs(t, []CodePair{
		NewStringCodePair(0, "MTEXT"),
	}, actual)
}

func TestReadMLineDirections(t *testing.T) {
	mline := parseEntity(t, "MLINE",
		NewShortCodePair(72, 2),
		NewDoubleCodePair(11, 1.0),
		NewDoubleCodePair(21, 2.0),
		NewDoubleCodePair(31, 0.0),
		NewDoubleCodePair(12, 1.0),
		NewDoubleCodePair(22, 0.0),
		NewDoubleCodePair(32, 0.0),
		NewDoubleCodePair(13, 0.0),
		NewDoubleCodePair(23, 1.0),
		NewDoubleCodePair(33, 0.0),
		NewDoubleCodePair(11, 3.0),
		NewDoubleCodePair(21, 4.0),
		NewDoubleCodePair(31, 0.0),
		NewDoubleCodePair(12, 0.0),
		NewDoubleCodePair(22, 1.0),
		NewDoubleCodePair(32, 0.0),
		NewDoubleCodePair(13, -1.0),
		NewDoubleCodePair(23, 0.0),
		NewDoubleCodePair(33, 0.0),
	).(*MLine)
	assertEqInt(t, 2, len(mline.Vertices))
	assertEqInt(t, 2, len(mline.SegmentDirections))
	assertEqInt(t, 2, len(mline.MiterDirections))
	assertEqPoint(t, Point{3.0, 4.0, 0.0}, mline.Vertices[1])
	assertEqPoint(t, Point{0.0, 1.0, 0.0}, mline.SegmentDirections[1])
	assertEqPoint(t, Point{-1.0, 0.0, 0.0}, mline.MiterDirections[1])
}

func TestReadLeaderWithWrongVertexCount(t *testing.T) {
	leader := parseEntity(t, "LEADER",
		NewShortCodePair(76, 3),
		NewDoubleCodePair(10, 1.0),
		NewDoubleCodePair(20, 2.0),
		NewDoubleCodePair(10, 3.0),
		NewDoubleCodePair(20, 4.0),
	).(*Leader)
	assertEqInt(t, 2, len(leader.Vertices))
	assertEqPoint(t, Point{3.0, 4.0, 0.0}, leader.Vertices[1])
}

func TestReadDimensionTypeWithFlags(t *testing.T) {
	// ArchiCAD writes 160: a rotated dimension (0) with user-positioned text (128) and its own block (32)
	rotated := parseEntity(t, "DIMENSION",
		NewStringCodePair(100, "AcDbDimension"),
		NewStringCodePair(2, "*D1"),
		NewShortCodePair(70, 160),
		NewStringCodePair(100, "AcDbAlignedDimension"),
		NewDoubleCodePair(12, 5.0),
		NewDoubleCodePair(13, 1.0),
		NewDoubleCodePair(14, 2.0),
		NewDoubleCodePair(50, 90.0),
		NewStringCodePair(100, "AcDbRotatedDimension"),
	).(*RotatedDimension)
	assertEqFloat64(t, 90.0, rotated.RotationAngle)
	assertEqFloat64(t, 5.0, rotated.InsertionPoint.X)
	assertEqString(t, "*D1", rotated.BlockName())
	// the type and the flags are split, and combined again when writing
	assertEqInt(t, int(DimensionTypeRotatedHorizontalOrVertical), int(rotated.DimensionType()))
	assertEqInt(t, 160, int(rotated.DimensionFlags()))
	assertEqBool(t, true, rotated.DimensionFlags().IsTextAtUserDefinedLocation())
	assertEqBool(t, true, rotated.DimensionFlags().IsBlockReferencedByThisDimensionOnly())
	assertEqBool(t, false, rotated.DimensionFlags().IsOrdinateXType())
	assertContainsCodePairs(t, []CodePair{
		NewStringCodePair(2, "*D1"),
		NewDoubleCodePair(10, 0.0),
		NewDoubleCodePair(20, 0.0),
		NewDoubleCodePair(30, 0.0),
		NewDoubleCodePair(11, 0.0),
		NewDoubleCodePair(21, 0.0),
		NewDoubleCodePair(31, 0.0),
		NewShortCodePair(70, 160),
	}, allCodePairs(rotated, R2004))

	radial := parseEntity(t, "DIMENSION",
		NewShortCodePair(70, 4|32),
		NewDoubleCodePair(15, 3.0),
		NewDoubleCodePair(40, 1.5),
	).(*RadialDimension)
	assertEqFloat64(t, 3.0, radial.DefinitionPoint2.X)
	assertEqFloat64(t, 1.5, radial.LeaderLength)
	assertEqInt(t, int(DimensionTypeRadius), int(radial.DimensionType()))
	assertEqInt(t, 32, int(radial.DimensionFlags()))

	ordinate := parseEntity(t, "DIMENSION",
		NewShortCodePair(70, int16(DimensionTypeOrdinate)|64),
	).(*OrdinateDimension)
	assertEqBool(t, true, ordinate.DimensionFlags().IsOrdinateXType())
}

func TestReadArcDimension(t *testing.T) {
	dimension := parseEntity(t, "ARC_DIMENSION",
		NewStringCodePair(100, "AcDbEntity"),
		NewStringCodePair(8, "DIMS"),
		NewStringCodePair(100, "AcDbDimension"),
		NewStringCodePair(2, "*D7"),
		NewDoubleCodePair(10, 1.0),
		NewShortCodePair(70, 5|32),
		NewShortCodePair(71, 5),
		NewDoubleCodePair(41, 1.25),
		NewStringCodePair(100, "AcDbArcDimension"),
		NewDoubleCodePair(13, 2.0),
		NewDoubleCodePair(14, 3.0),
		NewDoubleCodePair(15, 4.0),
		NewDoubleCodePair(25, 5.0),
		NewDoubleCodePair(40, 0.5),
		NewDoubleCodePair(41, 1.5),
		NewShortCodePair(70, 1),
		NewShortCodePair(71, 1),
		NewDoubleCodePair(16, 6.0),
		NewDoubleCodePair(17, 7.0),
	).(*ArcDimension)
	assertEqString(t, "DIMS", dimension.Layer())
	assertEqString(t, "*D7", dimension.BlockName())
	// the dimension data and the arc data use the same codes
	assertEqInt(t, int(DimensionTypeAngularThreePoint), int(dimension.DimensionType()))
	assertEqInt(t, 32, int(dimension.DimensionFlags()))
	assertEqInt(t, 5, int(dimension.AttachmentPoint()))
	assertEqFloat64(t, 1.25, dimension.TextLineSpacingFactor())
	assertEqFloat64(t, 0.5, dimension.StartAngle)
	assertEqFloat64(t, 1.5, dimension.EndAngle)
	assertEqBool(t, true, dimension.IsPartial)
	assertEqBool(t, true, dimension.HasLeader)
	assertEqPoint(t, Point{4.0, 5.0, 0.0}, dimension.ArcCenter)
	assertEqFloat64(t, 2.0, dimension.DefinitionPoint2.X)
	assertEqFloat64(t, 7.0, dimension.LeaderPoint2.X)
}

func TestRoundTripArcDimension(t *testing.T) {
	dimension := NewArcDimension()
	dimension.SetBlockName("*D1")
	dimension.ArcCenter = Point{1, 2, 0}
	dimension.StartAngle, dimension.EndAngle = 0.25, 1.75
	dimension.IsPartial = true
	drawing := *NewDrawing()
	drawing.Header.Version = R2018
	drawing.Entities = append(drawing.Entities, dimension)
	roundTripped := roundTripDrawing(t, &drawing)
	actual := roundTripped.Entities[0].(*ArcDimension)
	assertEqPoint(t, Point{1, 2, 0}, actual.ArcCenter)
	assertEqFloat64(t, 1.75, actual.EndAngle)
	assertEqBool(t, true, actual.IsPartial)
	assertEqInt(t, int(DimensionTypeAngularThreePoint), int(actual.DimensionType()))

	// ARC_DIMENSION exists since AutoCAD 2004
	assertNotContainsCodePairs(t, []CodePair{NewStringCodePair(0, "ARC_DIMENSION")}, drawingCodePairsFromEntity(t, dimension, R2000))
}

func TestReadUnsupportedDimensionTypeKeepsReading(t *testing.T) {
	entities := parseEntities(t,
		NewStringCodePair(0, "DIMENSION"),
		NewStringCodePair(8, "DIMS"),
		NewShortCodePair(70, 2), // 2-line angular dimension
		NewStringCodePair(0, "LINE"),
	)
	assertEqInt(t, 2, len(entities))
	unknown := entities[0].(*UnknownEntity)
	assertEqString(t, "DIMENSION", unknown.Type)
	assertEqString(t, "DIMS", unknown.Layer())
	_ = entities[1].(*Line)
}

func TestReadDimension(t *testing.T) {
	dim := parseEntity(t, "DIMENSION",
		NewStringCodePair(1, "text"),
		NewDoubleCodePair(10, 1.0),
		NewDoubleCodePair(20, 2.0),
		NewShortCodePair(70, 1), // aligned
		NewStringCodePair(100, "AcDbAlignedDimension"),
		NewDoubleCodePair(13, 3.0),
		NewDoubleCodePair(23, 4.0),
		NewDoubleCodePair(14, 5.0),
		NewDoubleCodePair(24, 6.0),
	).(*AlignedDimension)
	assertEqString(t, "text", dim.Text())
	assertEqPoint(t, Point{1.0, 2.0, 0.0}, dim.DefinitionPoint1())
	assertEqPoint(t, Point{3.0, 4.0, 0.0}, dim.DefinitionPoint2)
	assertEqPoint(t, Point{5.0, 6.0, 0.0}, dim.DefinitionPoint3)
}

func TestWriteDimension(t *testing.T) {
	dim := NewAlignedDimension()
	dim.SetDefinitionPoint1(Point{1.0, 2.0, 0.0})
	dim.DefinitionPoint2 = Point{3.0, 4.0, 0.0}
	dim.DefinitionPoint3 = Point{5.0, 6.0, 0.0}
	actual := allCodePairs(dim, R14)
	assertContainsCodePairs(t, []CodePair{
		NewDoubleCodePair(10, 1.0),
		NewDoubleCodePair(20, 2.0),
		NewDoubleCodePair(30, 0.0),
		NewDoubleCodePair(11, 0.0),
		NewDoubleCodePair(21, 0.0),
		NewDoubleCodePair(31, 0.0),
		NewShortCodePair(70, 1),
	}, actual)
	assertContainsCodePairs(t, []CodePair{
		NewStringCodePair(100, "AcDbAlignedDimension"),
		NewDoubleCodePair(13, 3.0),
		NewDoubleCodePair(23, 4.0),
		NewDoubleCodePair(33, 0.0),
		NewDoubleCodePair(14, 5.0),
		NewDoubleCodePair(24, 6.0),
		NewDoubleCodePair(34, 0.0),
	}, actual)
}

func TestWriteRotatedDimensionSkipsDefaultOptionalFields(t *testing.T) {
	// AutoCAD discards the whole drawing when 12 or 52 is written with its default value
	dim := NewRotatedDimension()
	dim.DefinitionPoint2 = Point{1.0, 2.0, 0.0}
	actual := allCodePairs(dim, R2004)
	assertContainsCodePairs(t, []CodePair{
		NewStringCodePair(100, "AcDbAlignedDimension"),
		NewDoubleCodePair(13, 1.0),
	}, actual)
	assertContainsCodePairs(t, []CodePair{
		NewDoubleCodePair(50, 0.0),
		NewStringCodePair(100, "AcDbRotatedDimension"),
	}, actual)

	dim.InsertionPoint = Point{3.0, 4.0, 0.0}
	dim.ExtensionLineAngle = 15.0
	actual = allCodePairs(dim, R2004)
	assertContainsCodePairs(t, []CodePair{
		NewStringCodePair(100, "AcDbAlignedDimension"),
		NewDoubleCodePair(12, 3.0),
		NewDoubleCodePair(22, 4.0),
		NewDoubleCodePair(32, 0.0),
		NewDoubleCodePair(13, 1.0),
	}, actual)
	assertContainsCodePairs(t, []CodePair{
		NewDoubleCodePair(50, 0.0),
		NewDoubleCodePair(52, 15.0),
		NewStringCodePair(100, "AcDbRotatedDimension"),
	}, actual)
}

func TestReadImage(t *testing.T) {
	img := parseEntity(t, "IMAGE",
		NewIntCodePair(91, 2),
		NewDoubleCodePair(14, 1.0),
		NewDoubleCodePair(24, 2.0),
		NewDoubleCodePair(14, 3.0),
		NewDoubleCodePair(24, 4.0),
	).(*Image)
	assertEqInt(t, 2, len(img.ClippingVertices()))
	assertEqPoint(t, Point{1.0, 2.0, 0.0}, img.ClippingVertices()[0])
	assertEqPoint(t, Point{3.0, 4.0, 0.0}, img.ClippingVertices()[1])
}

func TestWriteImage(t *testing.T) {
	img := NewImage()
	img.SetClippingVertices(append(img.ClippingVertices(), Point{1.0, 2.0, 0.0}))
	img.SetClippingVertices(append(img.ClippingVertices(), Point{3.0, 4.0, 0.0}))
	actual := allCodePairs(img, R14)
	assertContainsCodePairs(t, []CodePair{
		NewStringCodePair(100, "AcDbRasterImage"),
	}, actual)
	assertContainsCodePairs(t, []CodePair{
		NewIntCodePair(91, 2),
		NewDoubleCodePair(14, 1.0),
		NewDoubleCodePair(24, 2.0),
		NewDoubleCodePair(14, 3.0),
		NewDoubleCodePair(24, 4.0),
	}, actual)
}

func TestReadInsertAtEnd(t *testing.T) {
	ins := parseEntity(t, "INSERT",
		NewShortCodePair(66, 1), // has attributes
		NewStringCodePair(0, "ATTRIB"),
		NewStringCodePair(1, "attrib 1"),
		NewStringCodePair(0, "ATTRIB"),
		NewStringCodePair(1, "attrib 2"),
		NewStringCodePair(0, "SEQEND"),
	).(*Insert)
	assertEqInt(t, 2, len(ins.Attributes))
	assertEqString(t, "attrib 1", ins.Attributes[0].Value)
	assertEqString(t, "attrib 2", ins.Attributes[1].Value)
}

func TestReadInsertAtEndNoSeqend(t *testing.T) {
	ins := parseEntity(t, "INSERT",
		NewShortCodePair(66, 1), // has attributes
		NewStringCodePair(0, "ATTRIB"),
		NewStringCodePair(1, "attrib 1"),
		NewStringCodePair(0, "ATTRIB"),
		NewStringCodePair(1, "attrib 2"),
	).(*Insert)
	assertEqInt(t, 2, len(ins.Attributes))
	assertEqString(t, "attrib 1", ins.Attributes[0].Value)
	assertEqString(t, "attrib 2", ins.Attributes[1].Value)
}

func TestReadInsertWithTrailingEntity(t *testing.T) {
	entities := parseEntities(t,
		NewStringCodePair(0, "INSERT"),
		NewShortCodePair(66, 1), // has attributes
		NewStringCodePair(0, "ATTRIB"),
		NewStringCodePair(1, "attrib 1"),
		NewStringCodePair(0, "ATTRIB"),
		NewStringCodePair(1, "attrib 2"),
		NewStringCodePair(0, "SEQEND"),
		NewStringCodePair(0, "LINE"), // trailing entity
		NewDoubleCodePair(10, 11.0),
	)
	assertEqInt(t, 2, len(entities))
	ins := entities[0].(*Insert)
	assertEqInt(t, 2, len(ins.Attributes))
	assertEqString(t, "attrib 1", ins.Attributes[0].Value)
	assertEqString(t, "attrib 2", ins.Attributes[1].Value)
	line := entities[1].(*Line)
	assertEqPoint(t, Point{11.0, 0.0, 0.0}, line.P1)
}

func TestReadInsertWithTrailingEntityNoSeqend(t *testing.T) {
	entities := parseEntities(t,
		NewStringCodePair(0, "INSERT"),
		NewShortCodePair(66, 1), // has attributes
		NewStringCodePair(0, "ATTRIB"),
		NewStringCodePair(1, "attrib 1"),
		NewStringCodePair(0, "ATTRIB"),
		NewStringCodePair(1, "attrib 2"),
		NewStringCodePair(0, "LINE"), // trailing entity
		NewDoubleCodePair(10, 11.0),
	)
	assertEqInt(t, 2, len(entities))
	ins := entities[0].(*Insert)
	assertEqInt(t, 2, len(ins.Attributes))
	assertEqString(t, "attrib 1", ins.Attributes[0].Value)
	assertEqString(t, "attrib 2", ins.Attributes[1].Value)
	line := entities[1].(*Line)
	assertEqPoint(t, Point{11.0, 0.0, 0.0}, line.P1)
}

func TestWriteInsert(t *testing.T) {
	ins := NewInsert()
	att1 := *NewAttribute()
	att1.Value = "attrib 1"
	ins.Attributes = append(ins.Attributes, att1)
	att2 := *NewAttribute()
	att2.Value = "attrib 2"
	ins.Attributes = append(ins.Attributes, att2)
	actual := allCodePairs(ins, R14)
	assertContainsCodePairs(t, []CodePair{
		NewStringCodePair(1, "attrib 1"),
	}, actual)
	assertContainsCodePairs(t, []CodePair{
		NewStringCodePair(1, "attrib 2"),
	}, actual)
	assertContainsCodePairs(t, []CodePair{
		NewStringCodePair(0, "SEQEND"),
	}, actual)
}

func TestReadLWPolyline(t *testing.T) {
	lw := parseEntity(t, "LWPOLYLINE",
		NewShortCodePair(70, 1),
		NewIntCodePair(90, 2),      // 2 vertices
		NewDoubleCodePair(10, 1.0), // v1
		NewDoubleCodePair(20, 2.0),
		NewDoubleCodePair(10, 3.0), // v2
		NewDoubleCodePair(20, 4.0),
		NewIntCodePair(91, 42),
	).(*LWPolyline)
	assert(t, lw.IsClosed(), "expected LWPOLYLINE to be closed")
	assertEqInt(t, 2, len(lw.Vertices))
	assertEqFloat64(t, 1.0, lw.Vertices[0].X)
	assertEqFloat64(t, 2.0, lw.Vertices[0].Y)
	assertEqInt(t, 0, lw.Vertices[0].ID)
	assertEqFloat64(t, 3.0, lw.Vertices[1].X)
	assertEqFloat64(t, 4.0, lw.Vertices[1].Y)
	assertEqInt(t, 42, lw.Vertices[1].ID)
}

func TestWriteLWPolyline(t *testing.T) {
	lw := NewLWPolyline()
	lw.Vertices = append(lw.Vertices, LwVertex{X: 1.0, Y: 2.0})
	lw.Vertices = append(lw.Vertices, LwVertex{X: 3.0, Y: 4.0, ID: 42})
	actual := allCodePairs(lw, R2013)
	assertContainsCodePairs(t, []CodePair{
		NewDoubleCodePair(10, 1.0), // v1
		NewDoubleCodePair(20, 2.0),
		NewDoubleCodePair(10, 3.0), // v2
		NewDoubleCodePair(20, 4.0),
		NewIntCodePair(91, 42),
	}, actual)
}

func TestReadModelPoint(t *testing.T) {
	p := parseEntity(t, "POINT",
		NewDoubleCodePair(10, 1.0),
		NewDoubleCodePair(20, 2.0),
		NewDoubleCodePair(30, 3.0),
	).(*ModelPoint)
	assertEqPoint(t, Point{1.0, 2.0, 3.0}, p.Location)
}

func TestWriteModelPoint(t *testing.T) {
	p := NewModelPoint()
	p.Location = Point{1.0, 2.0, 3.0}
	actual := allCodePairs(p, R14)
	assertContainsCodePairs(t, []CodePair{
		NewStringCodePair(100, "AcDbPoint"),
		NewDoubleCodePair(10, 1.0),
		NewDoubleCodePair(20, 2.0),
		NewDoubleCodePair(30, 3.0),
	}, actual)
}

func TestReadPolylineWithNoVertices(t *testing.T) {
	p := parseEntity(t, "POLYLINE",
		NewDoubleCodePair(10, 1.0),
		NewDoubleCodePair(20, 2.0),
		NewDoubleCodePair(30, 3.0),
		NewStringCodePair(0, "SEQEND"),
	).(*Polyline)
	assertEqPoint(t, Point{1.0, 2.0, 3.0}, p.Location)
	assertEqInt(t, 0, len(p.Vertices))
}

func TestReadPolylineWithCLOValues(t *testing.T) {
	p := parseEntity(t, "POLYLINE",
		NewShortCodePair(250, 2),
	).(*Polyline)
	assertEqShort(t, 2, int16(p.CLO_PolylineType))
}

func TestReadPolylineWithMutlipleVertices(t *testing.T) {
	p := parseEntity(t, "POLYLINE",
		NewDoubleCodePair(10, 1.0),
		NewDoubleCodePair(20, 2.0),
		NewDoubleCodePair(30, 3.0),
		NewStringCodePair(0, "VERTEX"),
		NewDoubleCodePair(10, 11.0),
		NewDoubleCodePair(20, 22.0),
		NewDoubleCodePair(30, 33.0),
		NewStringCodePair(0, "VERTEX"),
		NewDoubleCodePair(10, 111.0),
		NewDoubleCodePair(20, 222.0),
		NewDoubleCodePair(30, 333.0),
		NewStringCodePair(0, "SEQEND"),
	).(*Polyline)
	assertEqPoint(t, Point{1.0, 2.0, 3.0}, p.Location)
	assertEqInt(t, 2, len(p.Vertices))
	assertEqPoint(t, Point{11.0, 22.0, 33.0}, p.Vertices[0].Location)
	assertEqPoint(t, Point{111.0, 222.0, 333.0}, p.Vertices[1].Location)
}

func TestReadPolylineWithNoVerticesNoSeqend(t *testing.T) {
	p := parseEntity(t, "POLYLINE",
		NewDoubleCodePair(10, 1.0),
		NewDoubleCodePair(20, 2.0),
		NewDoubleCodePair(30, 3.0),
	).(*Polyline)
	assertEqPoint(t, Point{1.0, 2.0, 3.0}, p.Location)
	assertEqInt(t, 0, len(p.Vertices))
}

func TestReadPolylineWithMultipleVerticesNoSeqend(t *testing.T) {
	p := parseEntity(t, "POLYLINE",
		NewDoubleCodePair(10, 1.0),
		NewDoubleCodePair(20, 2.0),
		NewDoubleCodePair(30, 3.0),
		NewStringCodePair(0, "VERTEX"),
		NewDoubleCodePair(10, 11.0),
		NewDoubleCodePair(20, 22.0),
		NewDoubleCodePair(30, 33.0),
		NewStringCodePair(0, "VERTEX"),
		NewDoubleCodePair(10, 111.0),
		NewDoubleCodePair(20, 222.0),
		NewDoubleCodePair(30, 333.0),
	).(*Polyline)
	assertEqPoint(t, Point{1.0, 2.0, 3.0}, p.Location)
	assertEqInt(t, 2, len(p.Vertices))
	assertEqPoint(t, Point{11.0, 22.0, 33.0}, p.Vertices[0].Location)
	assertEqPoint(t, Point{111.0, 222.0, 333.0}, p.Vertices[1].Location)
}

func TestReadPolylineWithNoVerticesNoSeqendTrailingEntity(t *testing.T) {
	entities := parseEntities(t,
		NewStringCodePair(0, "POLYLINE"),
		NewDoubleCodePair(10, 1.0),
		NewDoubleCodePair(20, 2.0),
		NewDoubleCodePair(30, 3.0),
		NewStringCodePair(0, "LINE"),
		NewDoubleCodePair(10, 11.0),
		NewDoubleCodePair(20, 22.0),
		NewDoubleCodePair(30, 33.0),
	)
	assertEqInt(t, 2, len(entities))

	p := entities[0].(*Polyline)
	assertEqPoint(t, Point{1.0, 2.0, 3.0}, p.Location)
	assertEqInt(t, 0, len(p.Vertices))

	l := entities[1].(*Line)
	assertEqPoint(t, Point{11.0, 22.0, 33.0}, l.P1)
}

func TestReadPolylineWithMultipleVerticesNoSeqendTrailingEntity(t *testing.T) {
	entities := parseEntities(t,
		NewStringCodePair(0, "POLYLINE"),
		NewDoubleCodePair(10, 1.0),
		NewDoubleCodePair(20, 2.0),
		NewDoubleCodePair(30, 3.0),
		NewStringCodePair(0, "VERTEX"),
		NewDoubleCodePair(10, 11.0),
		NewDoubleCodePair(20, 22.0),
		NewDoubleCodePair(30, 33.0),
		NewStringCodePair(0, "VERTEX"),
		NewDoubleCodePair(10, 111.0),
		NewDoubleCodePair(20, 222.0),
		NewDoubleCodePair(30, 333.0),
		NewStringCodePair(0, "LINE"),
		NewDoubleCodePair(10, 11.0),
		NewDoubleCodePair(20, 22.0),
		NewDoubleCodePair(30, 33.0),
	)
	assertEqInt(t, 2, len(entities))

	p := entities[0].(*Polyline)
	assertEqPoint(t, Point{1.0, 2.0, 3.0}, p.Location)
	assertEqInt(t, 2, len(p.Vertices))
	assertEqPoint(t, Point{11.0, 22.0, 33.0}, p.Vertices[0].Location)
	assertEqPoint(t, Point{111.0, 222.0, 333.0}, p.Vertices[1].Location)

	l := entities[1].(*Line)
	assertEqPoint(t, Point{11.0, 22.0, 33.0}, l.P1)
}

func TestWrite2DPolylineTest(t *testing.T) {
	p := NewPolyline()
	p.Vertices = append(p.Vertices, *NewVertex())
	actual := allCodePairs(p, R14)
	assertContainsCodePairs(t, []CodePair{
		NewStringCodePair(100, "AcDb2dPolyline"),
	}, actual)
	assertContainsCodePairs(t, []CodePair{
		NewStringCodePair(100, "AcDbVertex"),
		NewStringCodePair(100, "AcDb2dVertex"),
	}, actual)
	assertContainsCodePairs(t, []CodePair{
		NewStringCodePair(0, "SEQEND"),
	}, actual)
}

func TestWrite3DPolylineTest(t *testing.T) {
	p := NewPolyline()
	v := *NewVertex()
	v.Location.X = 1.0
	v.Location.Y = 2.0
	v.Location.Z = 3.0
	p.Vertices = append(p.Vertices, v)
	p.SetIs3DPolyline(true)
	actual := allCodePairs(p, R14)
	assertContainsCodePairs(t, []CodePair{
		NewStringCodePair(100, "AcDb3dPolyline"),
	}, actual)
	assertContainsCodePairs(t, []CodePair{
		NewStringCodePair(100, "AcDbVertex"),
		NewStringCodePair(100, "AcDb3dPolylineVertex"),
	}, actual)
	assertContainsCodePairs(t, []CodePair{
		NewStringCodePair(0, "SEQEND"),
	}, actual)
}

func TestRoundTripPolylineTest(t *testing.T) {
	p := NewPolyline()
	v := *NewVertex()
	v.Location.X = 1.0
	v.Location.Y = 2.0
	v.Location.Z = 3.0
	p.Vertices = append(p.Vertices, v)
	actual := drawingCodePairsFromEntity(t, p, R14)

	drawing := parseFromCodePairs(t, actual...)
	assertEqInt(t, 1, len(drawing.Entities))
	p2 := drawing.Entities[0].(*Polyline)
	assertEqInt(t, 1, len(p2.Vertices))
	assertEqPoint(t, v.Location, p2.Vertices[0].Location)
}

func TestReadSection(t *testing.T) {
	s := parseEntity(t, "SECTION",
		// 3 vertices
		NewIntCodePair(92, 3),
		NewDoubleCodePair(11, 1.0),
		NewDoubleCodePair(21, 2.0),
		NewDoubleCodePair(31, 3.0),
		NewDoubleCodePair(11, 11.0),
		NewDoubleCodePair(21, 22.0),
		NewDoubleCodePair(31, 33.0),
		NewDoubleCodePair(11, 111.0),
		NewDoubleCodePair(21, 222.0),
		NewDoubleCodePair(31, 333.0),
		// 1 back vertex
		NewIntCodePair(93, 1),
		NewDoubleCodePair(12, 4.0),
		NewDoubleCodePair(22, 5.0),
		NewDoubleCodePair(32, 6.0),
	).(*Section)
	assertEqInt(t, 3, len(s.Vertices))
	assertEqPoint(t, s.Vertices[0], Point{X: 1.0, Y: 2.0, Z: 3.0})
	assertEqPoint(t, s.Vertices[1], Point{X: 11.0, Y: 22.0, Z: 33.0})
	assertEqPoint(t, s.Vertices[2], Point{X: 111.0, Y: 222.0, Z: 333.0})
	assertEqInt(t, 1, len(s.BackLineVertices))
	assertEqPoint(t, s.BackLineVertices[0], Point{X: 4.0, Y: 5.0, Z: 6.0})
}

func TestWriteSection(t *testing.T) {
	s := NewSection()
	s.Vertices = append(s.Vertices, Point{X: 1.0, Y: 2.0, Z: 3.0})
	s.Vertices = append(s.Vertices, Point{X: 11.0, Y: 22.0, Z: 33.0})
	s.Vertices = append(s.Vertices, Point{X: 111.0, Y: 222.0, Z: 333.0})
	s.BackLineVertices = append(s.BackLineVertices, Point{X: 4.0, Y: 5.0, Z: 6.0})
	actual := allCodePairs(s, R2007)
	assertContainsCodePairs(t, []CodePair{
		// 3 vertices
		NewIntCodePair(92, 3),
		NewDoubleCodePair(11, 1.0),
		NewDoubleCodePair(21, 2.0),
		NewDoubleCodePair(31, 3.0),
		NewDoubleCodePair(11, 11.0),
		NewDoubleCodePair(21, 22.0),
		NewDoubleCodePair(31, 33.0),
		NewDoubleCodePair(11, 111.0),
		NewDoubleCodePair(21, 222.0),
		NewDoubleCodePair(31, 333.0),
		// 1 back vertex
		NewIntCodePair(93, 1),
		NewDoubleCodePair(12, 4.0),
		NewDoubleCodePair(22, 5.0),
		NewDoubleCodePair(32, 6.0),
	}, actual)
}

func TestReadSplineWithWeights(t *testing.T) {
	s := parseEntity(t, "SPLINE",
		NewShortCodePair(73, 2),
		NewDoubleCodePair(41, 7.0),
		NewDoubleCodePair(41, 8.0),
		NewDoubleCodePair(10, 1.0),
		NewDoubleCodePair(20, 2.0),
		NewDoubleCodePair(30, 3.0),
		NewDoubleCodePair(10, 4.0),
		NewDoubleCodePair(20, 5.0),
		NewDoubleCodePair(30, 6.0),
	).(*Spline)
	assertEqInt(t, 2, len(s.ControlPoints))
	assertEqFloat64(t, 7.0, s.ControlPoints[0].Weight)
	assertEqPoint(t, Point{X: 1.0, Y: 2.0, Z: 3.0}, s.ControlPoints[0].Point)
	assertEqFloat64(t, 8.0, s.ControlPoints[1].Weight)
	assertEqPoint(t, Point{X: 4.0, Y: 5.0, Z: 6.0}, s.ControlPoints[1].Point)
}

func TestReadSplineWithoutWeights(t *testing.T) {
	s := parseEntity(t, "SPLINE",
		NewShortCodePair(73, 2),
		NewDoubleCodePair(10, 1.0),
		NewDoubleCodePair(20, 2.0),
		NewDoubleCodePair(30, 3.0),
		NewDoubleCodePair(10, 4.0),
		NewDoubleCodePair(20, 5.0),
		NewDoubleCodePair(30, 6.0),
	).(*Spline)
	assertEqInt(t, 2, len(s.ControlPoints))
	assertEqFloat64(t, 1.0, s.ControlPoints[0].Weight)
	assertEqPoint(t, Point{X: 1.0, Y: 2.0, Z: 3.0}, s.ControlPoints[0].Point)
	assertEqFloat64(t, 1.0, s.ControlPoints[1].Weight)
	assertEqPoint(t, Point{X: 4.0, Y: 5.0, Z: 6.0}, s.ControlPoints[1].Point)
}

func TestWriteSplineWithStandardWeights(t *testing.T) {
	s := NewSpline()
	s.ControlPoints = append(s.ControlPoints, ControlPoint{Point: Point{X: 1.0, Y: 2.0, Z: 3.0}, Weight: 1.0})
	s.ControlPoints = append(s.ControlPoints, ControlPoint{Point: Point{X: 4.0, Y: 5.0, Z: 6.0}, Weight: 1.0})
	actual := allCodePairs(s, R13)
	assertContainsCodePairs(t, []CodePair{
		NewShortCodePair(73, 2),
	}, actual)
	assertContainsCodePairs(t, []CodePair{
		NewDoubleCodePair(10, 1.0),
		NewDoubleCodePair(20, 2.0),
		NewDoubleCodePair(30, 3.0),
		NewDoubleCodePair(10, 4.0),
		NewDoubleCodePair(20, 5.0),
		NewDoubleCodePair(30, 6.0),
	}, actual)
	assertNotContainsCodePairs(t, []CodePair{
		NewDoubleCodePair(41, 1.0),
		NewDoubleCodePair(41, 1.0),
	}, actual)
}

func TestWriteSplineWithNonStandardWeights(t *testing.T) {
	s := NewSpline()
	s.ControlPoints = append(s.ControlPoints, ControlPoint{Point: Point{X: 1.0, Y: 2.0, Z: 3.0}, Weight: 7.0})
	s.ControlPoints = append(s.ControlPoints, ControlPoint{Point: Point{X: 4.0, Y: 5.0, Z: 6.0}, Weight: 8.0})
	actual := allCodePairs(s, R13)
	assertContainsCodePairs(t, []CodePair{
		NewShortCodePair(73, 2),
	}, actual)
	assertContainsCodePairs(t, []CodePair{
		NewDoubleCodePair(10, 1.0),
		NewDoubleCodePair(20, 2.0),
		NewDoubleCodePair(30, 3.0),
		NewDoubleCodePair(10, 4.0),
		NewDoubleCodePair(20, 5.0),
		NewDoubleCodePair(30, 6.0),
	}, actual)
	assertContainsCodePairs(t, []CodePair{
		NewDoubleCodePair(41, 7.0),
		NewDoubleCodePair(41, 8.0),
	}, actual)
}

func TestReadUnderlay(t *testing.T) {
	u := parseEntity(t, "DGNUNDERLAY",
		NewDoubleCodePair(10, 1.0), // insertion point
		NewDoubleCodePair(20, 2.0),
		NewDoubleCodePair(30, 3.0),
		NewDoubleCodePair(11, 4.0), // boundary points
		NewDoubleCodePair(21, 5.0),
		NewDoubleCodePair(11, 6.0),
		NewDoubleCodePair(21, 7.0),
	).(*DgnUnderlay)
	assertEqPoint(t, Point{X: 1.0, Y: 2.0, Z: 3.0}, u.InsertionPoint())
	assertEqInt(t, 2, len(u.BoundaryPoints()))
	assertEqPoint(t, Point{X: 4.0, Y: 5.0, Z: 0.0}, u.BoundaryPoints()[0])
	assertEqPoint(t, Point{X: 6.0, Y: 7.0, Z: 0.0}, u.BoundaryPoints()[1])
}

func TestWriteUnderlay(t *testing.T) {
	u := NewDgnUnderlay()
	u.SetInsertionPoint(Point{X: 1.0, Y: 2.0, Z: 3.0})
	u.SetBoundaryPoints(append(u.BoundaryPoints(), Point{X: 4.0, Y: 5.0, Z: 0.0}))
	u.SetBoundaryPoints(append(u.BoundaryPoints(), Point{X: 6.0, Y: 7.0, Z: 0.0}))
	actual := allCodePairs(u, R14)
	assertContainsCodePairs(t, []CodePair{
		NewStringCodePair(0, "DGNUNDERLAY"),
	}, actual)
	assertContainsCodePairs(t, []CodePair{
		NewDoubleCodePair(10, 1.0),
		NewDoubleCodePair(20, 2.0),
		NewDoubleCodePair(30, 3.0),
	}, actual)
	assertContainsCodePairs(t, []CodePair{
		NewDoubleCodePair(11, 4.0),
		NewDoubleCodePair(21, 5.0),
		NewDoubleCodePair(11, 6.0),
		NewDoubleCodePair(21, 7.0),
	}, actual)
}

func TestReadWipeout(t *testing.T) {
	wo := parseEntity(t, "WIPEOUT",
		NewIntCodePair(91, 2),
		NewDoubleCodePair(14, 1.0),
		NewDoubleCodePair(24, 2.0),
		NewDoubleCodePair(14, 3.0),
		NewDoubleCodePair(24, 4.0),
	).(*Wipeout)
	assertEqInt(t, 2, len(wo.ClippingVertices()))
	assertEqPoint(t, Point{1.0, 2.0, 0.0}, wo.ClippingVertices()[0])
	assertEqPoint(t, Point{3.0, 4.0, 0.0}, wo.ClippingVertices()[1])
}

func TestWriteWipeout(t *testing.T) {
	wo := NewWipeout()
	wo.SetClippingVertices(append(wo.ClippingVertices(), Point{1.0, 2.0, 0.0}))
	wo.SetClippingVertices(append(wo.ClippingVertices(), Point{3.0, 4.0, 0.0}))
	actual := allCodePairs(wo, R2000)
	assertContainsCodePairs(t, []CodePair{
		NewStringCodePair(100, "AcDbWipeout"),
	}, actual)
	assertContainsCodePairs(t, []CodePair{
		NewIntCodePair(91, 2),
		NewDoubleCodePair(14, 1.0),
		NewDoubleCodePair(24, 2.0),
		NewDoubleCodePair(14, 3.0),
		NewDoubleCodePair(24, 4.0),
	}, actual)
}

func TestReadXLine(t *testing.T) {
	x := parseEntity(t, "XLINE",
		NewDoubleCodePair(10, 1.0),
		NewDoubleCodePair(20, 2.0),
		NewDoubleCodePair(30, 3.0),
		NewDoubleCodePair(11, 4.0),
		NewDoubleCodePair(21, 5.0),
		NewDoubleCodePair(31, 6.0),
	).(*XLine)
	assertEqPoint(t, Point{1.0, 2.0, 3.0}, x.FirstPoint)
	assertEqVector(t, Vector{4.0, 5.0, 6.0}, x.UnitDirectionVector)
}

func TestWriteXLine(t *testing.T) {
	x := NewXLine()
	x.FirstPoint = Point{1.0, 2.0, 3.0}
	x.UnitDirectionVector = Vector{4.0, 5.0, 6.0}
	actual := allCodePairs(x, R13)
	assertContainsCodePairs(t, []CodePair{
		NewDoubleCodePair(10, 1.0),
		NewDoubleCodePair(20, 2.0),
		NewDoubleCodePair(30, 3.0),
		NewDoubleCodePair(11, 4.0),
		NewDoubleCodePair(21, 5.0),
		NewDoubleCodePair(31, 6.0),
	}, actual)
}

func TestReadSkipsApplicationGroups(t *testing.T) {
	line := parseEntity(t, "LINE",
		NewStringCodePair(5, "A1"),
		NewStringCodePair(330, "1F"),
		// a reactor after the owner used to replace it
		NewStringCodePair(102, "{ACAD_REACTORS"),
		NewStringCodePair(330, "2E"),
		NewStringCodePair(102, "}"),
		NewStringCodePair(102, "{ACAD_XDICTIONARY"),
		NewStringCodePair(360, "3D"),
		NewStringCodePair(102, "}"),
		NewDoubleCodePair(10, 1.0),
	).(*Line)
	assertEqInt(t, 0x1F, int(line.getOwnerPointer().handle))
	assertEqPoint(t, Point{1.0, 0.0, 0.0}, line.P1)

	viewport := parseEntity(t, "VIEWPORT",
		NewStringCodePair(102, "{BLKREFS"),
		NewStringCodePair(331, "4C"),
		NewStringCodePair(102, "}"),
		NewStringCodePair(331, "5B"),
	).(*Viewport)
	assertEqInt(t, 1, len(viewport.FrozenLayerHandles))
	assertEqInt(t, 0x5B, int(viewport.FrozenLayerHandles[0]))
}

func TestReadUnclosedApplicationGroupEndsWithTheEntity(t *testing.T) {
	entities := parseEntities(t,
		NewStringCodePair(0, "LINE"),
		NewStringCodePair(102, "{ACAD_REACTORS"),
		NewStringCodePair(330, "2E"),
		NewStringCodePair(0, "CIRCLE"),
		NewDoubleCodePair(40, 2.0),
	)
	assertEqInt(t, 2, len(entities))
	assertEqFloat64(t, 2.0, entities[1].(*Circle).Radius)
}

func parseEntity(t *testing.T, entityType string, body ...CodePair) Entity {
	codePairs := []CodePair{NewStringCodePair(0, entityType)}
	codePairs = append(codePairs, body...)
	entities := parseEntities(t, codePairs...)
	assertEqInt(t, 1, len(entities))
	return entities[0]
}

func parseEntities(t *testing.T, body ...CodePair) []Entity {
	codePairs := []CodePair{
		NewStringCodePair(0, "SECTION"),
		NewStringCodePair(2, "ENTITIES"),
	}
	codePairs = append(codePairs, body...)
	codePairs = append(codePairs,
		NewStringCodePair(0, "ENDSEC"),
		NewStringCodePair(0, "EOF"),
	)
	drawing := parseFromCodePairs(t, codePairs...)
	return drawing.Entities
}

func codePairsWithCode(code int, pairs []CodePair) (found []CodePair) {
	for _, pair := range pairs {
		if pair.Code == code {
			found = append(found, pair)
		}
	}
	return
}

func TestReadAndWriteTrueColorBlack(t *testing.T) {
	// ArchiCAD writes 420 = 0 for black; it must not be mistaken for "no true color"
	for _, entityType := range []string{"LINE", "HATCH"} {
		black := parseEntity(t, entityType, NewIntCodePair(420, 0))
		assertEqBool(t, true, black.HasColor24Bit())
		assertEqInt(t, 0, black.Color24Bit())
		assertEqCodePairs(t, []CodePair{NewIntCodePair(420, 0)}, codePairsWithCode(420, drawingCodePairsFromEntity(t, black, R2004)))

		unset := parseEntity(t, entityType)
		assertEqBool(t, false, unset.HasColor24Bit())
		assertEqInt(t, 0, len(codePairsWithCode(420, drawingCodePairsFromEntity(t, unset, R2004))))
	}
}

func TestSetAndClearTrueColor(t *testing.T) {
	line := NewLine()
	assertEqBool(t, false, line.HasColor24Bit())
	line.SetColor24Bit(0)
	assertEqBool(t, true, line.HasColor24Bit())
	assertEqCodePairs(t, []CodePair{NewIntCodePair(420, 0)}, codePairsWithCode(420, drawingCodePairsFromEntity(t, line, R2004)))

	line.ClearColor24Bit()
	assertEqBool(t, false, line.HasColor24Bit())
	assertEqInt(t, 0, len(codePairsWithCode(420, drawingCodePairsFromEntity(t, line, R2004))))
}

func TestReadMTextTrueColorAndBackgroundColor(t *testing.T) {
	// the entity's 420 comes before the MTEXT data, the background fill's 420 after code 90
	mtext := parseEntity(t, "MTEXT",
		NewStringCodePair(100, "AcDbEntity"),
		NewIntCodePair(420, 0xA80E02),
		NewStringCodePair(100, "AcDbMText"),
		NewStringCodePair(1, "text"),
		NewIntCodePair(90, 1),
		NewShortCodePair(63, 7),
		NewIntCodePair(420, 0x112233),
	).(*MText)
	assertEqBool(t, true, mtext.HasColor24Bit())
	assertEqInt(t, 0xA80E02, mtext.Color24Bit())
	assertEqInt(t, 0x112233, mtext.BackgroundColorRGB)

	withoutBackground := parseEntity(t, "MTEXT", NewIntCodePair(420, 0xA80E02), NewStringCodePair(1, "text")).(*MText)
	assertEqInt(t, 0xA80E02, withoutBackground.Color24Bit())
	assertEqInt(t, 0, withoutBackground.BackgroundColorRGB)
}

package dxf

import (
	"math"
	"testing"
)

func mleaderPairs(content ...CodePair) []CodePair {
	pairs := []CodePair{
		NewStringCodePair(100, "AcDbEntity"),
		NewStringCodePair(8, "NOTES"),
		NewStringCodePair(100, "AcDbMLeader"),
		NewShortCodePair(270, 2),
		NewStringCodePair(300, "CONTEXT_DATA{"),
		NewDoubleCodePair(40, 2.0),
		NewDoubleCodePair(10, 5.0), NewDoubleCodePair(20, 5.0), NewDoubleCodePair(30, 0.0),
		NewDoubleCodePair(41, 2.5),
		NewDoubleCodePair(140, 0.18),
	}
	pairs = append(pairs, content...)
	pairs = append(pairs,
		NewStringCodePair(302, "LEADER{"),
		NewBoolCodePair(290, true),
		NewBoolCodePair(291, true),
		NewDoubleCodePair(10, 4.0), NewDoubleCodePair(20, 4.0), NewDoubleCodePair(30, 0.0),
		NewDoubleCodePair(11, 1.0), NewDoubleCodePair(21, 0.0), NewDoubleCodePair(31, 0.0),
		NewDoubleCodePair(40, 0.36),
		NewStringCodePair(304, "LEADER_LINE{"),
		NewDoubleCodePair(10, 0.0), NewDoubleCodePair(20, 0.0), NewDoubleCodePair(30, 0.0),
		NewDoubleCodePair(10, 2.0), NewDoubleCodePair(20, 3.0), NewDoubleCodePair(30, 0.0),
		NewIntCodePair(91, 0),
		NewIntCodePair(92, -1056964608),
		NewStringCodePair(305, "}"),
		NewIntCodePair(90, 0),
		NewStringCodePair(303, "}"),
		NewStringCodePair(301, "}"),
		// leader properties after the context data; 341 is the line type here
		NewStringCodePair(340, "1A"),
		NewIntCodePair(90, 0),
		NewStringCodePair(341, "1B"),
		NewDoubleCodePair(41, 99.0),
	)
	return pairs
}

func TestReadMLeaderWithText(t *testing.T) {
	leader := parseEntity(t, "MULTILEADER", mleaderPairs(
		NewBoolCodePair(290, true),
		NewStringCodePair(304, "Előtér\\Pszoba"),
		NewDoubleCodePair(12, 4.5), NewDoubleCodePair(22, 4.25), NewDoubleCodePair(32, 0.0),
		NewDoubleCodePair(13, 1.0), NewDoubleCodePair(23, 0.0), NewDoubleCodePair(33, 0.0),
		NewDoubleCodePair(42, 0.5),
		NewDoubleCodePair(43, 10.0),
		NewBoolCodePair(296, false),
	)...).(*MLeader)
	assertEqString(t, "NOTES", leader.Layer())
	assertEqFloat64(t, 2.0, leader.Scale)
	assertEqFloat64(t, 2.5, leader.TextHeight)
	assertEqFloat64(t, 0.18, leader.ArrowSize)
	assertEqBool(t, true, leader.HasText)
	assertEqBool(t, false, leader.HasBlock)
	assertEqString(t, "Előtér\\Pszoba", leader.Text)
	assertEqPoint(t, Point{4.5, 4.25, 0}, leader.TextLocation)
	assertEqVector(t, Vector{1, 0, 0}, leader.TextDirection)
	assertEqFloat64(t, 0.5, leader.TextRotation)
	assertEqFloat64(t, 10.0, leader.TextWidth)

	assertEqInt(t, 1, len(leader.Leaders))
	assertEqPoint(t, Point{4, 4, 0}, leader.Leaders[0].LastLeaderPoint)
	assertEqVector(t, Vector{1, 0, 0}, leader.Leaders[0].DoglegVector)
	assertEqFloat64(t, 0.36, leader.Leaders[0].DoglegLength)
	assertEqInt(t, 1, len(leader.Leaders[0].Lines))
	assertEqInt(t, 2, len(leader.Leaders[0].Lines[0]))
	assertEqPoint(t, Point{2, 3, 0}, leader.Leaders[0].Lines[0][1])

	// MText helpers work on the text content
	mtext := NewMText()
	mtext.Text = leader.Text
	assertEqString(t, "Előtér\nszoba", mtext.PlainText())
}

func TestWriteMLeaderAsRead(t *testing.T) {
	pairs := mleaderPairs(NewBoolCodePair(290, true), NewStringCodePair(304, "text"))
	leader := parseEntity(t, "MULTILEADER", pairs...).(*MLeader)
	written := allCodePairs(leader, R2018)
	assertContainsCodePairs(t, pairs[2:], written)

	assertNotContainsCodePairs(t, []CodePair{NewStringCodePair(0, "MULTILEADER")}, drawingCodePairsFromEntity(t, leader, R2004))
	assertNotContainsCodePairs(t, []CodePair{NewStringCodePair(0, "MULTILEADER")}, drawingCodePairsFromEntity(t, newMLeader(), R2018))
}

func TestWalkMLeaderBlockContent(t *testing.T) {
	leader := parseEntity(t, "MULTILEADER", mleaderPairs(
		NewBoolCodePair(290, false),
		NewBoolCodePair(296, true),
		NewStringCodePair(341, "2F"),
		NewDoubleCodePair(14, 0.0), NewDoubleCodePair(24, 0.0), NewDoubleCodePair(34, 1.0),
		NewDoubleCodePair(15, 10.0), NewDoubleCodePair(25, 20.0), NewDoubleCodePair(35, 0.0),
		NewDoubleCodePair(16, 2.0), NewDoubleCodePair(26, 2.0), NewDoubleCodePair(36, 2.0),
		NewDoubleCodePair(46, math.Pi/2),
	)...).(*MLeader)
	assertEqBool(t, true, leader.HasBlock)
	assertEqUInt64(t, 0x2F, uint64(leader.BlockRecordHandle))

	drawing := *NewDrawing()
	record := NewBlockRecord()
	record.Name = "TAG"
	record.SetHandle(0x2F)
	drawing.BlockRecords = append(drawing.BlockRecords, *record)
	drawing.Blocks = append(drawing.Blocks, Block{Name: "TAG", Entities: []Entity{lineEntity(Point{0, 0, 0}, Point{1, 0, 0})}})
	drawing.Entities = append(drawing.Entities, leader)
	assert(t, leader.ContentBlock(&drawing) != nil, "expected the content block")

	visits, issues := walkAll(t, &drawing, WalkOptions{})
	assertEqInt(t, 0, len(issues))
	lines := visitsOf[*Line](visits)
	assertEqInt(t, 1, len(lines))
	// scaled by 2 and rotated by 90° at (10, 20)
	assertNearPoint(t, Point{10, 22, 0}, lines[0].matrix.TransformPoint(Point{1, 0, 0}))

	// exploding keeps the leader itself and adds the block content
	result := drawing.Explode(ExplodeOptions{})
	assertEqInt(t, 2, len(result.Entities))
	_ = result.Entities[0].(*MLeader)
	_ = result.Entities[1].(*Line)
}

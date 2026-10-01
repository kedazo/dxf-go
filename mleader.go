package dxf

import (
	"math"
	"strings"
)

// MLeader is a MULTILEADER. Its leaders, text and block content are parsed from the context data (coordinates are
// world coordinates); the entity is written back exactly as it was read, so changing the fields has no effect on the
// written leader.
type MLeader struct {
	entityCommon

	// Scale is the overall scale of the leader (arrow sizes, text height and gaps are scaled by it).
	Scale     float64
	ArrowSize float64
	Leaders   []MLeaderLeader

	HasText       bool
	Text          string // MTEXT content with formatting codes
	TextLocation  Point
	TextDirection Vector
	TextHeight    float64
	TextRotation  float64 // radians
	TextWidth     float64

	HasBlock          bool
	BlockRecordHandle Handle
	BlockLocation     Point
	BlockNormal       Vector
	BlockScale        Vector
	BlockRotation     float64 // radians

	// StyleHandle is the MLEADERSTYLE (code 340 after the context data); see Drawing.MLeaderStyle.
	StyleHandle Handle

	// the subclass data as read
	leaderData []CodePair
}

// MLeaderLeader is one leader of a MULTILEADER: its lines end at the landing (LastLeaderPoint), from where the
// dogleg runs towards the content.
type MLeaderLeader struct {
	LastLeaderPoint Point
	DoglegVector    Vector
	DoglegLength    float64
	// Lines holds the vertices of every leader line, starting at the arrow.
	Lines [][]Point
}

func newMLeader() *MLeader {
	return &MLeader{
		entityCommon: newEntityCommon(),
		Scale:        1.0,
		BlockNormal:  *NewZAxis(),
		BlockScale:   Vector{1, 1, 1},
	}
}

func (e *MLeader) typeString() string { return "MULTILEADER" }

func (e *MLeader) minVersion() AcadVersion { return R2007 }

func (e *MLeader) maxVersion() AcadVersion {
	if len(e.leaderData) == 0 {
		// a leader that wasn't read can't be written
		return Version1_0
	}
	return R2018
}

func (e *MLeader) tryApplyCodePair(codePair CodePair) {
	if len(e.leaderData) > 0 {
		e.leaderData = append(e.leaderData, codePair)
		return
	}
	if codePair.Code == 100 && stringValue(codePair) == "AcDbMLeader" {
		e.leaderData = append(e.leaderData, codePair)
		return
	}
	tryApplyCodePairForEntity(e, codePair)
}

// parseLeaderData reads the context data: CONTEXT_DATA{ (300) … } (301) holds the content and the leaders, LEADER{
// (302) … } (303), which hold their lines, LEADER_LINE{ (304) … } (305).
func (e *MLeader) parseLeaderData() {
	inContext, inLeader, inLine := false, false, false
	var leader *MLeaderLeader
	for _, pair := range e.leaderData {
		code := pair.Code
		switch {
		case code == 300 && strings.HasPrefix(stringValue(pair), "CONTEXT_DATA"):
			inContext = true
			continue
		case code == 301 && inContext && !inLeader:
			inContext = false
			continue
		case !inContext:
			if code == 340 {
				e.StyleHandle = handleFromString(stringValue(pair))
			}
			continue
		case code == 302 && strings.HasPrefix(stringValue(pair), "LEADER"):
			inLeader = true
			e.Leaders = append(e.Leaders, MLeaderLeader{})
			leader = &e.Leaders[len(e.Leaders)-1]
			continue
		case code == 303 && inLeader && !inLine:
			inLeader = false
			continue
		case code == 304 && inLeader && strings.HasPrefix(stringValue(pair), "LEADER_LINE"):
			inLine = true
			leader.Lines = append(leader.Lines, nil)
			continue
		case code == 305 && inLine:
			inLine = false
			continue
		}

		switch {
		case inLine:
			line := &leader.Lines[len(leader.Lines)-1]
			switch code {
			case 10:
				*line = append(*line, Point{X: doubleValue(pair)})
			case 20, 30:
				if len(*line) > 0 {
					applyPointCodePair(&(*line)[len(*line)-1], 10, pair)
				}
			}
		case inLeader:
			switch code {
			case 10, 20, 30:
				applyPointCodePair(&leader.LastLeaderPoint, 10, pair)
			case 11:
				leader.DoglegVector.X = doubleValue(pair)
			case 21:
				leader.DoglegVector.Y = doubleValue(pair)
			case 31:
				leader.DoglegVector.Z = doubleValue(pair)
			case 40:
				leader.DoglegLength = doubleValue(pair)
			}
		default:
			e.applyContextCodePair(pair)
		}
	}
}

func (e *MLeader) applyContextCodePair(pair CodePair) {
	switch pair.Code {
	case 40:
		e.Scale = doubleValue(pair)
	case 41:
		e.TextHeight = doubleValue(pair)
	case 140:
		e.ArrowSize = doubleValue(pair)
	case 290:
		e.HasText = shortValue(pair) != 0 || boolValue(pair)
	case 296:
		e.HasBlock = shortValue(pair) != 0 || boolValue(pair)
	case 304:
		e.Text = stringValue(pair)
	case 12, 22, 32:
		applyPointCodePair(&e.TextLocation, 12, pair)
	case 13:
		e.TextDirection.X = doubleValue(pair)
	case 23:
		e.TextDirection.Y = doubleValue(pair)
	case 33:
		e.TextDirection.Z = doubleValue(pair)
	case 42:
		e.TextRotation = doubleValue(pair)
	case 43:
		e.TextWidth = doubleValue(pair)
	case 341:
		e.BlockRecordHandle = handleFromString(stringValue(pair))
	case 14:
		e.BlockNormal.X = doubleValue(pair)
	case 24:
		e.BlockNormal.Y = doubleValue(pair)
	case 34:
		e.BlockNormal.Z = doubleValue(pair)
	case 15, 25, 35:
		applyPointCodePair(&e.BlockLocation, 15, pair)
	case 16:
		e.BlockScale.X = doubleValue(pair)
	case 26:
		e.BlockScale.Y = doubleValue(pair)
	case 36:
		e.BlockScale.Z = doubleValue(pair)
	case 46:
		e.BlockRotation = doubleValue(pair)
	}
}

func boolValue(pair CodePair) bool {
	value, ok := pair.Value.(BoolCodePairValue)
	return ok && value.Value
}

func (e *MLeader) codePairs(version AcadVersion) (pairs []CodePair) {
	pairs = append(pairs, NewStringCodePair(0, "MULTILEADER"))
	pairs = append(pairs, codePairsForEntity(e, version)...)
	pairs = append(pairs, e.leaderData...)
	return
}

// ContentBlock returns the block a MULTILEADER with block content shows, or nil.
func (e *MLeader) ContentBlock(d *Drawing) *Block {
	if !e.HasBlock || e.BlockRecordHandle == 0 {
		return nil
	}
	for i := range d.BlockRecords {
		if d.BlockRecords[i].Handle() == e.BlockRecordHandle {
			return d.BlockByName(d.BlockRecords[i].Name)
		}
	}
	return nil
}

// contentInsert returns the INSERT that shows the block content, or nil.
func (e *MLeader) contentInsert(d *Drawing) *Insert {
	block := e.ContentBlock(d)
	if block == nil {
		return nil
	}
	insert := NewInsert()
	insert.Name = block.Name
	insert.ExtrusionDirection = e.BlockNormal
	insert.Location = WCSToOCSMatrix(e.BlockNormal).TransformPoint(e.BlockLocation)
	insert.XScaleFactor, insert.YScaleFactor, insert.ZScaleFactor = e.BlockScale.X, e.BlockScale.Y, e.BlockScale.Z
	insert.Rotation = e.BlockRotation * 180 / math.Pi
	insert.SetLayer(e.Layer())
	insert.SetColor(e.Color())
	insert.SetLineTypeName(e.LineTypeName())
	insert.SetLineWeight(e.LineWeight())
	insert.SetIsInPaperSpace(e.IsInPaperSpace())
	return insert
}

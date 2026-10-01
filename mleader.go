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

	// The leader's own properties, after the context data. One that has an MLeaderOverride flag only applies when
	// IsOverridden reports it; otherwise the MLEADERSTYLE's value does.
	PropertyOverrides       MLeaderOverride // code 90
	LeaderLineType          int16           // code 170: 0 = invisible, 1 = straight, 2 = spline
	LeaderLineColor         ObjectColor     // code 91
	LeaderLineTypeHandle    Handle          // code 341, the LTYPE
	LeaderLineWeight        LineWeight      // code 171
	IsLandingEnabled        bool            // code 290
	IsDoglegEnabled         bool            // code 291
	DoglegLength            float64         // code 41
	ArrowheadHandle         Handle          // code 342, the arrowhead's BLOCK_RECORD; 0 = closed filled
	ArrowheadSize           float64         // code 42
	ContentType             int16           // code 172: 0 = none, 1 = block, 2 = MTEXT, 3 = tolerance
	TextStyleHandle         Handle          // code 343, the STYLE
	TextLeftAttachment      int16           // code 173
	TextRightAttachment     int16           // code 95
	TextAngleType           int16           // code 174
	TextAlignment           int16           // code 175
	TextColor               ObjectColor     // code 92
	IsTextFrameEnabled      bool            // code 292
	BlockContentHandle      Handle          // code 344, the BLOCK_RECORD
	BlockContentColor       ObjectColor     // code 93
	BlockContentScale       Vector          // codes 10/20/30
	BlockContentRotation    float64         // code 43, radians
	BlockContentConnection  int16           // code 176: 0 = extents, 1 = base point
	IsAnnotative            bool            // code 293
	IsTextDirectionNegative bool            // code 294
	TextAlignInIPE          int16           // code 178
	TextAttachmentPoint     int16           // code 179
	StyleScale              float64         // code 45, overrides the style's Scale (MLeaderOverrideScale)
	TextAttachmentDirection int16           // code 271: 0 = horizontal, 1 = vertical
	BottomTextAttachment    int16           // code 272
	TopTextAttachment       int16           // code 273
	IsLeaderExtendedToText  bool            // code 295
	// Arrowheads are the arrowheads of single leader lines (codes 94/345).
	Arrowheads []MLeaderArrowhead
	// BlockAttributes are the attribute values of the block content (codes 330/177/44/302).
	BlockAttributes []MLeaderBlockAttribute

	// the subclass data as read
	leaderData []CodePair
}

// MLeaderOverride is a flag of MLeader.PropertyOverrides: the leader's own value of the property is used instead of
// the MLEADERSTYLE's.
type MLeaderOverride uint32

const (
	MLeaderOverrideLeaderLineType          MLeaderOverride = 1 << 0
	MLeaderOverrideLeaderLineColor         MLeaderOverride = 1 << 1
	MLeaderOverrideLeaderLineTypeHandle    MLeaderOverride = 1 << 2
	MLeaderOverrideLeaderLineWeight        MLeaderOverride = 1 << 3
	MLeaderOverrideLanding                 MLeaderOverride = 1 << 4
	MLeaderOverrideLandingGap              MLeaderOverride = 1 << 5
	MLeaderOverrideDogleg                  MLeaderOverride = 1 << 6
	MLeaderOverrideDoglegLength            MLeaderOverride = 1 << 7
	MLeaderOverrideArrowhead               MLeaderOverride = 1 << 8
	MLeaderOverrideArrowheadSize           MLeaderOverride = 1 << 9
	MLeaderOverrideContentType             MLeaderOverride = 1 << 10
	MLeaderOverrideTextStyle               MLeaderOverride = 1 << 11
	MLeaderOverrideTextLeftAttachment      MLeaderOverride = 1 << 12
	MLeaderOverrideTextAngleType           MLeaderOverride = 1 << 13
	MLeaderOverrideTextAlignment           MLeaderOverride = 1 << 14
	MLeaderOverrideTextColor               MLeaderOverride = 1 << 15
	MLeaderOverrideTextHeight              MLeaderOverride = 1 << 16 // the context's TextHeight
	MLeaderOverrideTextFrame               MLeaderOverride = 1 << 17
	MLeaderOverrideDefaultMText            MLeaderOverride = 1 << 18
	MLeaderOverrideBlockContent            MLeaderOverride = 1 << 19
	MLeaderOverrideBlockContentColor       MLeaderOverride = 1 << 20
	MLeaderOverrideBlockContentScale       MLeaderOverride = 1 << 21
	MLeaderOverrideBlockContentRotation    MLeaderOverride = 1 << 22
	MLeaderOverrideBlockContentConnection  MLeaderOverride = 1 << 23
	MLeaderOverrideScale                   MLeaderOverride = 1 << 24
	MLeaderOverrideTextRightAttachment     MLeaderOverride = 1 << 25
	MLeaderOverrideTextSwitchAlignment     MLeaderOverride = 1 << 26
	MLeaderOverrideTextAttachmentDirection MLeaderOverride = 1 << 27
	MLeaderOverrideTopTextAttachment       MLeaderOverride = 1 << 28
	MLeaderOverrideBottomTextAttachment    MLeaderOverride = 1 << 29
)

// IsOverridden reports whether the leader's own value of a property is used instead of its MLEADERSTYLE's.
func (e *MLeader) IsOverridden(property MLeaderOverride) bool {
	return e.PropertyOverrides&property != 0
}

// MLeaderArrowhead is the arrowhead of one leader line.
type MLeaderArrowhead struct {
	Index  int    // code 94
	Handle Handle // code 345, the arrowhead's BLOCK_RECORD
}

// MLeaderBlockAttribute is the value of one attribute of a MULTILEADER's block content.
type MLeaderBlockAttribute struct {
	AttributeDefinitionHandle Handle  // code 330, the ATTDEF in the content block
	Index                     int16   // code 177
	Width                     float64 // code 44
	Text                      string  // code 302
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

		// what AutoCAD assumes when a code is missing (as ezdxf)
		LeaderLineType:       1,
		LeaderLineColor:      ObjectColorByBlock,
		LeaderLineWeight:     LineWeightByBlock,
		IsLandingEnabled:     true,
		IsDoglegEnabled:      true,
		DoglegLength:         8,
		ArrowheadSize:        4,
		ContentType:          2,
		TextLeftAttachment:   1,
		TextRightAttachment:  1,
		TextAngleType:        1,
		TextAlignment:        2,
		TextColor:            ObjectColorByBlock,
		BlockContentColor:    ObjectColorByBlock,
		BlockContentScale:    Vector{1, 1, 1},
		TextAttachmentPoint:  1,
		StyleScale:           1,
		BottomTextAttachment: 9,
		TopTextAttachment:    9,
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
	inContext, inLeader, inLine, afterContext := false, false, false, false
	var leader *MLeaderLeader
	for _, pair := range e.leaderData {
		code := pair.Code
		switch {
		case code == 300 && strings.HasPrefix(stringValue(pair), "CONTEXT_DATA"):
			inContext = true
			continue
		case code == 301 && inContext && !inLeader:
			inContext, afterContext = false, true
			continue
		case !inContext:
			if afterContext {
				e.applyPropertyCodePair(pair)
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

// applyPropertyCodePair reads the leader's own properties, which follow the context data.
func (e *MLeader) applyPropertyCodePair(pair CodePair) {
	switch pair.Code {
	case 340:
		e.StyleHandle = handleFromString(stringValue(pair))
	case 90:
		e.PropertyOverrides = MLeaderOverride(uint32(intValue(pair)))
	case 170:
		e.LeaderLineType = shortValue(pair)
	case 91:
		e.LeaderLineColor = ObjectColor(intValue(pair))
	case 341:
		e.LeaderLineTypeHandle = handleFromString(stringValue(pair))
	case 171:
		e.LeaderLineWeight = LineWeight(shortValue(pair))
	case 290:
		e.IsLandingEnabled = flagValue(pair)
	case 291:
		e.IsDoglegEnabled = flagValue(pair)
	case 41:
		e.DoglegLength = doubleValue(pair)
	case 342:
		e.ArrowheadHandle = handleFromString(stringValue(pair))
	case 42:
		e.ArrowheadSize = doubleValue(pair)
	case 172:
		e.ContentType = shortValue(pair)
	case 343:
		e.TextStyleHandle = handleFromString(stringValue(pair))
	case 173:
		e.TextLeftAttachment = shortValue(pair)
	case 95:
		e.TextRightAttachment = int16(intValue(pair))
	case 174:
		e.TextAngleType = shortValue(pair)
	case 175:
		e.TextAlignment = shortValue(pair)
	case 92:
		e.TextColor = ObjectColor(intValue(pair))
	case 292:
		e.IsTextFrameEnabled = flagValue(pair)
	case 344:
		e.BlockContentHandle = handleFromString(stringValue(pair))
	case 93:
		e.BlockContentColor = ObjectColor(intValue(pair))
	case 10:
		e.BlockContentScale.X = doubleValue(pair)
	case 20:
		e.BlockContentScale.Y = doubleValue(pair)
	case 30:
		e.BlockContentScale.Z = doubleValue(pair)
	case 43:
		e.BlockContentRotation = doubleValue(pair)
	case 176:
		e.BlockContentConnection = shortValue(pair)
	case 293:
		e.IsAnnotative = flagValue(pair)
	case 94:
		e.Arrowheads = append(e.Arrowheads, MLeaderArrowhead{Index: intValue(pair)})
	case 345:
		if len(e.Arrowheads) > 0 {
			e.Arrowheads[len(e.Arrowheads)-1].Handle = handleFromString(stringValue(pair))
		}
	case 330:
		e.BlockAttributes = append(e.BlockAttributes, MLeaderBlockAttribute{AttributeDefinitionHandle: handleFromString(stringValue(pair))})
	case 177, 44, 302:
		if len(e.BlockAttributes) == 0 {
			return
		}
		attribute := &e.BlockAttributes[len(e.BlockAttributes)-1]
		switch pair.Code {
		case 177:
			attribute.Index = shortValue(pair)
		case 44:
			attribute.Width = doubleValue(pair)
		case 302:
			attribute.Text = stringValue(pair)
		}
	case 294:
		e.IsTextDirectionNegative = flagValue(pair)
	case 178:
		e.TextAlignInIPE = shortValue(pair)
	case 179:
		e.TextAttachmentPoint = shortValue(pair)
	case 45:
		e.StyleScale = doubleValue(pair)
	case 271:
		e.TextAttachmentDirection = shortValue(pair)
	case 272:
		e.BottomTextAttachment = shortValue(pair)
	case 273:
		e.TopTextAttachment = shortValue(pair)
	case 295:
		e.IsLeaderExtendedToText = flagValue(pair)
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

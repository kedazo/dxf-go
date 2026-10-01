package dxf

import (
	"strings"
)

// UnknownEntity is an entity of a type this library does not model, e.g. a third-party object. Its own group codes
// are kept as they were read, so it can be inspected and written back. Application groups (102) are dropped because
// the objects they reference are not written; other references (e.g. to styles in the OBJECTS section) may not
// survive writing either.
type UnknownEntity struct {
	entityCommon

	// Type is the entity type, e.g. "ACAD_TABLE".
	Type string
	// CodePairs are the entity's group codes after the common entity data, including subclass markers and extended
	// data.
	CodePairs []CodePair

	isInApplicationGroup bool
	isInSubclassData     bool
}

// NewUnknownEntity creates an UnknownEntity of the given type.
func NewUnknownEntity(entityType string) *UnknownEntity {
	return &UnknownEntity{
		entityCommon: newEntityCommon(),
		Type:         entityType,
	}
}

func (e *UnknownEntity) typeString() string { return e.Type }

func (e *UnknownEntity) minVersion() AcadVersion {
	for _, pair := range e.CodePairs {
		if pair.Code == 100 {
			// subclass markers only exist since R13
			return R13
		}
	}
	return Version1_0
}

func (e *UnknownEntity) maxVersion() AcadVersion { return R2018 }

func (e *UnknownEntity) tryApplyCodePair(codePair CodePair) {
	if e.isInSubclassData {
		e.CodePairs = append(e.CodePairs, codePair)
		return
	}

	switch codePair.Code {
	case 100:
		if value, ok := codePair.Value.(StringCodePairValue); ok && value.Value == "AcDbEntity" {
			// written again with the common entity data
			return
		}
		e.isInSubclassData = true
		e.CodePairs = append(e.CodePairs, codePair)
	case 102:
		if value, ok := codePair.Value.(StringCodePairValue); ok {
			e.isInApplicationGroup = strings.HasPrefix(value.Value, "{")
		}
	default:
		if e.isInApplicationGroup || tryApplyCodePairForEntity(e, codePair) {
			return
		}
		e.CodePairs = append(e.CodePairs, codePair)
	}
}

func (e *UnknownEntity) codePairs(version AcadVersion) (pairs []CodePair) {
	pairs = append(pairs, NewStringCodePair(0, e.Type))
	pairs = append(pairs, codePairsForEntity(e, version)...)
	pairs = append(pairs, e.CodePairs...)
	return
}

// UnsupportedEntities counts the entities (including those in blocks) whose type this library does not model.
func (d *Drawing) UnsupportedEntities() map[string]int {
	counts := map[string]int{}
	d.forEachEntity(func(e *Entity) bool {
		if unknown, ok := (*e).(*UnknownEntity); ok {
			counts[unknown.Type]++
		}
		return true
	})
	return counts
}

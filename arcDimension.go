package dxf

// ARC_DIMENSION shares the common dimension data (AcDbDimension) with the other dimensions, but its own subclass
// reuses codes 70, 71 and 41, so the subclass decides what a code means.
func (this *ArcDimension) tryApplyCodePair(codePair CodePair) {
	if codePair.Code == 100 {
		this.lastSubclassMarker = stringValue(codePair)
		return
	}

	if this.lastSubclassMarker == "AcDbArcDimension" {
		switch codePair.Code {
		case 13, 23, 33:
			applyPointCodePair(&this.DefinitionPoint2, 13, codePair)
			return
		case 14, 24, 34:
			applyPointCodePair(&this.DefinitionPoint3, 14, codePair)
			return
		case 15, 25, 35:
			applyPointCodePair(&this.ArcCenter, 15, codePair)
			return
		case 16, 26, 36:
			applyPointCodePair(&this.LeaderPoint1, 16, codePair)
			return
		case 17, 27, 37:
			applyPointCodePair(&this.LeaderPoint2, 17, codePair)
			return
		case 40:
			this.StartAngle = doubleValue(codePair)
			return
		case 41:
			this.EndAngle = doubleValue(codePair)
			return
		case 70:
			this.IsPartial = shortValue(codePair) != 0
			return
		case 71:
			this.HasLeader = shortValue(codePair) != 0
			return
		}
	}

	if !tryApplyCodePairForDimension(this, codePair) {
		tryApplyCodePairForEntity(this, codePair)
	}
}

// applyPointCodePair applies the X (xCode), Y (xCode + 10) or Z (xCode + 20) coordinate of a point.
func applyPointCodePair(point *Point, xCode int, codePair CodePair) {
	value := doubleValue(codePair)
	switch codePair.Code {
	case xCode:
		point.X = value
	case xCode + 10:
		point.Y = value
	case xCode + 20:
		point.Z = value
	}
}

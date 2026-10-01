package dxf

import (
	"strings"
)

// Table is an ACAD_TABLE. Like an INSERT it shows an anonymous block (*T…) at InsertionPoint, which holds the
// table's lines and texts; Walk and Explode visit it that way. The fields describe the table as read; the entity is
// written back exactly as it was read, so changing them has no effect on the written table.
type Table struct {
	entityCommon

	BlockName           string
	InsertionPoint      Point
	HorizontalDirection Vector
	RowCount            int
	ColumnCount         int
	RowHeights          []float64
	ColumnWidths        []float64
	// CellTexts holds the text of every cell, row by row; cells without text have an empty string.
	CellTexts []string

	// the subclass data as read
	tableData          []CodePair
	lastSubclassMarker string
	isInCellData       bool
}

func newTable() *Table {
	return &Table{
		entityCommon:        newEntityCommon(),
		HorizontalDirection: *NewXAxis(),
	}
}

func (e *Table) typeString() string { return "ACAD_TABLE" }

func (e *Table) minVersion() AcadVersion { return R2004 }

func (e *Table) maxVersion() AcadVersion {
	if len(e.tableData) == 0 {
		// a table that wasn't read can't be written
		return Version1_0
	}
	return R2018
}

// CellText returns the text of a cell, or "" if the cell doesn't exist.
func (e *Table) CellText(row, column int) string {
	index := row*e.ColumnCount + column
	if row < 0 || column < 0 || column >= e.ColumnCount || index >= len(e.CellTexts) {
		return ""
	}
	return e.CellTexts[index]
}

func (e *Table) tryApplyCodePair(codePair CodePair) {
	if codePair.Code == 100 {
		marker := stringValue(codePair)
		if marker != "AcDbEntity" {
			e.lastSubclassMarker = marker
			e.tableData = append(e.tableData, codePair)
		}
		return
	}
	if e.lastSubclassMarker == "" {
		tryApplyCodePairForEntity(e, codePair)
		return
	}

	e.tableData = append(e.tableData, codePair)
	switch e.lastSubclassMarker {
	case "AcDbBlockReference":
		switch codePair.Code {
		case 2:
			e.BlockName = stringValue(codePair)
		case 10, 20, 30:
			applyPointCodePair(&e.InsertionPoint, 10, codePair)
		}
	case "AcDbTable":
		if codePair.Code == 171 || codePair.Code == 301 {
			// the cells reuse the header codes
			e.isInCellData = true
		}
		if e.isInCellData {
			return
		}
		switch codePair.Code {
		case 11:
			e.HorizontalDirection.X = doubleValue(codePair)
		case 21:
			e.HorizontalDirection.Y = doubleValue(codePair)
		case 31:
			e.HorizontalDirection.Z = doubleValue(codePair)
		case 91:
			e.RowCount = intValue(codePair)
		case 92:
			e.ColumnCount = intValue(codePair)
		case 141:
			e.RowHeights = append(e.RowHeights, doubleValue(codePair))
		case 142:
			e.ColumnWidths = append(e.ColumnWidths, doubleValue(codePair))
		}
	}
}

// parseCells reads the cell texts: cells start with code 171 (R2004) or 301 (R2007+, text in 302); R2004 text is
// split into chunks with codes 2 or 3 followed by 1.
func (e *Table) parseCells() {
	var tablePairs []CodePair
	inTable := false
	for _, pair := range e.tableData {
		if pair.Code == 100 {
			inTable = stringValue(pair) == "AcDbTable"
			continue
		}
		if inTable {
			tablePairs = append(tablePairs, pair)
		}
	}

	splitCode := 171
	for _, pair := range tablePairs {
		if pair.Code == 302 {
			splitCode = 301
			break
		}
	}

	e.CellTexts = nil
	var cell *strings.Builder
	var r2007Text *string
	flush := func() {
		if cell == nil {
			return
		}
		if r2007Text != nil {
			e.CellTexts = append(e.CellTexts, *r2007Text)
		} else {
			e.CellTexts = append(e.CellTexts, cell.String())
		}
	}
	for _, pair := range tablePairs {
		switch {
		case pair.Code == splitCode:
			flush()
			cell = &strings.Builder{}
			r2007Text = nil
		case cell == nil:
			// table header
		case pair.Code == 302 && r2007Text == nil:
			text := stringValue(pair)
			r2007Text = &text
		case pair.Code >= 1 && pair.Code <= 3 && splitCode == 171:
			cell.WriteString(stringValue(pair))
		}
	}
	flush()
}

func (e *Table) codePairs(version AcadVersion) (pairs []CodePair) {
	pairs = append(pairs, NewStringCodePair(0, "ACAD_TABLE"))
	pairs = append(pairs, codePairsForEntity(e, version)...)
	pairs = append(pairs, e.tableData...)
	return
}

// asInsert returns the INSERT that shows the table's block.
func (e *Table) asInsert() *Insert {
	insert := NewInsert()
	insert.Name = e.BlockName
	insert.Location = e.InsertionPoint
	insert.SetLayer(e.Layer())
	insert.SetColor(e.Color())
	insert.SetColor24Bit(e.Color24Bit())
	insert.SetColorName(e.ColorName())
	insert.SetLineTypeName(e.LineTypeName())
	insert.SetLineWeight(e.LineWeight())
	insert.SetIsInPaperSpace(e.IsInPaperSpace())
	return insert
}

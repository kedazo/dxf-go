package dxf

import (
	"errors"
	"math"
	"testing"
)

func lineEntity(p1, p2 Point) *Line {
	line := NewLine()
	line.P1 = p1
	line.P2 = p2
	return line
}

func insertEntity(name string, location Point) *Insert {
	insert := NewInsert()
	insert.Name = name
	insert.Location = location
	return insert
}

type visit struct {
	entity Entity
	matrix Matrix
	depth  int
}

func walkAll(t *testing.T, d *Drawing, options WalkOptions) ([]visit, []WalkIssue) {
	var visits []visit
	issues, err := d.Walk(options, func(e Entity, m Matrix, path []*Insert) error {
		visits = append(visits, visit{e, m, len(path)})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return visits, issues
}

func visitsOf[T Entity](visits []visit) (result []visit) {
	for _, v := range visits {
		if _, ok := v.entity.(T); ok {
			result = append(result, v)
		}
	}
	return
}

func TestWalkNestedInserts(t *testing.T) {
	drawing := *NewDrawing()
	// block A: a unit line starting at its base point
	drawing.Blocks = append(drawing.Blocks, Block{Name: "A", BasePoint: Point{1, 1, 0}, Entities: []Entity{lineEntity(Point{1, 1, 0}, Point{2, 1, 0})}})
	// block B: A scaled by 2 and rotated by 90° at (10, 0)
	inner := insertEntity("a", Point{10, 0, 0})
	inner.XScaleFactor, inner.YScaleFactor, inner.ZScaleFactor = 2, 2, 2
	inner.Rotation = 90
	drawing.Blocks = append(drawing.Blocks, Block{Name: "B", Entities: []Entity{inner}})
	// B mirrored along X at (100, 0)
	outer := insertEntity("B", Point{100, 0, 0})
	outer.XScaleFactor = -1
	drawing.Entities = append(drawing.Entities, outer)

	visits, issues := walkAll(t, &drawing, WalkOptions{})
	assertEqInt(t, 0, len(issues))
	assertEqInt(t, 3, len(visits))
	lines := visitsOf[*Line](visits)
	assertEqInt(t, 1, len(lines))
	assertEqInt(t, 2, lines[0].depth)

	line := lines[0].entity.(*Line)
	assertNearPoint(t, Point{90, 0, 0}, lines[0].matrix.TransformPoint(line.P1))
	assertNearPoint(t, Point{90, 2, 0}, lines[0].matrix.TransformPoint(line.P2))
	assertEqBool(t, true, lines[0].matrix.IsMirrored())
	// the original entity is not modified
	assertEqPoint(t, Point{1, 1, 0}, line.P1)
}

func TestWalkMInsertCells(t *testing.T) {
	drawing := *NewDrawing()
	drawing.Blocks = append(drawing.Blocks, Block{Name: "CELL", Entities: []Entity{lineEntity(Point{}, Point{1, 0, 0})}})
	insert := insertEntity("CELL", Point{5, 5, 0})
	insert.ColumnCount, insert.RowCount = 2, 3
	insert.ColumnSpacing, insert.RowSpacing = 10, 20
	insert.Rotation = 90
	drawing.Entities = append(drawing.Entities, insert)

	visits, _ := walkAll(t, &drawing, WalkOptions{})
	lines := visitsOf[*Line](visits)
	assertEqInt(t, 6, len(lines))
	// the last cell (column 1, row 2) is offset by (10, 40) before the rotation
	assertNearPoint(t, Point{5 - 40, 5 + 10, 0}, lines[5].matrix.TransformPoint(Point{}))
}

func TestWalkInsertWithExtrusion(t *testing.T) {
	drawing := *NewDrawing()
	drawing.Blocks = append(drawing.Blocks, Block{Name: "B", Entities: []Entity{lineEntity(Point{}, Point{1, 0, 0})}})
	insert := insertEntity("B", Point{2, 3, 4})
	insert.ExtrusionDirection = Vector{0, 0, -1}
	drawing.Entities = append(drawing.Entities, insert)

	visits, _ := walkAll(t, &drawing, WalkOptions{})
	m := visitsOf[*Line](visits)[0].matrix
	// the insertion point is in the INSERT's object coordinate system
	assertNearPoint(t, Point{-2, 3, -4}, m.TransformPoint(Point{}))
	assertNearPoint(t, Point{-3, 3, -4}, m.TransformPoint(Point{1, 0, 0}))
}

func TestWalkReportsCyclesMissingBlocksAndXrefs(t *testing.T) {
	drawing := *NewDrawing()
	drawing.Blocks = append(drawing.Blocks,
		Block{Name: "A", Entities: []Entity{insertEntity("B", Point{})}},
		Block{Name: "B", Entities: []Entity{insertEntity("A", Point{}), lineEntity(Point{}, Point{1, 0, 0})}},
		Block{Name: "SELF", Entities: []Entity{insertEntity("SELF", Point{})}},
		Block{Name: "X", Flags: 4, XrefName: "other.dwg"},
	)
	drawing.Entities = append(drawing.Entities,
		insertEntity("A", Point{}),
		insertEntity("SELF", Point{}),
		insertEntity("MISSING", Point{}),
		insertEntity("X", Point{}),
	)

	visits, issues := walkAll(t, &drawing, WalkOptions{})
	assertEqInt(t, 1, len(visitsOf[*Line](visits)))
	kinds := map[IssueKind]int{}
	for _, issue := range issues {
		kinds[issue.Kind]++
	}
	assertEqInt(t, 2, kinds[IssueCycle])
	assertEqInt(t, 1, kinds[IssueMissingBlock])
	assertEqInt(t, 1, kinds[IssueXref])
}

func TestWalkDepthLimit(t *testing.T) {
	drawing := *NewDrawing()
	drawing.Blocks = append(drawing.Blocks,
		Block{Name: "1", Entities: []Entity{insertEntity("2", Point{})}},
		Block{Name: "2", Entities: []Entity{insertEntity("3", Point{})}},
		Block{Name: "3", Entities: []Entity{lineEntity(Point{}, Point{1, 0, 0})}},
	)
	drawing.Entities = append(drawing.Entities, insertEntity("1", Point{}))

	visits, issues := walkAll(t, &drawing, WalkOptions{MaxDepth: 2})
	assertEqInt(t, 0, len(visitsOf[*Line](visits)))
	assertEqInt(t, 1, len(issues))
	assertEqInt(t, int(IssueDepthLimit), int(issues[0].Kind))
}

func TestWalkAttributesAndAttributeDefinitions(t *testing.T) {
	drawing := *NewDrawing()
	variable := NewAttributeDefinition()
	constant := NewAttributeDefinition()
	constant.SetIsConstant(true)
	drawing.Blocks = append(drawing.Blocks, Block{Name: "ROOM", Entities: []Entity{variable, constant}})
	insert := insertEntity("ROOM", Point{100, 0, 0})
	attribute := NewAttribute()
	attribute.Location = Point{101, 1, 0}
	insert.Attributes = []Attribute{*attribute}
	drawing.Entities = append(drawing.Entities, insert)

	visits, _ := walkAll(t, &drawing, WalkOptions{})
	assertEqInt(t, 1, len(visitsOf[*AttributeDefinition](visits)))
	attributes := visitsOf[*Attribute](visits)
	assertEqInt(t, 1, len(attributes))
	// attributes are already in the INSERT's coordinates
	assertNearPoint(t, Point{101, 1, 0}, attributes[0].matrix.TransformPoint(attributes[0].entity.(*Attribute).Location))

	visits, _ = walkAll(t, &drawing, WalkOptions{IncludeAttributeDefinitions: true})
	assertEqInt(t, 2, len(visitsOf[*AttributeDefinition](visits)))
}

func TestWalkSkipBlockAndErrors(t *testing.T) {
	drawing := *NewDrawing()
	drawing.Blocks = append(drawing.Blocks, Block{Name: "B", Entities: []Entity{lineEntity(Point{}, Point{1, 0, 0})}})
	drawing.Entities = append(drawing.Entities, insertEntity("B", Point{}), lineEntity(Point{}, Point{2, 0, 0}))

	count := 0
	_, err := drawing.Walk(WalkOptions{}, func(e Entity, m Matrix, path []*Insert) error {
		count++
		if _, ok := e.(*Insert); ok {
			return SkipBlock
		}
		return nil
	})
	assert(t, err == nil, "SkipBlock is not an error")
	assertEqInt(t, 2, count)

	stop := errors.New("stop")
	count = 0
	_, err = drawing.Walk(WalkOptions{}, func(e Entity, m Matrix, path []*Insert) error {
		count++
		return stop
	})
	assert(t, err == stop, "expected the callback's error")
	assertEqInt(t, 1, count)
}

func TestWalkInvisibleEntitiesAndDimensionBlocks(t *testing.T) {
	drawing := *NewDrawing()
	hidden := lineEntity(Point{}, Point{1, 0, 0})
	hidden.SetIsVisible(false)
	dimension := NewAlignedDimension()
	dimension.SetBlockName("*D1")
	drawing.Blocks = append(drawing.Blocks, Block{Name: "*D1", Flags: 1, Entities: []Entity{lineEntity(Point{}, Point{3, 0, 0})}})
	drawing.Entities = append(drawing.Entities, hidden, dimension)

	visits, _ := walkAll(t, &drawing, WalkOptions{})
	assertEqInt(t, 1, len(visits))

	visits, _ = walkAll(t, &drawing, WalkOptions{IncludeInvisible: true, IncludeDimensionBlocks: true})
	assertEqInt(t, 3, len(visits))
	assertEqInt(t, 2, len(visitsOf[*Line](visits)))

	// arc dimensions have dimension blocks too
	arcDimension := NewArcDimension()
	arcDimension.SetBlockName("*D1")
	drawing.Entities = []Entity{arcDimension}
	visits, _ = walkAll(t, &drawing, WalkOptions{IncludeDimensionBlocks: true})
	assertEqInt(t, 1, len(visitsOf[*Line](visits)))
}

func TestInsertMatrix(t *testing.T) {
	insert := insertEntity("B", Point{10, 20, 0})
	insert.XScaleFactor, insert.YScaleFactor = 2, 3
	insert.Rotation = 90
	block := &Block{BasePoint: Point{1, 1, 0}}
	m := InsertMatrix(insert, block, 0, 0)
	assertNearPoint(t, Point{10, 20, 0}, m.TransformPoint(Point{1, 1, 0}))
	// (1, 0) from the base point is scaled to (2, 0) and rotated to (0, 2)
	assertNearPoint(t, Point{10, 22, 0}, m.TransformPoint(Point{2, 1, 0}))
	assertNearFloat64(t, 6, m.Determinant3())
	assertNearFloat64(t, 0, math.Abs(InsertMatrix(insert, nil, 0, 0).TransformPoint(Point{}).Sub(Point{10, 20, 0}).Length()))
}

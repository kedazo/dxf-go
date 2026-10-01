package dxf

import (
	"errors"
	"fmt"
	"math"
	"strings"
)

// SkipBlock can be returned by a WalkFunc for an INSERT (or dimension) to not visit the block's contents and the
// INSERT's attributes.
var SkipBlock = errors.New("skip block")

// WalkFunc is called for every entity reachable through block references. m maps the entity's coordinates to world
// coordinates (it is the identity for top-level entities); path holds the INSERTs that lead to the entity, outermost
// first, and must not be kept after the call. Returning SkipBlock for an INSERT skips its contents, any other error
// stops the walk.
type WalkFunc func(e Entity, m Matrix, path []*Insert) error

// WalkOptions controls which entities Walk visits.
type WalkOptions struct {
	// IncludeDimensionBlocks visits the anonymous blocks (*D…) that hold the geometry of dimensions.
	IncludeDimensionBlocks bool
	// IncludeAttributeDefinitions visits the non-constant ATTDEFs inside blocks; AutoCAD only shows their values
	// through the INSERT's ATTRIBs. Constant ATTDEFs are always visited.
	IncludeAttributeDefinitions bool
	// IncludeInvisible visits invisible entities too.
	IncludeInvisible bool
	// MaxDepth limits how deeply blocks are nested; 0 means 64.
	MaxDepth int
}

// IssueKind classifies a WalkIssue.
type IssueKind int

const (
	IssueMissingBlock IssueKind = iota
	IssueCycle
	IssueXref
	IssueDepthLimit
	IssueDegenerate
	IssueUnsupported
	IssueApproximated
)

func (k IssueKind) String() string {
	switch k {
	case IssueMissingBlock:
		return "missing block"
	case IssueCycle:
		return "cyclic block reference"
	case IssueXref:
		return "external reference"
	case IssueDepthLimit:
		return "nesting too deep"
	case IssueDegenerate:
		return "degenerate transformation"
	case IssueUnsupported:
		return "unsupported entity"
	case IssueApproximated:
		return "approximated"
	}
	return fmt.Sprintf("IssueKind(%d)", int(k))
}

// WalkIssue reports something Walk or Explode could not handle exactly.
type WalkIssue struct {
	Kind    IssueKind
	Entity  Entity
	Path    []*Insert
	Message string
}

func (i WalkIssue) String() string {
	return fmt.Sprintf("%s: %s", i.Kind, i.Message)
}

// Walk visits the drawing's entities and, recursively, the contents of the blocks they reference.
func (d *Drawing) Walk(options WalkOptions, fn WalkFunc) ([]WalkIssue, error) {
	return d.WalkEntities(d.Entities, IdentityMatrix(), options, fn)
}

// WalkEntities is like Walk for a list of entities whose coordinates m maps to world coordinates.
func (d *Drawing) WalkEntities(entities []Entity, m Matrix, options WalkOptions, fn WalkFunc) ([]WalkIssue, error) {
	walker := blockWalker{drawing: d, options: options, fn: fn, blocks: map[string]*Block{}}
	if walker.options.MaxDepth <= 0 {
		walker.options.MaxDepth = 64
	}
	for i := range d.Blocks {
		name := strings.ToUpper(d.Blocks[i].Name)
		if _, exists := walker.blocks[name]; !exists {
			walker.blocks[name] = &d.Blocks[i]
		}
	}
	err := walker.walkEntities(entities, m, nil, nil)
	return walker.issues, err
}

type blockWalker struct {
	drawing *Drawing
	options WalkOptions
	fn      WalkFunc
	blocks  map[string]*Block
	issues  []WalkIssue
}

func (w *blockWalker) report(kind IssueKind, e Entity, path []*Insert, format string, args ...interface{}) {
	w.issues = append(w.issues, WalkIssue{Kind: kind, Entity: e, Path: append([]*Insert(nil), path...), Message: fmt.Sprintf(format, args...)})
}

func (w *blockWalker) walkEntities(entities []Entity, m Matrix, path []*Insert, blockNames []string) error {
	for _, e := range entities {
		if err := w.walkEntity(e, m, path, blockNames); err != nil {
			return err
		}
	}
	return nil
}

func (w *blockWalker) walkEntity(e Entity, m Matrix, path []*Insert, blockNames []string) error {
	if !w.options.IncludeInvisible && !e.IsVisible() {
		return nil
	}
	if attdef, ok := e.(*AttributeDefinition); ok && len(blockNames) > 0 && !w.options.IncludeAttributeDefinitions && !attdef.IsConstant() {
		return nil
	}

	visiblePath := path[:len(path):len(path)]
	err := w.fn(e, m, visiblePath)
	if err == SkipBlock {
		return nil
	}
	if err != nil {
		return err
	}

	switch ent := e.(type) {
	case *Insert:
		if err := w.walkInsert(ent, m, path, blockNames); err != nil {
			return err
		}
		for i := range ent.Attributes {
			// attributes are positioned like the INSERT itself, not in block coordinates
			if err := w.walkEntity(&ent.Attributes[i], m, path, blockNames); err != nil {
				return err
			}
		}
	case Dimension:
		if w.options.IncludeDimensionBlocks && ent.BlockName() != "" {
			if block := w.resolveBlock(e, ent.BlockName(), path, blockNames); block != nil {
				// the dimension block is in the same coordinates as the dimension
				return w.walkEntities(block.Entities, m, path, append(blockNames[:len(blockNames):len(blockNames)], strings.ToUpper(block.Name)))
			}
		}
	}
	return nil
}

func (w *blockWalker) walkInsert(insert *Insert, m Matrix, path []*Insert, blockNames []string) error {
	block := w.resolveBlock(insert, insert.Name, path, blockNames)
	if block == nil {
		return nil
	}

	insertPath := append(path[:len(path):len(path)], insert)
	nestedNames := append(blockNames[:len(blockNames):len(blockNames)], strings.ToUpper(block.Name))
	for row := 0; row < max(1, int(insert.RowCount)); row++ {
		for column := 0; column < max(1, int(insert.ColumnCount)); column++ {
			cellMatrix := m.Mul(InsertMatrix(insert, block, column, row))
			determinant := cellMatrix.Determinant3()
			if determinant == 0 || math.IsNaN(determinant) || math.IsInf(determinant, 0) {
				w.report(IssueDegenerate, insert, path, "INSERT of %q has a degenerate transformation", insert.Name)
				return nil
			}
			if err := w.walkEntities(block.Entities, cellMatrix, insertPath, nestedNames); err != nil {
				return err
			}
		}
	}
	return nil
}

func (w *blockWalker) resolveBlock(e Entity, name string, path []*Insert, blockNames []string) *Block {
	upperName := strings.ToUpper(name)
	block, ok := w.blocks[upperName]
	switch {
	case !ok:
		w.report(IssueMissingBlock, e, path, "block %q does not exist", name)
		return nil
	case block.IsXref() || block.XrefName != "":
		w.report(IssueXref, e, path, "block %q references the external drawing %q", name, block.XrefName)
		return nil
	case len(blockNames) >= w.options.MaxDepth:
		w.report(IssueDepthLimit, e, path, "block %q is nested deeper than %d levels", name, w.options.MaxDepth)
		return nil
	}
	for _, parentName := range blockNames {
		if parentName == upperName {
			w.report(IssueCycle, e, path, "block %q references itself", name)
			return nil
		}
	}
	return block
}

// InsertMatrix returns the transformation from the coordinates of block to world coordinates for one cell (column,
// row) of insert: the block's base point moves to the insertion point, then the block is scaled, the MINSERT cell
// offset is added, the result is rotated and finally placed in the INSERT's object coordinate system.
func InsertMatrix(insert *Insert, block *Block, column, row int) Matrix {
	basePoint := Point{}
	if block != nil {
		basePoint = block.BasePoint
	}
	return OCSToWCSMatrix(insert.ExtrusionDirection).
		Mul(TranslationMatrix(insert.Location.ToVector())).
		Mul(RotationZMatrix(insert.Rotation * math.Pi / 180.0)).
		Mul(TranslationMatrix(Vector{float64(column) * insert.ColumnSpacing, float64(row) * insert.RowSpacing, 0})).
		Mul(ScaleMatrix(insert.XScaleFactor, insert.YScaleFactor, insert.ZScaleFactor)).
		Mul(TranslationMatrix(basePoint.ToVector().Neg()))
}

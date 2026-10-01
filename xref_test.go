package dxf

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// xrefDrawings returns a host drawing with two INSERTs of the xref block "XR" and the drawing that "XR" points to:
// a LINE on layer "WALL", a LINE on layer "0", an INSERT of its own block "B" and a paper space LINE.
func xrefDrawings() (host, xref *Drawing) {
	xref = NewDrawing()
	wall := lineEntity(Point{0, 0, 0}, Point{1, 0, 0})
	wall.SetLayer("WALL")
	xref.Entities = append(xref.Entities, wall, lineEntity(Point{0, 1, 0}, Point{1, 1, 0}), insertEntity("B", Point{5, 5, 0}))
	paper := lineEntity(Point{100, 100, 0}, Point{200, 100, 0})
	paper.SetIsInPaperSpace(true)
	xref.Entities = append(xref.Entities, paper)
	block := NewBlock()
	block.Name = "B"
	block.Entities = []Entity{lineEntity(Point{0, 0, 0}, Point{0, 1, 0})}
	xref.Blocks = append(xref.Blocks, *block)
	wallLayer := *NewLayer()
	wallLayer.Name = "WALL"
	wallLayer.Color = 3
	xref.Layers = append(xref.Layers, wallLayer)

	host = NewDrawing()
	xrefBlock := NewBlock()
	xrefBlock.Name = "XR"
	xrefBlock.Flags = 4
	xrefBlock.XrefName = `xrefs\sub.dwg`
	host.Blocks = append(host.Blocks, *xrefBlock)
	first := insertEntity("XR", Point{10, 0, 0})
	first.SetLayer("PLAN")
	second := insertEntity("XR", Point{20, 0, 0})
	second.XScaleFactor, second.YScaleFactor = 2, 2
	host.Entities = append(host.Entities, first, second)
	return
}

func TestWalkWithoutXrefResolverReportsXrefs(t *testing.T) {
	host, _ := xrefDrawings()
	visits, issues := walkAll(t, host, WalkOptions{})
	assertEqInt(t, 2, len(visits))
	assertEqInt(t, 2, len(issues))
	assertEqInt(t, int(IssueXref), int(issues[0].Kind))
}

func TestWalkResolvesXrefs(t *testing.T) {
	host, xref := xrefDrawings()
	calls := 0
	options := WalkOptions{ResolveXref: func(block *Block) (*Drawing, error) {
		calls++
		assertEqString(t, `xrefs\sub.dwg`, block.XrefName)
		return xref, nil
	}}

	var lines []Line
	issues, err := host.Walk(options, func(e Entity, m Matrix, path []*Insert) error {
		if line, ok := e.(*Line); ok {
			assertEqString(t, "XR", path[0].Name)
			lines = append(lines, Line{P1: m.TransformPoint(line.P1), P2: m.TransformPoint(line.P2)})
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	assertEqInt(t, 0, len(issues))
	assertEqInt(t, 1, calls)
	// 3 model space lines per INSERT (the nested block "B" comes from the xref's own blocks), no paper space line
	assertEqInt(t, 6, len(lines))
	assertNearPoint(t, Point{10, 0, 0}, lines[0].P1)
	assertNearPoint(t, Point{15, 5, 0}, lines[2].P1)
	assertNearPoint(t, Point{22, 0, 0}, lines[3].P2)
	assertNearPoint(t, Point{30, 10, 0}, lines[5].P1)
}

func TestWalkReportsUnresolvedXrefs(t *testing.T) {
	host, _ := xrefDrawings()
	_, issues := walkAll(t, host, WalkOptions{ResolveXref: func(*Block) (*Drawing, error) { return nil, errors.New("no such file") }})
	assertEqInt(t, 1, len(issues))
	assertEqInt(t, int(IssueXref), int(issues[0].Kind))
	assertContains(t, "no such file", issues[0].Message)

	_, issues = walkAll(t, host, WalkOptions{ResolveXref: func(*Block) (*Drawing, error) { return nil, nil }})
	assertEqInt(t, 1, len(issues))
}

func TestExplodeXrefLayers(t *testing.T) {
	host, xref := xrefDrawings()
	result := host.Explode(ExplodeOptions{Recursive: true, InheritProperties: true, ResolveXref: func(*Block) (*Drawing, error) { return xref, nil }})
	assertEqInt(t, 0, len(result.Issues))
	assertEqInt(t, 6, len(result.Entities))
	// xref layers are named like AutoCAD's; layer "0" is resolved from the INSERT
	assertEqString(t, "XR|WALL", result.Entities[0].Layer())
	assertEqString(t, "PLAN", result.Entities[1].Layer())
	assertEqInt(t, 1, len(result.XrefLayers))
	assertEqString(t, "XR|WALL", result.XrefLayers[0].Name)
	assertEqInt(t, 3, int(result.XrefLayers[0].Color))
	// the xref drawing itself is unchanged
	assertEqString(t, "WALL", xref.Entities[0].Layer())
}

func writeXrefFile(t *testing.T, path string) {
	_, xref := xrefDrawings()
	xref.Header.Version = R2004
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := xref.SaveFile(path); err != nil {
		t.Fatal(err)
	}
}

func TestXrefFileResolverFindsGarbledNames(t *testing.T) {
	// "É-01 FÖLDSZINT" extracted from an archive with the wrong code page, folder included
	dir := t.TempDir()
	writeXrefFile(t, filepath.Join(dir, "Xrefed Nézetek", "É-01 FÖLDSZINT.dxf"))
	if err := os.Rename(filepath.Join(dir, "Xrefed Nézetek"), filepath.Join(dir, "Xrefed NВzetek")); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(dir, "Xrefed NВzetek", "É-01 FÖLDSZINT.dxf"), filepath.Join(dir, "Xrefed NВzetek", "Р-01 FЩLDSZINT.dxf")); err != nil {
		t.Fatal(err)
	}

	resolve := XrefFileResolver(dir)
	drawing, err := resolve(&Block{Name: "XR", XrefName: `Xrefed Nézetek\É-01 FÖLDSZINT.dwg`})
	if err != nil {
		t.Fatal(err)
	}
	assertEqInt(t, 4, len(drawing.Entities))

	// ASCII characters still have to match
	_, err = resolve(&Block{Name: "XR", XrefName: `Xrefed Nézetek\É-02 FÖLDSZINT.dwg`})
	assert(t, err != nil, "expected no match for a different ASCII character")

	// two candidates are ambiguous
	writeXrefFile(t, filepath.Join(dir, "Xrefed NВzetek", "Ъ-01 FЩLDSZINT.dxf"))
	_, err = XrefFileResolver(dir)(&Block{Name: "XR", XrefName: `Xrefed Nézetek\É-01 FÖLDSZINT.dwg`})
	assert(t, err != nil, "expected an ambiguous match to fail")
}

func TestXrefFileResolverConvertsDWG(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "plan.dwg"), []byte("not really a DWG"), 0o644); err != nil {
		t.Fatal(err)
	}

	// without a converter a DWG without a DXF next to it can't be resolved
	_, err := XrefFileResolver(dir)(&Block{Name: "XR", XrefName: "plan.dwg"})
	assert(t, err != nil, "expected an error without a converter")

	conversions := 0
	resolve := XrefFileResolverWith(dir, XrefFileResolverOptions{ConvertDWG: func(dwgPath string) (string, error) {
		conversions++
		assertEqString(t, filepath.Join(dir, "plan.dwg"), dwgPath)
		dxfPath := filepath.Join(dir, "converted", "plan.dxf")
		writeXrefFile(t, dxfPath)
		return dxfPath, nil
	}})
	for i := 0; i < 2; i++ {
		drawing, err := resolve(&Block{Name: "XR", XrefName: "plan.dwg"})
		if err != nil {
			t.Fatal(err)
		}
		assertEqInt(t, 4, len(drawing.Entities))
	}
	assertEqInt(t, 1, conversions)

	failing := XrefFileResolverWith(dir, XrefFileResolverOptions{ConvertDWG: func(string) (string, error) { return "", errors.New("broken") }})
	_, err = failing(&Block{Name: "XR", XrefName: "plan.dwg"})
	assertContains(t, "broken", err.Error())
}

func TestExplodeXrefLayerOverridesAndSources(t *testing.T) {
	host, xref := xrefDrawings()
	resolve := func(*Block) (*Drawing, error) { return xref, nil }

	// a host layer named "<xref>|<layer>" overrides the xref's layer (AutoCAD's VISRETAIN)
	override := *NewLayer()
	override.Name = "XR|WALL"
	override.Color = 5
	host.Layers = append(host.Layers, override)
	result := host.Explode(ExplodeOptions{Recursive: true, ResolveXref: resolve})
	assertEqInt(t, 1, len(result.XrefLayers))
	assertEqInt(t, 5, int(result.XrefLayers[0].Color))

	// every entity from the xref knows its drawing; the host's own entities don't
	assertEqInt(t, 6, len(result.XrefSources))
	assert(t, result.XrefSources[result.Entities[0]] == xref, "expected the xref drawing as the source")

	// merged: xref entities keep their layer names, and a host layer of the same name wins
	hostWall := *NewLayer()
	hostWall.Name = "WALL"
	hostWall.Color = 1
	host.Layers = append(host.Layers, hostWall)
	merged := host.Explode(ExplodeOptions{Recursive: true, ResolveXref: resolve, MergeXrefLayers: true})
	assertEqString(t, "WALL", merged.Entities[0].Layer())
	assertEqInt(t, 0, len(merged.XrefLayers))

	host.Layers = host.Layers[:len(host.Layers)-1]
	merged = host.Explode(ExplodeOptions{Recursive: true, ResolveXref: resolve, MergeXrefLayers: true})
	assertEqInt(t, 1, len(merged.XrefLayers))
	assertEqString(t, "WALL", merged.XrefLayers[0].Name)
	assertEqInt(t, 3, int(merged.XrefLayers[0].Color))
}

func TestXrefFileResolver(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "xrefs"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, xref := xrefDrawings()
	xref.Header.Version = R2004
	if err := xref.SaveFile(filepath.Join(dir, "xrefs", "sub.dxf")); err != nil {
		t.Fatal(err)
	}

	resolve := XrefFileResolver(dir)
	// a DWG reference is read from the DXF with the same name
	resolved, err := resolve(&Block{Name: "XR", XrefName: `xrefs\sub.dwg`})
	if err != nil {
		t.Fatal(err)
	}
	assertEqInt(t, 4, len(resolved.Entities))
	again, _ := resolve(&Block{Name: "XR2", XrefName: "xrefs/sub.dxf"})
	assert(t, again == resolved, "expected the file to be read once")

	_, err = resolve(&Block{Name: "MISSING", XrefName: "xrefs/missing.dwg"})
	assert(t, err != nil, "expected an error for a missing file")
	_, err = resolve(&Block{Name: "NOPATH"})
	assert(t, err != nil, "expected an error for an xref without a path")

	host, _ := xrefDrawings()
	result := host.Explode(ExplodeOptions{Recursive: true, ResolveXref: resolve})
	assertEqInt(t, 0, len(result.Issues))
	assertEqInt(t, 6, len(result.Entities))
}

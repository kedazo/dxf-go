package dxf

import (
	"fmt"
	"reflect"
	"testing"
)

var allEntityTypeStrings = []string{
	"3DFACE", "3DSOLID", "ACAD_PROXY_ENTITY", "ACAD_TABLE", "ARC", "ARC_DIMENSION", "ARCALIGNEDTEXT", "ATTDEF", "ATTRIB", "BODY", "CIRCLE", "ELLIPSE",
	"HATCH", "HELIX", "IMAGE", "INSERT", "LEADER", "LIGHT", "LINE", "LWPOLYLINE", "MESH", "MLINE", "MTEXT", "OLEFRAME",
	"OLE2FRAME", "POINT", "POLYLINE", "RAY", "REGION", "RTEXT", "SECTION", "SEQEND", "SHAPE", "SOLID", "SPLINE", "TEXT",
	"TOLERANCE", "TRACE", "DGNUNDERLAY", "DWFUNDERLAY", "PDFUNDERLAY", "VERTEX", "WIPEOUT", "XLINE",
}

func TestCloneEveryEntityType(t *testing.T) {
	for _, typeString := range allEntityTypeStrings {
		entity, ok := createEntity(typeString)
		if !ok {
			entity, ok = createCustomEntity(typeString)
		}
		assert(t, ok, "unknown entity type "+typeString)
		entity.SetHandle(0x42)
		entity.setOwnerPointerHandle(0x43)
		entity.SetPreviewImageData([]string{"AB"})

		clone := CloneEntity(entity)
		assertEqString(t, reflect.TypeOf(entity).String(), reflect.TypeOf(clone).String())
		assert(t, clone != entity, typeString+": clone is the same object")
		assertEqUInt64(t, 0, uint64(clone.Handle()))
		assertEqUInt64(t, 0, uint64(clone.getOwnerPointer().handle))
		assertEqUInt64(t, 0x42, uint64(entity.Handle()))
		assert(t, !sharesBacking(entity.PreviewImageData(), clone.PreviewImageData()), typeString+": preview data is shared")
	}
}

// sharesBacking reports whether two non-empty slices use the same backing array.
func sharesBacking(a, b interface{}) bool {
	va, vb := reflect.ValueOf(a), reflect.ValueOf(b)
	return va.Len() > 0 && vb.Len() > 0 && va.Index(0).Addr().Pointer() == vb.Index(0).Addr().Pointer()
}

// assertNoSharedSlices walks exported fields of two values and fails on slices or pointers that are shared.
func assertNoSharedSlices(t *testing.T, path string, a, b reflect.Value) {
	t.Helper()
	switch a.Kind() {
	case reflect.Struct:
		for i := 0; i < a.NumField(); i++ {
			if a.Type().Field(i).IsExported() {
				assertNoSharedSlices(t, path+"."+a.Type().Field(i).Name, a.Field(i), b.Field(i))
			}
		}
	case reflect.Slice:
		if a.Len() > 0 && b.Len() > 0 && a.Pointer() == b.Pointer() {
			t.Errorf("%s is shared", path)
		}
		for i := 0; i < a.Len() && i < b.Len(); i++ {
			assertNoSharedSlices(t, fmt.Sprintf("%s[%d]", path, i), a.Index(i), b.Index(i))
		}
	case reflect.Ptr, reflect.Interface:
		if a.IsNil() || b.IsNil() {
			return
		}
		if a.Kind() == reflect.Ptr && a.Pointer() == b.Pointer() {
			t.Errorf("%s is shared", path)
		}
		assertNoSharedSlices(t, path, a.Elem(), b.Elem())
	}
}

func TestCloneDoesNotShareData(t *testing.T) {
	polyline := NewPolyline()
	polyline.Vertices = []Vertex{*NewVertex(), *NewVertex()}
	polyline.Vertices[0].SetHandle(0x10)
	polyline.seqend.SetHandle(0x11)

	attribute := NewAttribute()
	attribute.SetHandle(0x20)
	attribute.MText.ExtendedText = []string{"chunk"}
	attribute.secondaryAttributeHandles = []string{"30"}
	insert := NewInsert()
	insert.Attributes = []Attribute{*attribute}

	spline := NewSpline()
	spline.ControlPoints = []ControlPoint{{Point{1, 2, 3}, 1}}
	spline.FitPoints = []Point{{1, 2, 3}}
	spline.KnotValues = []float64{0, 1}

	hatch := NewHatch()
	hatch.Paths = []HatchBoundaryPath{{
		Vertices:      [][2]float64{{1, 2}},
		Edges:         []HatchEdge{&HatchArcEdge{Radius: 1}, &HatchSplineEdge{Knots: []float64{0, 1}}},
		SourceHandles: []Handle{1},
	}}
	hatch.PatternLines = []HatchPatternLine{{Dashes: []float64{1}}}
	hatch.Gradient = &HatchGradient{Colors: []HatchGradientColor{{Value: 1}}}

	mtext := NewMText()
	mtext.ExtendedText = []string{"a"}
	mtext.ColumnHeights = []float64{1}

	leader := NewLeader()
	leader.Vertices = []Point{{1, 1, 1}}

	for _, entity := range []Entity{polyline, insert, spline, hatch, mtext, leader} {
		entity.SetLayer("WALL")
		clone := CloneEntity(entity)
		assertNoSharedSlices(t, reflect.TypeOf(entity).String(), reflect.ValueOf(entity).Elem(), reflect.ValueOf(clone).Elem())
		assertEqString(t, "WALL", clone.Layer())
	}

	clonedPolyline := CloneEntity(polyline).(*Polyline)
	assertEqUInt64(t, 0, uint64(clonedPolyline.Vertices[0].Handle()))
	assertEqUInt64(t, 0, uint64(clonedPolyline.seqend.Handle()))
	assertEqUInt64(t, 0x10, uint64(polyline.Vertices[0].Handle()))

	clonedInsert := CloneEntity(insert).(*Insert)
	assertEqUInt64(t, 0, uint64(clonedInsert.Attributes[0].Handle()))
	clonedInsert.Attributes[0].secondaryAttributeHandles[0] = "changed"
	assertEqString(t, "30", insert.Attributes[0].secondaryAttributeHandles[0])

	clonedHatch := CloneEntity(hatch).(*Hatch)
	clonedHatch.Paths[0].Edges[0].(*HatchArcEdge).Radius = 5
	assertEqFloat64(t, 1, hatch.Paths[0].Edges[0].(*HatchArcEdge).Radius)
	clonedHatch.Gradient.Colors[0].Value = 0
	assertEqFloat64(t, 1, hatch.Gradient.Colors[0].Value)
	assert(t, reflect.DeepEqual(hatch.Paths, CloneEntity(hatch).(*Hatch).Paths), "clone differs from the original")
}

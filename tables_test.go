package dxf

import (
	"testing"
)

func TestReadLayer(t *testing.T) {
	drawing := parseTableItem(t, "LAYER",
		NewStringCodePair(2, "layer-name"),
	)
	assertEqInt(t, 1, len(drawing.Layers))
	layer := drawing.Layers[0]
	assertEqString(t, "layer-name", layer.Name)
}

func TestWriteLayer(t *testing.T) {
	l := *NewLayer()
	l.Name = "layer-name"
	d := NewDrawing()
	d.Layers = append(d.Layers, l)
	actual, err := d.CodePairs()
	if err != nil {
		t.Error(err)
	}
	assertContainsCodePairs(t, []CodePair{
		NewStringCodePair(100, "AcDbSymbolTableRecord"),
		NewStringCodePair(2, "layer-name"),
		NewShortCodePair(70, 0),
		NewShortCodePair(62, 7),
		NewStringCodePair(6, "CONTINUOUS"),
	}, actual)
}

func TestRoundTripLayer(t *testing.T) {
	l := *NewLayer()
	l.Name = "layer-name"
	d := NewDrawing()
	d.Layers = append(d.Layers, l)
	r := roundTripDrawing(t, d)
	var l2 *Layer
	for i := range r.Layers {
		l2 = &r.Layers[i]
		if l2.Name == "layer-name" {
			break
		}
	}

	if l2 == nil {
		t.Errorf("Layer not found in round-tripped drawing")
	}
}

func TestReadTableItemHandles(t *testing.T) {
	layer := parseTableItem(t, "LAYER", NewStringCodePair(5, "2A"), NewStringCodePair(2, "layer")).Layers[0]
	assertEqUInt64(t, 0x2A, uint64(layer.Handle()))
	// DIMSTYLE uses 105 for its handle; 5 is its R14 arrow block name
	dimStyle := parseTableItem(t, "DIMSTYLE", NewStringCodePair(105, "2B"), NewStringCodePair(2, "style")).DimStyles[0]
	assertEqUInt64(t, 0x2B, uint64(dimStyle.Handle()))
}

func TestSavedHandlesDoNotCollideWithReadHandles(t *testing.T) {
	d := NewDrawing()
	d.Header.Version = R2004
	d.Header.NextAvailableHandle = 0x100
	layer := *NewLayer()
	layer.Name = "read"
	layer.SetHandle(0x2A)
	d.Layers = append(d.Layers, layer)
	read := NewLine()
	read.SetHandle(0x180)
	d.Entities = append(d.Entities, read, NewLine())

	seen := map[string]bool{}
	actual := drawingCodePairs(t, *d)
	for _, pair := range actual {
		if pair.Code != 5 && pair.Code != 105 {
			continue
		}
		handle := pair.Value.(StringCodePairValue).Value
		if seen[handle] {
			t.Errorf("handle %s is used twice", handle)
		}
		seen[handle] = true
		if value := handleFromString(handle); value != 0x2A && value != 0x180 && value <= 0x180 {
			t.Errorf("new handle %s is not above the kept handles", handle)
		}
	}
	assertEqBool(t, true, seen["2A"])
	assertEqBool(t, true, seen["180"])
}

func TestReadAndWriteLayerTrueColor(t *testing.T) {
	// 420 = 0 is true color black, not "no true color"
	drawing := parseTableItem(t, "LAYER",
		NewStringCodePair(2, "black"),
		NewShortCodePair(62, 18),
		NewIntCodePair(420, 0),
	)
	black := drawing.Layers[0]
	assertEqBool(t, true, black.HasColor24Bit)
	assertEqInt(t, 0, black.Color24Bit)

	unset := *NewLayer()
	unset.Name = "unset"
	assertEqBool(t, false, unset.HasColor24Bit)

	d := NewDrawing()
	d.Header.Version = R2004
	d.Layers = append(d.Layers, black, unset)
	actual := drawingCodePairs(t, *d)
	assertContainsCodePairs(t, []CodePair{
		NewStringCodePair(2, "black"),
		NewShortCodePair(70, 0),
		NewShortCodePair(62, 18),
		NewIntCodePair(420, 0),
		NewStringCodePair(6, "CONTINUOUS"),
	}, actual)
	assertEqInt(t, 1, len(codePairsWithCode(420, actual)))
}

func TestReadLayers(t *testing.T) {
	drawing := parseFromCodePairs(t,
		// section decl
		NewStringCodePair(0, "SECTION"),
		NewStringCodePair(2, "TABLES"),
		// table decl
		NewStringCodePair(0, "TABLE"),
		NewStringCodePair(2, "LAYER"),
		// item
		NewStringCodePair(0, "LAYER"),
		NewStringCodePair(2, "layer-1"),
		// item
		NewStringCodePair(0, "LAYER"),
		NewStringCodePair(2, "layer-2"),
		// end
		NewStringCodePair(0, "ENDTAB"),
		NewStringCodePair(0, "ENDSEC"),
		NewStringCodePair(0, "EOF"),
	)
	assertEqInt(t, 2, len(drawing.Layers))
	assertEqString(t, "layer-1", drawing.Layers[0].Name)
	assertEqString(t, "layer-2", drawing.Layers[1].Name)
}

func TestReadTableWithHandle(t *testing.T) {
	drawing := parseFromCodePairs(t,
		// section decl
		NewStringCodePair(0, "SECTION"),
		NewStringCodePair(2, "TABLES"),
		// table decl
		NewStringCodePair(0, "TABLE"),
		NewStringCodePair(2, "VPORT"),
		NewStringCodePair(5, "ABCD"), // n.b., handle is on the table, not the table item
		// item
		NewStringCodePair(0, "VPORT"),
		NewStringCodePair(2, "vport-name"),
		// end
		NewStringCodePair(0, "ENDTAB"),
		NewStringCodePair(0, "ENDSEC"),
		NewStringCodePair(0, "EOF"),
	)
	assertEqInt(t, 1, len(drawing.ViewPorts))
	assertEqString(t, "vport-name", drawing.ViewPorts[0].Name)
}

func TestUnsupportedTable(t *testing.T) {
	drawing := parseFromCodePairs(t,
		NewStringCodePair(0, "SECTION"),
		NewStringCodePair(2, "TABLES"),
		NewStringCodePair(0, "TABLE"),
		NewStringCodePair(2, "UNSUPPORTED"),
		NewStringCodePair(0, "UNSUPPORTED"),
		NewStringCodePair(2, "unsupported-name"),
		NewStringCodePair(0, "ENDTAB"),
		NewStringCodePair(0, "TABLE"),
		NewStringCodePair(2, "LAYER"),
		NewStringCodePair(0, "LAYER"),
		NewStringCodePair(2, "layer-name"),
		NewStringCodePair(0, "ENDTAB"),
		NewStringCodePair(0, "ENDSEC"),
		NewStringCodePair(0, "EOF"),
	)
	assertEqInt(t, 1, len(drawing.Layers))
	assertEqString(t, "layer-name", drawing.Layers[0].Name)
}

func parseTableItem(t *testing.T, tableType string, codePairs ...CodePair) (drawing Drawing) {
	allPairs := []CodePair{
		NewStringCodePair(0, "SECTION"),
		NewStringCodePair(2, "TABLES"),
		NewStringCodePair(0, "TABLE"),
		NewStringCodePair(2, tableType),
		NewStringCodePair(0, tableType),
	}
	allPairs = append(allPairs, codePairs...)
	allPairs = append(allPairs,
		NewStringCodePair(0, "ENDTAB"),
		NewStringCodePair(0, "ENDSEC"),
		NewStringCodePair(0, "EOF"),
	)
	drawing = parseFromCodePairs(t, allPairs...)
	return
}

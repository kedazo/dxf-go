# dxf-go

A Go library for reading and writing DXF CAD drawings, with a focus on reading real-world files exported by
architectural software such as ArchiCAD (which writes DXF through the ODA SDK).

It reads text and binary DXF from R12 up to AutoCAD 2018 (AC1032), including the header, tables, blocks and
entities, and writes drawings back.

## Features

- **Entities:** lines, arcs, circles, ellipses, (LW)polylines with bulges, polyface and polygon meshes, splines,
  text, MTEXT, attributes, inserts (including MINSERT arrays), dimensions (including ARC_DIMENSION), leaders,
  hatches, solids, 3D faces, images, viewports, MESH, ACAD_TABLE and MULTILEADER.
- **Layouts:** viewports with their model-to-paper transform (view target, twist and scale) and clip outline, the
  sheet size of each layout and the entities drawn on it, and the image files that IMAGE entities show.
- **Unsupported entity types** are kept as `UnknownEntity` with their raw group codes instead of being dropped.
- **HATCH:** polyline and edge boundaries (lines, arcs, ellipses, splines), bulges, patterns, gradients and seed
  points. Boundaries can be flattened to polygons.
- **Blocks:** `Walk` visits everything reachable through nested block references with its world transform, and
  `Explode` turns them into world-space copies. The transforms are exact, including mirrored, non-uniformly scaled
  and tilted blocks. External references (xrefs) can be resolved to other drawings: DXF files next to the host, DWG
  files converted with LibreDWG, and file names garbled by archives extracted with the wrong code page.
- **Text:** pre-2007 code pages (`$DWGCODEPAGE`, e.g. ANSI_1250), UTF-8 for 2007+, and plain text from MTEXT and
  TEXT formatting codes.
- **Geometry helpers:** vectors, matrices and the object coordinate system (OCS) used by planar entities.

## Usage

```bash
go get github.com/kedazo/dxf-go
```

The generated code is committed, so no `go generate` is needed to use the library.

```go
package main

import (
	"fmt"

	dxf "github.com/kedazo/dxf-go"
)

func main() {
	drawing, err := dxf.ReadFile("plan.dxf")
	if err != nil {
		panic(err)
	}

	// replace block references by their contents in world coordinates
	result := drawing.Explode(dxf.ExplodeOptions{Recursive: true, IncludeDimensionBlocks: true, InheritProperties: true})
	for _, entity := range result.Entities {
		switch e := entity.(type) {
		case *dxf.Line:
			fmt.Println("line", e.P1.String(), e.P2.String(), "on layer", e.Layer())
		case *dxf.Hatch:
			fmt.Println("hatch with", len(e.BoundaryPolygonsWCS(0.01)), "boundaries")
		case *dxf.MText:
			fmt.Println("text", e.PlainText())
		}
	}
}
```

More examples are in [example_test.go](example_test.go).

## Checking drawings

`cmd/dxfcheck` reads drawings and reports anything that looks parsed wrong or is missing. DWG files and DWG xrefs are
converted with LibreDWG's `dwg2dxf` first, and the conversion is checked against the DWG's own entity counts:

```bash
go run github.com/kedazo/dxf-go/cmd/dxfcheck [-keep dir] plan.dxf sheet.dxf xref.dwg
```

## Development

The entity, header and table code is generated from the XML files in `spec/`:

```bash
go generate ./
go test ./...
```

Never edit `*.generated.go` by hand; change the generator or the spec and regenerate.

## Changes compared to the original

This is a fork of [ixmilia/dxf-go](https://github.com/ixmilia/dxf-go). The most important changes:

- Faster reading (buffered input), Go 1.26, and tolerant parsing of malformed headers and comments, with
  warnings in `Drawing.Warnings`.
- BLOCKS section reading, with POLYLINE vertices and INSERT attributes grouped inside blocks.
- Complete HATCH support (read, write, flatten), new MESH, ACAD_TABLE, MULTILEADER and ARC_DIMENSION entities,
  and `UnknownEntity` for everything else.
- `Drawing.Walk` and `Drawing.Explode` for block references, with exact transforms.
- VIEWPORT entities, and LAYOUT and IMAGEDEF objects read from the OBJECTS section.
- Code page decoding for pre-2007 text (also in binary DXF), and MTEXT/TEXT plain text.
- Multiline attributes stored as embedded MText, as AutoCAD writes them.
- Fixes for upstream bugs:
  - all dimensions were read as aligned;
  - BYLAYER and BYBLOCK line weights were swapped;
  - numbers were rounded to 12 decimals when writing;
  - MLINE directions were wrong;
  - several crashes on malformed input.

## License

MIT, see [LICENSE.txt](LICENSE.txt). The original library is by Brett V. Forsgren / IxMilia.

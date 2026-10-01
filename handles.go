package dxf

// Handle returns the block's handle (code 5 of its BLOCK), or 0.
func (b *Block) Handle() Handle {
	return b.handle
}

// ParseHandle parses a handle stored as hexadecimal text, e.g. Layer.PlotStyleHandle or Entity.MaterialHandle(); it
// returns 0 if the text isn't a handle.
func ParseHandle(s string) Handle {
	return handleFromString(s)
}

// ItemByHandle returns what has the handle, or nil. It finds:
//   - entities, also inside blocks, and their sub-records: *Vertex of a POLYLINE, *Attribute of an INSERT;
//   - table records: *AppId, *BlockRecord, *DimStyle, *Layer, *LineType, *Style, *Ucs, *View, *ViewPort;
//   - blocks: *Block;
//   - objects: *Layout, *ImageDefinition, *RasterVariables, *WipeoutVariables, *MLeaderStyle, *TableStyle.
//
// Each call indexes the whole drawing; for many lookups, build HandleIndex once.
func (d *Drawing) ItemByHandle(h Handle) any {
	if h == 0 {
		return nil
	}
	return d.HandleIndex()[h]
}

// HandleIndex maps every handle in the drawing to what has it (see ItemByHandle). It is not updated when the drawing
// changes. If two items share a handle, the first one found wins: entities, then table records, blocks and objects.
func (d *Drawing) HandleIndex() map[Handle]any {
	index := make(map[Handle]any)
	add := func(h Handle, item any) {
		if _, exists := index[h]; h != 0 && !exists {
			index[h] = item
		}
	}

	addEntity := func(e Entity) {
		add(e.Handle(), e)
		switch e := e.(type) {
		case *Polyline:
			for i := range e.Vertices {
				add(e.Vertices[i].Handle(), &e.Vertices[i])
			}
		case *Insert:
			for i := range e.Attributes {
				add(e.Attributes[i].Handle(), &e.Attributes[i])
			}
		}
	}
	d.forEachEntity(func(e *Entity) bool {
		addEntity(*e)
		return true
	})

	for i := range d.AppIds {
		add(d.AppIds[i].Handle(), &d.AppIds[i])
	}
	for i := range d.BlockRecords {
		add(d.BlockRecords[i].Handle(), &d.BlockRecords[i])
	}
	for i := range d.DimStyles {
		add(d.DimStyles[i].Handle(), &d.DimStyles[i])
	}
	for i := range d.Layers {
		add(d.Layers[i].Handle(), &d.Layers[i])
	}
	for i := range d.LineTypes {
		add(d.LineTypes[i].Handle(), &d.LineTypes[i])
	}
	for i := range d.Styles {
		add(d.Styles[i].Handle(), &d.Styles[i])
	}
	for i := range d.Ucss {
		add(d.Ucss[i].Handle(), &d.Ucss[i])
	}
	for i := range d.Views {
		add(d.Views[i].Handle(), &d.Views[i])
	}
	for i := range d.ViewPorts {
		add(d.ViewPorts[i].Handle(), &d.ViewPorts[i])
	}
	for i := range d.Blocks {
		add(d.Blocks[i].Handle(), &d.Blocks[i])
	}

	for i := range d.Layouts {
		add(d.Layouts[i].Handle, &d.Layouts[i])
	}
	for i := range d.ImageDefinitions {
		add(d.ImageDefinitions[i].Handle, &d.ImageDefinitions[i])
	}
	if d.RasterVariables != nil {
		add(d.RasterVariables.Handle, d.RasterVariables)
	}
	if d.WipeoutVariables != nil {
		add(d.WipeoutVariables.Handle, d.WipeoutVariables)
	}
	for i := range d.MLeaderStyles {
		add(d.MLeaderStyles[i].Handle, &d.MLeaderStyles[i])
	}
	for i := range d.TableStyles {
		add(d.TableStyles[i].Handle, &d.TableStyles[i])
	}
	return index
}

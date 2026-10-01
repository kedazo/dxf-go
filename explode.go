package dxf

import (
	"strings"
)

// ExplodeOptions controls Explode and ExplodeInsert.
type ExplodeOptions struct {
	// Recursive also explodes the INSERTs inside exploded blocks; otherwise they are kept as transformed INSERTs.
	Recursive bool
	// IncludeDimensionBlocks replaces dimensions with the geometry of their anonymous (*D…) blocks.
	IncludeDimensionBlocks bool
	// InheritProperties resolves entities on layer "0" and BYBLOCK colors, line types and line weights from the
	// INSERTs, the way AutoCAD displays them.
	InheritProperties bool
	// KeepNegativeExtrusion keeps the (0, 0, -1) extrusion directions mirroring produces; by default such entities
	// are rewritten with (0, 0, 1) so consumers that ignore extrusion directions draw them correctly.
	KeepNegativeExtrusion bool
	// ResolveXref supplies the drawings of external references, see WalkOptions.ResolveXref. Their entities are put
	// on "<xref block>|<layer>" layers (layer "0" stays "0"), the way AutoCAD names xref layers; XrefLayers lists
	// those layers.
	ResolveXref func(block *Block) (*Drawing, error)
	// MergeXrefLayers keeps the layer names of xref entities, like binding xrefs into the host: a host layer with the
	// same name wins (ArchiCAD exports use the same layer names in a sheet and its xrefs), and XrefLayers only lists
	// the layers the host doesn't have.
	MergeXrefLayers bool
}

// ExplodeResult holds the exploded entities and everything that could not be exploded exactly.
type ExplodeResult struct {
	Entities []Entity
	Issues   []WalkIssue
	// XrefLayers are the layers of the resolved external references, named "<xref block>|<layer>" like their
	// exploded entities, so their colors, line types and line weights can be looked up. A host layer with that name
	// overrides the xref's (AutoCAD keeps such overrides with VISRETAIN).
	XrefLayers []Layer
	// XrefSources maps every entity that comes from an external reference to the drawing it was read from, whose
	// line types, text styles, dimension styles and image definitions apply to it.
	XrefSources map[Entity]*Drawing
}

// Explode returns the drawing's entities with every INSERT replaced by copies of its block's entities in world
// coordinates. Visible attributes become TEXT (or MTEXT for multiline attributes), invisible entities are left out
// and the drawing itself is not changed.
func (d *Drawing) Explode(options ExplodeOptions) ExplodeResult {
	return d.explodeEntities(d.Entities, options)
}

// ExplodeInsert returns the contents of one INSERT (including its attributes as TEXT) in world coordinates.
func (d *Drawing) ExplodeInsert(insert *Insert, options ExplodeOptions) ExplodeResult {
	return d.explodeEntities([]Entity{insert}, options)
}

func (d *Drawing) explodeEntities(entities []Entity, options ExplodeOptions) (result ExplodeResult) {
	walkOptions := WalkOptions{IncludeDimensionBlocks: options.IncludeDimensionBlocks, ResolveXref: options.ResolveXref}
	attributeOwners := map[*Attribute]*Insert{}
	xrefsWithLayers := map[string]bool{}
	var source walkSource

	emitTransformed := func(transformed []Entity, issues []WalkIssue, path []*Insert) {
		for _, issue := range issues {
			issue.Path = append([]*Insert(nil), path...)
			result.Issues = append(result.Issues, issue)
		}
		for _, entity := range transformed {
			if len(path) > 0 {
				entity.SetIsInPaperSpace(path[0].IsInPaperSpace())
			}
			if source.xrefName != "" {
				if entity.Layer() != "0" && !options.MergeXrefLayers {
					entity.SetLayer(source.xrefName + "|" + entity.Layer())
				}
				if result.XrefSources == nil {
					result.XrefSources = map[Entity]*Drawing{}
				}
				result.XrefSources[entity] = source.drawing
			}
			if options.InheritProperties {
				ResolveInherited(entity, path)
			}
			result.Entities = append(result.Entities, entity)
		}
	}
	emit := func(e Entity, m Matrix, path []*Insert) {
		transformed, issues := TransformEntity(e, m, options)
		emitTransformed(transformed, issues, path)
	}

	walkIssues, _ := d.walkEntitiesFrom(entities, IdentityMatrix(), walkOptions, func(e Entity, m Matrix, path []*Insert, from walkSource) error {
		source = from
		if source.xrefName != "" && !xrefsWithLayers[source.xrefName] {
			xrefsWithLayers[source.xrefName] = true
			result.XrefLayers = append(result.XrefLayers, xrefLayers(d, source, options.MergeXrefLayers)...)
		}
		switch ent := e.(type) {
		case *Insert:
			for i := range ent.Attributes {
				attributeOwners[&ent.Attributes[i]] = ent
			}
			if len(path) == 0 || options.Recursive {
				// the walk continues with the block's contents
				return nil
			}
			transformed, issues := TransformEntity(e, m, options)
			if len(transformed) == 0 {
				// an INSERT can't be sheared: explode this one too
				result.Issues = append(result.Issues, WalkIssue{Kind: IssueApproximated, Entity: e, Path: append([]*Insert(nil), path...),
					Message: "the nested INSERT of " + ent.Name + " can't be kept under this transformation, so it was exploded"})
				return nil
			}
			emitTransformed(transformed, issues, path)
			return SkipBlock
		case *Table:
			// tables are always exploded into their block's lines and texts
			return nil
		case *Attribute:
			if ent.IsInvisible() {
				return nil
			}
			ownerPath := path
			if owner := attributeOwners[ent]; owner != nil {
				ownerPath = append(path[:len(path):len(path)], owner)
			}
			emit(attributeAsText(ent), m, ownerPath)
		case *AttributeDefinition:
			if len(path) > 0 {
				// only constant attribute definitions are shown as part of a block
				emit(attributeDefinitionAsText(ent), m, path)
			} else {
				emit(e, m, path)
			}
		case Dimension:
			if !options.IncludeDimensionBlocks || ent.BlockName() == "" {
				emit(e, m, path)
			}
		default:
			emit(e, m, path)
		}
		return nil
	})

	result.Issues = append(walkIssues, result.Issues...)
	return
}

// xrefLayers returns the layers an xref adds to the host: renamed "<xref>|<layer>" unless merged, with the host's
// layer of the same name taking precedence.
func xrefLayers(host *Drawing, source walkSource, merge bool) (layers []Layer) {
	hostLayers := make(map[string]*Layer, len(host.Layers))
	for i := range host.Layers {
		hostLayers[strings.ToUpper(host.Layers[i].Name)] = &host.Layers[i]
	}
	for _, layer := range source.drawing.Layers {
		if layer.Name == "0" {
			continue
		}
		if !merge {
			layer.Name = source.xrefName + "|" + layer.Name
		}
		if override, ok := hostLayers[strings.ToUpper(layer.Name)]; ok {
			if merge {
				continue
			}
			layer = *override
		}
		layer.SetHandle(0)
		layers = append(layers, layer)
	}
	return
}

// ResolveInherited applies what an entity inherits from the INSERTs it is shown through (outermost first): an
// entity on layer "0" is shown on the INSERT's layer, BYBLOCK colors, line types and line weights are the INSERT's.
func ResolveInherited(e Entity, path []*Insert) {
	for i := len(path) - 1; i >= 0; i-- {
		insert := path[i]
		if e.Layer() == "0" {
			e.SetLayer(insert.Layer())
		}
		if e.Color() == ByBlock() && !e.HasColor24Bit() {
			e.SetColor(insert.Color())
			copyColor24Bit(insert, e)
			e.SetColorName(insert.ColorName())
		}
		if strings.EqualFold(e.LineTypeName(), "BYBLOCK") {
			e.SetLineTypeName(insert.LineTypeName())
		}
		if e.LineWeight() == LineWeightByBlock {
			e.SetLineWeight(insert.LineWeight())
		}
	}
}

// attributeAsText returns the TEXT (or the MTEXT of a multiline attribute) that shows an attribute's value.
func attributeAsText(attribute *Attribute) Entity {
	if attribute.IsMultiline() && attribute.MText.FormattedText() != "" {
		mtext := CloneEntity(&attribute.MText).(*MText)
		copyEntityProperties(attribute, mtext)
		return mtext
	}

	text := NewText()
	copyEntityProperties(attribute, text)
	text.Thickness = attribute.Thickness
	text.Location = attribute.Location
	text.Height = attribute.TextHeight
	text.Value = attribute.Value
	text.Rotation = attribute.Rotation
	text.RelativeXScaleFactor = attribute.RelativeXScaleFactor
	text.ObliqueAngle = attribute.ObliqueAngle
	text.TextStyleName = attribute.TextStyleName
	text.TextGenerationFlags = attribute.TextGenerationFlags
	text.HorizontalTextJustification = attribute.HorizontalTextJustification
	text.VerticalTextJustification = attribute.VerticalTextJustification
	text.SecondAlignmentPoint = attribute.SecondAlignmentPoint
	text.Normal = attribute.Normal
	return text
}

// attributeDefinitionAsText returns the TEXT that shows a constant attribute definition's value.
func attributeDefinitionAsText(definition *AttributeDefinition) Entity {
	if definition.IsMultiline() && definition.MText.FormattedText() != "" {
		mtext := CloneEntity(&definition.MText).(*MText)
		copyEntityProperties(definition, mtext)
		return mtext
	}

	text := NewText()
	copyEntityProperties(definition, text)
	text.Thickness = definition.Thickness
	text.Location = definition.Location
	text.Height = definition.TextHeight
	text.Value = definition.Value
	text.Rotation = definition.Rotation
	text.RelativeXScaleFactor = definition.RelativeXScaleFactor
	text.ObliqueAngle = definition.ObliqueAngle
	text.TextStyleName = definition.TextStyleName
	text.TextGenerationFlags = definition.TextGenerationFlags
	text.HorizontalTextJustification = definition.HorizontalTextJustification
	text.VerticalTextJustification = definition.VerticalTextJustification
	text.SecondAlignmentPoint = definition.SecondAlignmentPoint
	text.Normal = definition.Normal
	return text
}

func copyEntityProperties(from, to Entity) {
	to.SetIsInPaperSpace(from.IsInPaperSpace())
	to.SetLayer(from.Layer())
	to.SetLineTypeName(from.LineTypeName())
	to.SetElevation(from.Elevation())
	to.SetMaterialHandle(from.MaterialHandle())
	to.SetColor(from.Color())
	to.SetLineWeight(from.LineWeight())
	to.SetLineTypeScale(from.LineTypeScale())
	to.SetIsVisible(from.IsVisible())
	copyColor24Bit(from, to)
	to.SetColorName(from.ColorName())
	to.SetTransparency(from.Transparency())
	to.SetShadowMode(from.ShadowMode())
}

// copyColor24Bit copies the true color, including whether one is set at all (0 is black, not "no true color").
func copyColor24Bit(from, to Entity) {
	if from.HasColor24Bit() {
		to.SetColor24Bit(from.Color24Bit())
	} else {
		to.ClearColor24Bit()
	}
}

package dxf

import (
	"reflect"
)

// CloneEntity returns a deep copy of an entity that can be added to a drawing: slices, nested structs and boundary
// edges are copied, the handles of the copy and its sub-entities (vertices, attributes, SEQEND) are cleared and it has
// no owner.
func CloneEntity(e Entity) Entity {
	original := reflect.ValueOf(e)
	if original.Kind() != reflect.Ptr || original.IsNil() {
		return e
	}

	copied := reflect.New(original.Elem().Type())
	copied.Elem().Set(original.Elem())
	deepCopyExportedFields(copied.Elem())

	clone := copied.Interface().(Entity)
	resetClonedEntity(clone)
	return clone
}

// deepCopyExportedFields replaces the exported slices, pointers and interfaces reachable from v with copies.
func deepCopyExportedFields(v reflect.Value) {
	switch v.Kind() {
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if v.Type().Field(i).IsExported() {
				deepCopyExportedFields(v.Field(i))
			}
		}
	case reflect.Slice:
		if v.IsNil() {
			return
		}
		copied := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
		reflect.Copy(copied, v)
		for i := 0; i < copied.Len(); i++ {
			deepCopyExportedFields(copied.Index(i))
		}
		v.Set(copied)
	case reflect.Ptr:
		if v.IsNil() {
			return
		}
		copied := reflect.New(v.Type().Elem())
		copied.Elem().Set(v.Elem())
		deepCopyExportedFields(copied.Elem())
		v.Set(copied)
	case reflect.Interface:
		if v.IsNil() {
			return
		}
		copied := reflect.New(v.Elem().Type()).Elem()
		copied.Set(v.Elem())
		deepCopyExportedFields(copied)
		v.Set(copied)
	case reflect.Map:
		if v.IsNil() {
			return
		}
		copied := reflect.MakeMapWithSize(v.Type(), v.Len())
		iterator := v.MapRange()
		for iterator.Next() {
			value := reflect.New(iterator.Value().Type()).Elem()
			value.Set(iterator.Value())
			deepCopyExportedFields(value)
			copied.SetMapIndex(iterator.Key(), value)
		}
		v.Set(copied)
	}
}

// resetClonedEntity copies the unexported slices that reflection can't reach and clears handles and owners.
func resetClonedEntity(e Entity) {
	resetCommonEntityFields(e)

	switch ent := e.(type) {
	case *Insert:
		ent.seqend.SetHandle(0)
		for i := range ent.Attributes {
			resetCommonEntityFields(&ent.Attributes[i])
			ent.Attributes[i].secondaryAttributeHandles = cloneSlice(ent.Attributes[i].secondaryAttributeHandles)
			resetCommonEntityFields(&ent.Attributes[i].MText)
		}
	case *Polyline:
		ent.seqend.SetHandle(0)
		for i := range ent.Vertices {
			resetCommonEntityFields(&ent.Vertices[i])
		}
	case *Attribute:
		ent.secondaryAttributeHandles = cloneSlice(ent.secondaryAttributeHandles)
		resetCommonEntityFields(&ent.MText)
	case *AttributeDefinition:
		resetCommonEntityFields(&ent.MText)
	case *Hatch:
		ent.hatchData = nil
	case *Mesh:
		ent.meshData = nil
		ent.overrideData = cloneSlice(ent.overrideData)
	case *Table:
		ent.tableData = cloneSlice(ent.tableData)
	case *MLeader:
		ent.leaderData = cloneSlice(ent.leaderData)
	case *ProxyEntity:
		ent.graphicsDataString = cloneSlice(ent.graphicsDataString)
		ent.entityDataString = cloneSlice(ent.entityDataString)
	case *Image:
		ent.clippingVertices = cloneSlice(ent.clippingVertices)
	case *Wipeout:
		ent.clippingVertices = cloneSlice(ent.clippingVertices)
	case *Leader:
		ent.verticesX = cloneSlice(ent.verticesX)
		ent.verticesY = cloneSlice(ent.verticesY)
		ent.verticesZ = cloneSlice(ent.verticesZ)
	case *MLine:
		ent.vertexX = cloneSlice(ent.vertexX)
		ent.vertexY = cloneSlice(ent.vertexY)
		ent.vertexZ = cloneSlice(ent.vertexZ)
		ent.segmentDirectionX = cloneSlice(ent.segmentDirectionX)
		ent.segmentDirectionY = cloneSlice(ent.segmentDirectionY)
		ent.segmentDirectionZ = cloneSlice(ent.segmentDirectionZ)
		ent.miterDirectionX = cloneSlice(ent.miterDirectionX)
		ent.miterDirectionY = cloneSlice(ent.miterDirectionY)
		ent.miterDirectionZ = cloneSlice(ent.miterDirectionZ)
		ent.parameterCounts = cloneSlice(ent.parameterCounts)
		ent.areaFillParameterCounts = cloneSlice(ent.areaFillParameterCounts)
	case *OleFrame:
		ent.binaryDataStrings = cloneSlice(ent.binaryDataStrings)
	case *Ole2Frame:
		ent.binaryDataStrings = cloneSlice(ent.binaryDataStrings)
	case *Spline:
		ent.weights = cloneSlice(ent.weights)
	case *DgnUnderlay:
		ent.boundaryPoints = cloneSlice(ent.boundaryPoints)
	case *DwfUnderlay:
		ent.boundaryPoints = cloneSlice(ent.boundaryPoints)
	case *PdfUnderlay:
		ent.boundaryPoints = cloneSlice(ent.boundaryPoints)
	}
}

func resetCommonEntityFields(e Entity) {
	e.SetHandle(0)
	e.SetOwner(nil)
	e.setOwnerPointerHandle(0)
	e.SetPreviewImageData(cloneSlice(e.PreviewImageData()))
	e.SetXData(e.XData().clone())
}

func cloneSlice[T any](s []T) []T {
	if s == nil {
		return nil
	}
	return append(make([]T, 0, len(s)), s...)
}

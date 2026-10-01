package dxf

import "strings"

// XData is the extended data of an entity, table record, block or object: the values that applications (1001)
// attach to it, in file order. It is read only; the writer doesn't write it (UnknownEntity keeps its own copy in
// CodePairs).
type XData []XDataApplication

// XDataApplication is the extended data of one application, e.g. "ACAD".
type XDataApplication struct {
	Name  string // code 1001, a registered application (APPID)
	Items []XDataItem
}

// XDataItem is one extended data value. Code tells which field holds it:
//   - 1000 (string), 1003 (layer name), 1004 (binary data, as hex text), 1005 (handle, as hex text): String;
//   - 1010 (point), 1011 (world position), 1012 (world displacement), 1013 (world direction): Point;
//   - 1040 (real), 1041 (distance), 1042 (scale factor): Real;
//   - 1070 (16-bit integer), 1071 (32-bit integer): Int;
//   - 1002: a list, `{` … `}` in the file, with its values in Items.
type XDataItem struct {
	Code   int
	String string
	Point  Point
	Real   float64
	Int    int
	Items  []XDataItem
}

// Application returns the data of the named application (case-insensitive), or nil.
func (x XData) Application(name string) *XDataApplication {
	for i := range x {
		if strings.EqualFold(x[i].Name, name) {
			return &x[i]
		}
	}
	return nil
}

// TrueTypeFont returns the TrueType font of a text style, as AutoCAD keeps it in the style's ACAD extended data: the
// family name (e.g. "Arial Narrow", where FontFile is ARIALN.TTF) and the bold (bit 25) and italic (bit 24) flags of
// code 1071, which is also FontFlags. ok is false if the style has no such data.
func (s *Style) TrueTypeFont() (family string, bold, italic, ok bool) {
	application := s.XData.Application("ACAD")
	if application == nil {
		return "", false, false, false
	}
	for _, item := range application.Items {
		switch item.Code {
		case 1000:
			if !ok {
				family, ok = item.String, true
			}
		case 1071:
			bold, italic = item.Int&0x2000000 != 0, item.Int&0x1000000 != 0
		}
	}
	return family, bold, italic, ok
}

// clone returns a deep copy.
func (x XData) clone() XData {
	if x == nil {
		return nil
	}
	copied := make(XData, len(x))
	for i, application := range x {
		copied[i] = XDataApplication{Name: application.Name, Items: cloneXDataItems(application.Items)}
	}
	return copied
}

func cloneXDataItems(items []XDataItem) []XDataItem {
	if items == nil {
		return nil
	}
	copied := make([]XDataItem, len(items))
	for i, item := range items {
		copied[i] = item
		copied[i].Items = cloneXDataItems(item.Items)
	}
	return copied
}

// xdataReader collects extended data from an item's code pairs: everything from the first 1001 on.
type xdataReader struct {
	data XData
	// lists are the open 1002 lists, innermost last
	lists [][]XDataItem
}

// add takes a code pair of the item; pairs before the first 1001 are ignored.
func (r *xdataReader) add(pair CodePair) {
	switch {
	case pair.Code == 1001:
		r.closeLists()
		r.data = append(r.data, XDataApplication{Name: stringValue(pair)})
	case pair.Code < 1000 || len(r.data) == 0:
		// not extended data
	case pair.Code == 1002:
		if stringValue(pair) == "}" {
			if len(r.lists) > 0 {
				list := r.lists[len(r.lists)-1]
				r.lists = r.lists[:len(r.lists)-1]
				r.append(XDataItem{Code: 1002, Items: list})
			}
		} else {
			r.lists = append(r.lists, []XDataItem{})
		}
	case pair.Code >= 1020 && pair.Code <= 1033:
		// the Y and Z of the last point
		if items := r.items(); len(*items) > 0 && (*items)[len(*items)-1].Code == pair.Code-10*((pair.Code-1010)/10) {
			applyPointCodePair(&(*items)[len(*items)-1].Point, pair.Code-10*((pair.Code-1010)/10), pair)
		}
	case pair.Code >= 1010 && pair.Code <= 1013:
		r.append(XDataItem{Code: pair.Code, Point: Point{X: doubleValue(pair)}})
	case pair.Code >= 1040 && pair.Code <= 1042:
		r.append(XDataItem{Code: pair.Code, Real: doubleValue(pair)})
	case pair.Code == 1070 || pair.Code == 1071:
		r.append(XDataItem{Code: pair.Code, Int: intValue(pair)})
	default:
		r.append(XDataItem{Code: pair.Code, String: stringValue(pair)})
	}
}

// items returns where the next value goes: the innermost open list, or the current application.
func (r *xdataReader) items() *[]XDataItem {
	if len(r.lists) > 0 {
		return &r.lists[len(r.lists)-1]
	}
	return &r.data[len(r.data)-1].Items
}

func (r *xdataReader) append(item XDataItem) {
	items := r.items()
	*items = append(*items, item)
}

// closeLists ends lists that weren't closed, keeping their values.
func (r *xdataReader) closeLists() {
	for len(r.lists) > 0 {
		r.add(NewStringCodePair(1002, "}"))
	}
}

// result returns the extended data read, or nil if there was none.
func (r *xdataReader) result() XData {
	if len(r.data) > 0 {
		r.closeLists()
	}
	return r.data
}

// xdataFromPairs reads the extended data among an item's code pairs.
func xdataFromPairs(pairs []CodePair) XData {
	var reader xdataReader
	for _, pair := range pairs {
		reader.add(pair)
	}
	return reader.result()
}

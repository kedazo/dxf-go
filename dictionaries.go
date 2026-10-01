package dxf

import "strings"

// Dictionary is a DICTIONARY (or ACDBDICTIONARYWDFLT) object: named references to other objects. The first
// dictionary of the OBJECTS section is the named object dictionary, the root of all others (see
// Drawing.NamedObjectDictionary); e.g. its ACAD_MLEADERSTYLE entry is the dictionary of the multileader styles by name.
type Dictionary struct {
	Handle                 Handle
	OwnerHandle            Handle // code 330; 0 for the named object dictionary
	IsHardOwner            bool   // code 280: the entries are owned by the dictionary
	DuplicateRecordCloning int16  // code 281
	Entries                []DictionaryEntry
	// DefaultHandle is the default entry of an ACDBDICTIONARYWDFLT (code 340), e.g. the "Normal" plot style; 0 for a
	// plain DICTIONARY.
	DefaultHandle Handle
	XData         XData // extended data (1001…)
}

// DictionaryEntry is a name (code 3) and the object it refers to (350, or 360 if the dictionary owns it).
type DictionaryEntry struct {
	Name        string
	Handle      Handle
	IsHardOwner bool // the reference was code 360
}

// Lookup returns the handle the dictionary has under the name (case-insensitive), or 0.
func (d *Dictionary) Lookup(name string) Handle {
	for _, entry := range d.Entries {
		if strings.EqualFold(entry.Name, name) {
			return entry.Handle
		}
	}
	return 0
}

// XRecord is an XRECORD object: arbitrary data an application keeps in a dictionary.
type XRecord struct {
	Handle                 Handle
	OwnerHandle            Handle // code 330, usually a dictionary
	DuplicateRecordCloning int16  // code 280
	// Data are the record's code pairs as read.
	Data  []CodePair
	XData XData // extended data (1001…)
}

// DictionaryVariable is a DICTIONARYVAR object: the value of a setting that is kept in the AcDbVariableDictionary
// rather than in the header, e.g. XCLIPFRAME or DIMASSOC. See Drawing.DictionaryVariable.
type DictionaryVariable struct {
	Handle       Handle
	SchemaNumber int16  // code 280
	Value        string // code 1
	XData        XData  // extended data (1001…)
}

// Scale is a SCALE object: an annotation scale, e.g. "1:100" = 1 paper unit per 100 drawing units.
type Scale struct {
	Handle       Handle
	Name         string  // code 300
	PaperUnits   float64 // code 140
	DrawingUnits float64 // code 141
	IsUnitScale  bool    // code 290
	XData        XData   // extended data (1001…)
}

func parseDictionary(pairs []CodePair) (dictionary Dictionary) {
	dictionary.XData = xdataFromPairs(pairs)
	name := ""
	for _, pair := range pairs {
		switch pair.Code {
		case 5:
			dictionary.Handle = handleFromString(stringValue(pair))
		case 330:
			dictionary.OwnerHandle = handleFromString(stringValue(pair))
		case 280:
			dictionary.IsHardOwner = shortValue(pair) != 0
		case 281:
			dictionary.DuplicateRecordCloning = shortValue(pair)
		case 3:
			name = stringValue(pair)
		case 350, 360:
			dictionary.Entries = append(dictionary.Entries,
				DictionaryEntry{Name: name, Handle: handleFromString(stringValue(pair)), IsHardOwner: pair.Code == 360})
		case 340:
			dictionary.DefaultHandle = handleFromString(stringValue(pair))
		}
	}
	return
}

func parseXRecord(pairs []CodePair) (record XRecord) {
	record.XData = xdataFromPairs(pairs)
	for i, pair := range pairs {
		switch {
		case pair.Code == 5:
			record.Handle = handleFromString(stringValue(pair))
		case pair.Code == 330:
			record.OwnerHandle = handleFromString(stringValue(pair))
		case pair.Code == 100 && stringValue(pair) == "AcDbXrecord":
			// the data follows the cloning flag, and the extended data (if any) follows the data
			data := pairs[i+1:]
			if len(data) > 0 && data[0].Code == 280 {
				record.DuplicateRecordCloning = shortValue(data[0])
				data = data[1:]
			}
			for j, dataPair := range data {
				if dataPair.Code == 1001 {
					data = data[:j]
					break
				}
			}
			record.Data = append([]CodePair(nil), data...)
			return
		}
	}
	return
}

func parseDictionaryVariable(pairs []CodePair) (variable DictionaryVariable) {
	variable.XData = xdataFromPairs(pairs)
	for _, pair := range pairs {
		switch pair.Code {
		case 5:
			variable.Handle = handleFromString(stringValue(pair))
		case 280:
			variable.SchemaNumber = shortValue(pair)
		case 1:
			variable.Value = stringValue(pair)
		}
	}
	return
}

func parseScale(pairs []CodePair) (scale Scale) {
	scale.XData = xdataFromPairs(pairs)
	for _, pair := range pairs {
		switch pair.Code {
		case 5:
			scale.Handle = handleFromString(stringValue(pair))
		case 300:
			scale.Name = stringValue(pair)
		case 140:
			scale.PaperUnits = doubleValue(pair)
		case 141:
			scale.DrawingUnits = doubleValue(pair)
		case 290:
			scale.IsUnitScale = flagValue(pair)
		}
	}
	return
}

// NamedObjectDictionary returns the root dictionary of the OBJECTS section, or nil.
func (d *Drawing) NamedObjectDictionary() *Dictionary {
	for i := range d.Dictionaries {
		if d.Dictionaries[i].OwnerHandle == 0 {
			return &d.Dictionaries[i]
		}
	}
	return nil
}

// dictionaryByHandle returns the dictionary with the handle, or nil.
func (d *Drawing) dictionaryByHandle(h Handle) *Dictionary {
	for i := range d.Dictionaries {
		if h != 0 && d.Dictionaries[i].Handle == h {
			return &d.Dictionaries[i]
		}
	}
	return nil
}

// NamedObject follows names from the named object dictionary and returns the handle found, or 0: e.g.
// NamedObject("ACAD_MLEADERSTYLE", "Standard") is the handle of the "Standard" multileader style. Look the handle up
// with ItemByHandle.
func (d *Drawing) NamedObject(names ...string) Handle {
	dictionary := d.NamedObjectDictionary()
	var handle Handle
	for i, name := range names {
		if dictionary == nil {
			return 0
		}
		handle = dictionary.Lookup(name)
		if i < len(names)-1 {
			dictionary = d.dictionaryByHandle(handle)
		}
	}
	return handle
}

// ObjectName returns the name a dictionary gives the object with the handle, e.g. the name of a multileader style,
// table style, layout or scale; "" if no dictionary refers to it.
func (d *Drawing) ObjectName(h Handle) string {
	for i := range d.Dictionaries {
		for _, entry := range d.Dictionaries[i].Entries {
			if h != 0 && entry.Handle == h {
				return entry.Name
			}
		}
	}
	return ""
}

// DictionaryVariable returns the value of a setting kept in the AcDbVariableDictionary, e.g. "XCLIPFRAME".
func (d *Drawing) DictionaryVariable(name string) (value string, ok bool) {
	handle := d.NamedObject("AcDbVariableDictionary", name)
	for i := range d.DictionaryVariables {
		if handle != 0 && d.DictionaryVariables[i].Handle == handle {
			return d.DictionaryVariables[i].Value, true
		}
	}
	return "", false
}

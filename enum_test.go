package dxf

import (
	"encoding/xml"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestEnumString(t *testing.T) {
	assertEqString(t, "UnitsMeters", UnitsMeters.String())
	assertEqString(t, "UnitsMeters", fmt.Sprintf("%v", UnitsMeters))
	assertEqString(t, "AngleFormatSurveyorUnits", AngleFormatSurveyorUnits.String())
	assertEqString(t, "AttachmentPointBottomRight", AttachmentPointBottomRight.String())

	// values without a name used to recurse forever
	assertEqString(t, "Units(99)", Units(99).String())
	assertEqString(t, "AttachmentPoint(0)", fmt.Sprintf("%v", AttachmentPoint(0)))
}

func TestEnumStringCoversEveryValue(t *testing.T) {
	file, err := os.Open("spec/EnumSpec.xml")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	var spec struct {
		Enums []struct {
			Name   string `xml:"Name,attr"`
			Values []struct {
				Name  string `xml:"Name,attr"`
				Value string `xml:"Value,attr"`
			} `xml:"Value"`
		} `xml:"Enum"`
	}
	if err := xml.NewDecoder(file).Decode(&spec); err != nil {
		t.Fatal(err)
	}
	generated, err := os.ReadFile("enums.generated.go")
	if err != nil {
		t.Fatal(err)
	}

	for _, enum := range spec.Enums {
		// an empty value repeats the previous expression, as in Go; aliases of an earlier value need no case
		seen := map[string]bool{}
		expression := ""
		for i, value := range enum.Values {
			if value.Value != "" {
				expression = value.Value
			}
			resolved := expression
			if expression == "iota" {
				resolved = strconv.Itoa(i)
			}
			if seen[resolved] {
				continue
			}
			seen[resolved] = true
			name := enum.Name + value.Name
			if !strings.Contains(string(generated), fmt.Sprintf("\tcase %s:\n\t\treturn %q\n", name, name)) {
				t.Errorf("%s.String() has no case for %s", enum.Name, name)
			}
		}
	}
}

// Command dxfcheck reads drawings with this library and reports anything that looks parsed wrong or is missing.
//
//	go run github.com/kedazo/dxf-go/cmd/dxfcheck [-keep dir] file.dxf|file.dwg ...
//
// DWG files are converted with LibreDWG's dwg2dxf first, and the conversion is checked against the entity counts of
// dwgread's JSON output, because dwg2dxf sometimes drops entities. For every drawing it checks:
//   - every entity record of the BLOCKS and ENTITIES sections ends up in the drawing,
//   - every HATCH is written back with identical data,
//   - texts decode without replacement characters or leftover format codes,
//   - layouts, viewports, images and external references (resolved next to the file; DWG xrefs are converted too)
//     can be found,
//   - Explode reports no issues.
//
// The exit status is 1 if any problem was found.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	dxf "github.com/kedazo/dxf-go"
	"golang.org/x/text/encoding/htmlindex"
)

var (
	problems int
	// conversionDir receives the DXF files converted from DWG
	conversionDir string
)

func problem(format string, args ...interface{}) {
	problems++
	fmt.Printf("  !! "+format+"\n", args...)
}

func main() {
	// run returns instead of exiting so that the temporary directory is removed
	os.Exit(run())
}

func run() int {
	keep := flag.String("keep", "", "directory to keep converted DXF files in (default: a temporary directory)")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: dxfcheck [-keep dir] file.dxf|file.dwg ...")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() == 0 {
		flag.Usage()
		return 2
	}

	conversionDir = *keep
	if conversionDir == "" {
		dir, err := os.MkdirTemp("", "dxfcheck")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
		defer os.RemoveAll(dir)
		conversionDir = dir
	} else if err := os.MkdirAll(conversionDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	for i, path := range flag.Args() {
		fmt.Println("==", path)
		if strings.EqualFold(filepath.Ext(path), ".dwg") {
			converted := filepath.Join(conversionDir, fmt.Sprintf("%d-%s.dxf", i+1, strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))))
			if !convertDWG(path, converted) {
				continue
			}
			path = converted
		}
		check(path)
	}
	if problems > 0 {
		fmt.Printf("%d problem(s) found\n", problems)
		return 1
	}
	fmt.Println("no problems found")
	return 0
}

// convertDWG converts a DWG with dwg2dxf and compares the entity counts with dwgread's JSON output.
func convertDWG(path, converted string) bool {
	if _, err := exec.LookPath("dwg2dxf"); err != nil {
		problem("can't convert DWG: dwg2dxf (LibreDWG) is not installed")
		return false
	}
	if output, err := exec.Command("dwg2dxf", "-y", "-o", converted, path).CombinedOutput(); err != nil {
		problem("dwg2dxf failed: %v\n%s", err, output)
		return false
	}
	fmt.Println("  converted with dwg2dxf to", converted)

	dwgCounts, err := dwgEntityCounts(path, converted+".json")
	if err != nil {
		fmt.Println("  (entity counts of the DWG not checked:", err, ")")
		return true
	}
	dxfCounts := map[string]int{}
	for _, pair := range readRawPairs(converted) {
		if pair.code == 0 {
			dxfCounts[pair.value]++
		}
	}
	var lost []string
	for entityType, count := range dwgCounts {
		if dxfCounts[entityType] < count {
			lost = append(lost, fmt.Sprintf("%s %d of %d", entityType, count-dxfCounts[entityType], count))
		}
	}
	sort.Strings(lost)
	if len(lost) > 0 {
		problem("dwg2dxf dropped entities that are in the DWG: %s", strings.Join(lost, ", "))
	}
	return true
}

// dwgEntityCounts counts the entities of a DWG by type, as dwgread reads them.
func dwgEntityCounts(path, jsonPath string) (map[string]int, error) {
	if _, err := exec.LookPath("dwgread"); err != nil {
		return nil, fmt.Errorf("dwgread is not installed")
	}
	defer os.Remove(jsonPath)
	if output, err := exec.Command("dwgread", "-O", "JSON", "-o", jsonPath, path).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("dwgread failed: %v: %s", err, output)
	}
	file, err := os.Open(jsonPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var dump struct {
		Objects []struct {
			Entity string `json:"entity"`
		} `json:"OBJECTS"`
	}
	if err := json.NewDecoder(file).Decode(&dump); err != nil {
		return nil, err
	}
	counts := map[string]int{}
	for _, object := range dump.Objects {
		if object.Entity == "" {
			continue
		}
		entityType := object.Entity
		if strings.HasPrefix(entityType, "DIMENSION_") {
			entityType = "DIMENSION"
		}
		counts[entityType]++
	}
	return counts, nil
}

type rawPair struct {
	code  int
	value string
}

func readRawPairs(path string) (pairs []rawPair) {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1<<20), 1<<26)
	for scanner.Scan() {
		codeLine := strings.TrimSpace(scanner.Text())
		if !scanner.Scan() {
			break
		}
		code, err := strconv.Atoi(codeLine)
		if err != nil {
			// binary DXF or not a DXF: no raw comparison
			return nil
		}
		pairs = append(pairs, rawPair{code, strings.TrimSpace(strings.TrimRight(scanner.Text(), "\r"))})
	}
	return pairs
}

func check(path string) {
	start := time.Now()
	d, err := dxf.ReadFile(path)
	if err != nil {
		problem("can't read: %v", err)
		return
	}
	fmt.Printf("  %s, code page %s, units %v, read in %v\n", d.Header.Version, d.Header.DrawingCodePage, d.Header.DefaultDrawingUnits, time.Since(start).Round(time.Millisecond))
	for _, warning := range d.Warnings {
		problem("reader warning: %s", warning)
	}
	if unsupported := d.UnsupportedEntities(); len(unsupported) > 0 {
		fmt.Printf("  unsupported entity types (kept as UnknownEntity): %v\n", unsupported)
	}

	raw := readRawPairs(path)
	var all []dxf.Entity
	all = append(all, d.Entities...)
	for _, block := range d.Blocks {
		all = append(all, block.Entities...)
	}
	if raw != nil {
		checkRecordCounts(&d, raw)
		checkHatches(&d, raw)
	}
	checkTexts(&d, all)
	checkLayouts(&d, all, filepath.Dir(path))
	checkExplode(&d, filepath.Dir(path))
}

var typeNames = map[string]string{
	"Line": "LINE", "LWPolyline": "LWPOLYLINE", "Polyline": "POLYLINE", "Arc": "ARC", "Circle": "CIRCLE",
	"Ellipse": "ELLIPSE", "Hatch": "HATCH", "MText": "MTEXT", "Text": "TEXT", "Insert": "INSERT",
	"AttributeDefinition": "ATTDEF", "Wipeout": "WIPEOUT", "Image": "IMAGE", "Leader": "LEADER", "Solid": "SOLID",
	"Spline": "SPLINE", "ModelPoint": "POINT", "Viewport": "VIEWPORT", "Face": "3DFACE", "MLeader": "MULTILEADER",
	"Table": "ACAD_TABLE", "Mesh": "MESH", "ArcDimension": "ARC_DIMENSION", "Trace": "TRACE", "Ray": "RAY",
	"XLine": "XLINE", "Shape": "SHAPE", "Tolerance": "TOLERANCE", "Region": "REGION", "Body": "BODY",
	"Solid3D": "3DSOLID", "Helix": "HELIX", "Light": "LIGHT", "MLine": "MLINE", "OleFrame": "OLEFRAME",
	"Ole2Frame": "OLE2FRAME", "ProxyEntity": "ACAD_PROXY_ENTITY", "RText": "RTEXT", "Section": "SECTION",
	"ArcAlignedText": "ARCALIGNEDTEXT", "DgnUnderlay": "DGNUNDERLAY", "DwfUnderlay": "DWFUNDERLAY",
	"PdfUnderlay": "PDFUNDERLAY",
}

// checkRecordCounts compares the 0/<type> records of BLOCKS and ENTITIES with what the drawing holds.
func checkRecordCounts(d *dxf.Drawing, raw []rawPair) {
	rawCounts := map[string]int{}
	section := ""
	for i, pair := range raw {
		if pair.code == 0 && pair.value == "SECTION" && i+1 < len(raw) {
			section = raw[i+1].value
			continue
		}
		if pair.code == 0 && (section == "BLOCKS" || section == "ENTITIES") && pair.value != "ENDSEC" {
			rawCounts[pair.value]++
		}
	}

	counts := map[string]int{"BLOCK": len(d.Blocks), "ENDBLK": len(d.Blocks)}
	add := func(entities []dxf.Entity) {
		for _, e := range entities {
			name := strings.TrimPrefix(fmt.Sprintf("%T", e), "*dxf.")
			switch x := e.(type) {
			case *dxf.UnknownEntity:
				counts[x.Type]++
			case *dxf.Insert:
				counts["INSERT"]++
				counts["ATTRIB"] += len(x.Attributes)
				if len(x.Attributes) > 0 {
					counts["SEQEND"]++
				}
			case *dxf.Polyline:
				counts["POLYLINE"]++
				counts["VERTEX"] += len(x.Vertices)
				counts["SEQEND"]++
			case dxf.Dimension:
				if name == "ArcDimension" {
					counts["ARC_DIMENSION"]++
				} else {
					counts["DIMENSION"]++
				}
			default:
				if typeName, ok := typeNames[name]; ok {
					counts[typeName]++
				} else {
					counts[name]++
				}
			}
		}
	}
	add(d.Entities)
	for _, block := range d.Blocks {
		add(block.Entities)
	}

	var mismatches []string
	for entityType, count := range rawCounts {
		if counts[entityType] != count {
			mismatches = append(mismatches, fmt.Sprintf("%s: %d in the file, %d read", entityType, count, counts[entityType]))
		}
	}
	sort.Strings(mismatches)
	for _, mismatch := range mismatches {
		problem("record count differs, %s", mismatch)
	}
	total := 0
	for _, count := range rawCounts {
		total += count
	}
	fmt.Printf("  %d entity records in %d types, %d mismatches\n", total, len(rawCounts), len(mismatches))
}

// checkHatches compares every HATCH's data (after 100/AcDbHatch) in the file with what the library writes.
func checkHatches(d *dxf.Drawing, raw []rawPair) {
	written, err := d.CodePairs()
	if err != nil {
		problem("can't write the drawing: %v", err)
		return
	}
	var writtenPairs []rawPair
	for _, pair := range written {
		writtenPairs = append(writtenPairs, rawPair{pair.Code, valueString(pair.Value)})
	}
	decode := codePageDecoder(d.Header.DrawingCodePage)

	rawHatches, writtenHatches := hatchData(raw), hatchData(writtenPairs)
	if len(rawHatches) == 0 {
		return
	}
	differing := 0
	for i := range rawHatches {
		if i >= len(writtenHatches) || !samePairs(rawHatches[i], writtenHatches[i], decode) {
			differing++
		}
	}
	if differing > 0 {
		problem("%d of %d hatches are written back differently", differing, len(rawHatches))
	}
	badPaths := 0
	for _, e := range append(append([]dxf.Entity{}, d.Entities...), blockEntities(d)...) {
		if hatch, ok := e.(*dxf.Hatch); ok {
			for i := range hatch.Paths {
				if points, exact := hatch.Paths[i].Polygon(0); len(points) < 3 || !exact {
					badPaths++
				}
			}
		}
	}
	if badPaths > 0 {
		problem("%d hatch boundary paths can't be flattened exactly", badPaths)
	}
	fmt.Printf("  %d hatches, %d written back differently, %d bad boundary paths\n", len(rawHatches), differing, badPaths)
}

func blockEntities(d *dxf.Drawing) (entities []dxf.Entity) {
	for _, block := range d.Blocks {
		entities = append(entities, block.Entities...)
	}
	return
}

func hatchData(pairs []rawPair) (hatches [][]rawPair) {
	for i := 0; i < len(pairs); i++ {
		if pairs[i].code != 0 || pairs[i].value != "HATCH" {
			continue
		}
		j, start := i+1, -1
		for ; j < len(pairs) && pairs[j].code != 0; j++ {
			if pairs[j].code == 100 && pairs[j].value == "AcDbHatch" {
				start = j + 1
			}
		}
		if start >= 0 {
			hatches = append(hatches, pairs[start:j])
		}
		i = j - 1
	}
	return
}

func samePairs(a, b []rawPair, decode func(string) string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].code != b[i].code || !sameValue(decode(a[i].value), b[i].value) {
			return false
		}
	}
	return true
}

func sameValue(a, b string) bool {
	if a == b {
		return true
	}
	x, errA := strconv.ParseFloat(a, 64)
	y, errB := strconv.ParseFloat(b, 64)
	if errA == nil && errB == nil {
		return math.Abs(x-y) <= 1e-12*math.Max(1, math.Abs(x))
	}
	// handles are written without leading zeros and in either case
	return strings.EqualFold(strings.TrimLeft(a, "0"), strings.TrimLeft(b, "0"))
}

// codePageDecoder decodes raw pre-2007 strings ($DWGCODEPAGE ANSI_xxxx) to UTF-8; valid UTF-8 is kept.
func codePageDecoder(codePage string) func(string) string {
	var decoder func(string) (string, error)
	if number, ok := strings.CutPrefix(strings.ToUpper(codePage), "ANSI_"); ok {
		if encoding, err := htmlindex.Get("windows-" + number); err == nil {
			decoder = encoding.NewDecoder().String
		}
	}
	return func(s string) string {
		if decoder == nil || utf8.ValidString(s) {
			return s
		}
		if decoded, err := decoder(s); err == nil {
			return decoded
		}
		return s
	}
}

func valueString(value dxf.CodePairValue) string {
	switch v := value.(type) {
	case dxf.StringCodePairValue:
		return v.Value
	case dxf.DoubleCodePairValue:
		return strconv.FormatFloat(v.Value, 'g', -1, 64)
	case dxf.IntCodePairValue:
		return strconv.Itoa(v.Value)
	case dxf.ShortCodePairValue:
		return strconv.Itoa(int(v.Value))
	case dxf.LongCodePairValue:
		return strconv.FormatInt(v.Value, 10)
	case dxf.BoolCodePairValue:
		if v.Value {
			return "1"
		}
		return "0"
	}
	return fmt.Sprint(value)
}

// checkTexts looks for replacement characters and format codes left in plain text.
func checkTexts(d *dxf.Drawing, all []dxf.Entity) {
	checked, suspicious := 0, 0
	inspect := func(kind, text string) {
		checked++
		if strings.ContainsRune(text, utf8.RuneError) || strings.ContainsAny(text, "{}") || strings.Contains(text, "%%") ||
			strings.Contains(text, `\P`) || strings.Contains(text, `\U+`) {
			suspicious++
			if suspicious <= 5 {
				problem("suspicious %s text %q", kind, text)
			}
		}
	}
	for _, e := range all {
		switch x := e.(type) {
		case *dxf.MText:
			inspect("MTEXT", x.PlainText())
		case *dxf.Text:
			inspect("TEXT", x.PlainText())
		case *dxf.Insert:
			for _, attribute := range x.Attributes {
				inspect("ATTRIB", attribute.Value)
			}
		}
	}
	for _, layer := range d.Layers {
		inspect("layer name", layer.Name)
	}
	for _, block := range d.Blocks {
		inspect("block name", block.Name)
	}
	if suspicious > 5 {
		problem("... %d suspicious texts in total", suspicious)
	}
	fmt.Printf("  %d texts and names checked, %d suspicious\n", checked, suspicious)
}

// checkLayouts reports layouts and their viewports, images and external references.
func checkLayouts(d *dxf.Drawing, all []dxf.Entity, dir string) {
	for i := range d.Layouts {
		layout := &d.Layouts[i]
		if layout.IsModel() {
			continue
		}
		entities := d.LayoutEntities(layout)
		if len(entities) == 0 {
			continue
		}
		minimum, maximum, ok := layout.SheetRectangle()
		viewports := 0
		for _, e := range entities {
			if viewport, isViewport := e.(*dxf.Viewport); isViewport && viewport.IsOn() && !viewport.IsPaperSpaceView() {
				viewports++
			}
		}
		if ok {
			fmt.Printf("  layout %q: %.0f x %.0f sheet, %d entities, %d viewports onto model space\n", layout.Name, maximum.X-minimum.X, maximum.Y-minimum.Y, len(entities), viewports)
		} else {
			fmt.Printf("  layout %q: no paper size, %d entities, %d viewports onto model space\n", layout.Name, len(entities), viewports)
		}
	}
	for _, e := range all {
		if image, ok := e.(*dxf.Image); ok {
			definition := d.ImageDefinition(image)
			switch {
			case definition == nil:
				problem("IMAGE without an image definition")
			case !fileExists(dir, definition.FileName):
				problem("image file %q is missing", definition.FileName)
			default:
				fmt.Printf("  image %q found\n", definition.FileName)
			}
		}
	}
	for _, block := range d.Blocks {
		if block.IsXref() || block.XrefName != "" {
			fmt.Printf("  external reference %q -> %q\n", block.Name, block.XrefName)
		}
	}
}

func fileExists(dir, name string) bool {
	name = strings.ReplaceAll(name, `\`, "/")
	if !filepath.IsAbs(name) {
		name = filepath.Join(dir, name)
	}
	_, err := os.Stat(name)
	return err == nil
}

// xrefResolver resolves external references next to the drawing (garbled names included): DXF files are read as they
// are, DWG files are read from a DXF next to them or else converted with dwg2dxf (and checked) into the conversion
// directory.
func xrefResolver(dir string) func(block *dxf.Block) (*dxf.Drawing, error) {
	conversions := 0
	return dxf.XrefFileResolverWith(dir, dxf.XrefFileResolverOptions{ConvertDWG: func(dwgPath string) (string, error) {
		conversions++
		dxfPath := filepath.Join(conversionDir, fmt.Sprintf("xref%d-%s.dxf", conversions, strings.TrimSuffix(filepath.Base(dwgPath), filepath.Ext(dwgPath))))
		if !convertDWG(dwgPath, dxfPath) {
			return "", fmt.Errorf("dwg2dxf failed")
		}
		return dxfPath, nil
	}})
}

// checkExplode explodes the drawing, resolving external references next to it, and reports the issues.
func checkExplode(d *dxf.Drawing, dir string) {
	start := time.Now()
	result := d.Explode(dxf.ExplodeOptions{Recursive: true, IncludeDimensionBlocks: true, InheritProperties: true,
		ResolveXref: xrefResolver(dir)})
	counts := map[string]int{}
	for _, issue := range result.Issues {
		counts[issue.Kind.String()]++
	}
	shown := map[string]int{}
	for _, issue := range result.Issues {
		kind := issue.Kind.String()
		if shown[kind] < 3 {
			shown[kind]++
			problem("explode: %s", issue)
		}
	}
	for kind, count := range counts {
		if count > 3 {
			problem("explode: ... %d %s issues in total", count, kind)
		}
	}
	fmt.Printf("  exploded to %d entities in %v, %d issues\n", len(result.Entities), time.Since(start).Round(time.Millisecond), len(result.Issues))
}

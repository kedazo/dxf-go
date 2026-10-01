package dxf

import (
	"fmt"
	"hash/fnv"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

// XrefFileResolverOptions extends XrefFileResolverWith.
type XrefFileResolverOptions struct {
	// ConvertDWG converts a DWG file to DXF and returns the DXF file's path. It is used for DWG references that have
	// no DXF file next to them; without it such references can't be resolved. See DWG2DXF.
	ConvertDWG func(dwgPath string) (dxfPath string, err error)
	// Read reads a DXF file; ReadFile is used if it is nil. It allows e.g. a fallback encoding.
	Read func(path string) (*Drawing, error)
}

// FindXrefFile finds the file an external reference (Block.XrefName) points to. The reference is taken relative to
// dir unless it is absolute, and backslashes count as separators. For a DWG reference, a DXF file with the same name
// next to it is preferred (e.g. one converted with LibreDWG's dwg2dxf); the DWG's own path is returned if there is
// none, so the result ends in ".dwg" when the caller has to convert it.
//
// Files and folders whose names were garbled by extracting an archive with the wrong code page are found too: a name
// matches if it only differs in its non-ASCII characters (e.g. "Р-01 FЩLDSZINT" for "É-01 FÖLDSZINT"). A name
// matching more than one file is not found.
func FindXrefFile(dir, reference string) (string, error) {
	name := strings.ReplaceAll(reference, `\`, "/")
	if name == "" {
		return "", fmt.Errorf("empty external drawing path")
	}
	if !filepath.IsAbs(name) {
		name = filepath.Join(dir, name)
	}

	extension := filepath.Ext(name)
	if strings.EqualFold(extension, ".dwg") {
		base := strings.TrimSuffix(name, extension)
		for _, candidate := range []string{base + ".dxf", base + ".DXF"} {
			if path, ok := findFile(candidate); ok {
				return path, nil
			}
		}
	}
	if path, ok := findFile(name); ok {
		return path, nil
	}
	if strings.EqualFold(extension, ".dwg") {
		return "", fmt.Errorf("neither %s nor a DXF file next to it found", name)
	}
	return "", fmt.Errorf("%s not found", name)
}

// XrefFileResolver returns a WalkOptions.ResolveXref function that reads external references from the DXF files that
// FindXrefFile finds next to dir. DWG references without a DXF file next to them can't be resolved; see
// XrefFileResolverWith. Every file is read once.
func XrefFileResolver(dir string) func(block *Block) (*Drawing, error) {
	return XrefFileResolverWith(dir, XrefFileResolverOptions{})
}

// XrefFileResolverWith is XrefFileResolver with options, e.g. to convert DWG references on the fly or to read files
// differently.
func XrefFileResolverWith(dir string, options XrefFileResolverOptions) func(block *Block) (*Drawing, error) {
	readFile := options.Read
	if readFile == nil {
		readFile = func(path string) (*Drawing, error) {
			drawing, err := ReadFile(path)
			return &drawing, err
		}
	}
	drawings := map[string]*Drawing{}
	read := func(path string) (*Drawing, error) {
		if drawing, ok := drawings[path]; ok {
			return drawing, nil
		}
		drawing, err := readFile(path)
		if err != nil {
			return nil, err
		}
		drawings[path] = drawing
		return drawing, nil
	}

	return func(block *Block) (*Drawing, error) {
		path, err := FindXrefFile(dir, block.XrefName)
		if err != nil {
			return nil, err
		}
		if !strings.EqualFold(filepath.Ext(path), ".dwg") {
			return read(path)
		}

		if options.ConvertDWG == nil {
			return nil, fmt.Errorf("%s is a DWG file and there is no DXF file next to it", path)
		}
		if drawing, ok := drawings[path]; ok {
			return drawing, nil
		}
		dxfPath, err := options.ConvertDWG(path)
		if err != nil {
			return nil, fmt.Errorf("converting %s: %w", path, err)
		}
		drawing, err := read(dxfPath)
		if err == nil {
			drawings[path] = drawing
		}
		return drawing, err
	}
}

// DWG2DXF returns a converter for XrefFileResolverOptions.ConvertDWG that runs LibreDWG's dwg2dxf, which has to be
// installed, and writes the DXF files into outputDir. Note that dwg2dxf can silently drop entities; cmd/dxfcheck
// checks a conversion against the DWG.
func DWG2DXF(outputDir string) func(dwgPath string) (string, error) {
	return func(dwgPath string) (string, error) {
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			return "", err
		}
		// the hash keeps DWG files with the same name in different folders apart
		hash := fnv.New32a()
		hash.Write([]byte(dwgPath))
		name := strings.TrimSuffix(filepath.Base(dwgPath), filepath.Ext(dwgPath))
		dxfPath := filepath.Join(outputDir, fmt.Sprintf("%s-%08x.dxf", name, hash.Sum32()))
		if output, err := exec.Command("dwg2dxf", "-y", "-o", dxfPath, dwgPath).CombinedOutput(); err != nil {
			return "", fmt.Errorf("dwg2dxf: %v: %s", err, strings.TrimSpace(string(output)))
		}
		return dxfPath, nil
	}
}

// findFile returns path if it exists, otherwise the file whose path differs from it only in non-ASCII characters,
// folder by folder. ok is false if there is no such file or more than one.
func findFile(path string) (found string, ok bool) {
	if _, err := os.Stat(path); err == nil {
		return path, true
	}
	parent, name := filepath.Dir(path), filepath.Base(path)
	if parent == path {
		return "", false
	}
	if parent, ok = findFile(parent); !ok {
		return "", false
	}
	if candidate := filepath.Join(parent, name); fileExists(candidate) {
		return candidate, true
	}
	entries, err := os.ReadDir(parent)
	if err != nil {
		return "", false
	}
	var matches []string
	for _, entry := range entries {
		if garbledNameMatches(entry.Name(), name) {
			matches = append(matches, entry.Name())
		}
	}
	sort.Strings(matches)
	if len(matches) != 1 {
		return "", false
	}
	return filepath.Join(parent, matches[0]), true
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// garbledNameMatches reports whether two file names are the same apart from their non-ASCII characters, which an
// archive extracted with the wrong code page replaces one by one (bytes that aren't UTF-8 count as one character).
func garbledNameMatches(a, b string) bool {
	runesA, runesB := []rune(a), []rune(b)
	if len(runesA) != len(runesB) {
		return false
	}
	for i := range runesA {
		asciiA, asciiB := runesA[i] < 0x80, runesB[i] < 0x80
		switch {
		case asciiA && asciiB:
			if unicode.ToLower(runesA[i]) != unicode.ToLower(runesB[i]) {
				return false
			}
		case asciiA != asciiB:
			return false
		}
	}
	return true
}

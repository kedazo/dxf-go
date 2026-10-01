package dxf

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// XrefFileResolver returns a WalkOptions.ResolveXref function that reads external references from DXF files. An
// xref's path (Block.XrefName) is taken relative to dir unless it is absolute, and backslashes count as separators.
// If the path names a DWG file, which this package can't read, a DXF file with the same name is read instead (for
// example one converted with LibreDWG's dwg2dxf). Every file is read once.
func XrefFileResolver(dir string) func(block *Block) (*Drawing, error) {
	drawings := map[string]*Drawing{}
	return func(block *Block) (*Drawing, error) {
		name := strings.ReplaceAll(block.XrefName, `\`, "/")
		if name == "" {
			return nil, fmt.Errorf("block %q has no external drawing path", block.Name)
		}
		if !filepath.IsAbs(name) {
			name = filepath.Join(dir, name)
		}
		candidates := []string{name}
		if extension := filepath.Ext(name); strings.EqualFold(extension, ".dwg") {
			base := strings.TrimSuffix(name, extension)
			candidates = []string{base + ".dxf", base + ".DXF"}
		}

		for _, path := range candidates {
			if drawing, ok := drawings[path]; ok {
				return drawing, nil
			}
			if _, err := os.Stat(path); err != nil {
				continue
			}
			drawing, err := ReadFile(path)
			if err != nil {
				return nil, err
			}
			drawings[path] = &drawing
			return &drawing, nil
		}
		return nil, fmt.Errorf("%s not found", strings.Join(candidates, " or "))
	}
}

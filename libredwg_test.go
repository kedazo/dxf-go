package dxf

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fakeDWG2DXF writes a shell script that behaves like dwg2dxf -y -o output input: it logs its arguments, checks
// that the input exists and writes a small DXF file as the output.
func fakeDWG2DXF(t *testing.T, dir, name string) (program, logPath string) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake dwg2dxf is a shell script")
	}
	template := filepath.Join(dir, "template.dxf")
	writeXrefFile(t, template)
	logPath = filepath.Join(dir, "arguments.log")
	program = filepath.Join(dir, name)
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$@\" > '" + logPath + "'\n" +
		"test -f \"$4\" || exit 3\n" +
		"cp '" + template + "' \"$3\"\n"
	if err := os.WriteFile(program, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return
}

func TestFindLibreDWGTool(t *testing.T) {
	bundled := t.TempDir()
	program, _ := fakeDWG2DXF(t, bundled, "fake-dwg2dxf")
	t.Setenv("PATH", t.TempDir())

	// not on PATH, but next to the program
	found, err := findTool("fake-dwg2dxf", []string{t.TempDir(), bundled})
	if err != nil {
		t.Fatal(err)
	}
	assertEqString(t, program, found)

	// PATH wins
	onPath := t.TempDir()
	pathProgram, _ := fakeDWG2DXF(t, onPath, "fake-dwg2dxf")
	t.Setenv("PATH", onPath)
	found, err = findTool("fake-dwg2dxf", []string{bundled})
	if err != nil {
		t.Fatal(err)
	}
	assertEqString(t, pathProgram, found)

	_, err = findTool("missing-dwg2dxf", []string{bundled})
	assert(t, err != nil, "expected an error for a missing program")
}

func TestDWG2DXFWithUsesASCIINames(t *testing.T) {
	tools := t.TempDir()
	program, logPath := fakeDWG2DXF(t, tools, "dwg2dxf")

	// a garbled DWG name in a folder with accents
	source := filepath.Join(t.TempDir(), "Kovács")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	dwgPath := filepath.Join(source, "Р-01  FЩLDSZINTI.dwg")
	if err := os.WriteFile(dwgPath, []byte("DWG"), 0o644); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "Átalakított")

	dxfPath, err := DWG2DXFWith(program, output)(dwgPath)
	if err != nil {
		t.Fatal(err)
	}
	assertEqString(t, output, filepath.Dir(dxfPath))
	assert(t, isASCIIString(filepath.Base(dxfPath)), "expected an ASCII DXF name, got "+dxfPath)
	drawing, err := ReadFile(dxfPath)
	if err != nil {
		t.Fatal(err)
	}
	assertEqInt(t, 4, len(drawing.Entities))

	logged, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	assert(t, isASCIIString(string(logged)), "dwg2dxf got non-ASCII arguments: "+string(logged))
	assert(t, !strings.Contains(string(logged), "/"), "dwg2dxf got paths instead of names: "+string(logged))

	// the ASCII copy of the DWG is removed again
	entries, _ := os.ReadDir(output)
	assertEqInt(t, 1, len(entries))

	// an ASCII path is passed as it is
	plain := filepath.Join(t.TempDir(), "plan.dwg")
	if err := os.WriteFile(plain, []byte("DWG"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := DWG2DXFWith(program, t.TempDir())(plain); err != nil {
		t.Fatal(err)
	}
	logged, _ = os.ReadFile(logPath)
	assertContains(t, plain, string(logged))
}

func TestDWG2DXFWithReportsFailures(t *testing.T) {
	tools := t.TempDir()
	program, _ := fakeDWG2DXF(t, tools, "dwg2dxf")
	// the input doesn't exist, so the fake exits with an error
	_, err := DWG2DXFWith(program, t.TempDir())(filepath.Join(t.TempDir(), "missing.dwg"))
	assert(t, err != nil, "expected the conversion to fail")

	t.Setenv("PATH", t.TempDir())
	_, err = DWG2DXFWith(filepath.Join(tools, "not-there"), t.TempDir())(filepath.Join(tools, "template.dxf"))
	assert(t, err != nil, "expected a missing program to fail")
}

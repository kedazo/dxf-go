package dxf

import (
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// FindLibreDWGTool returns the path of a LibreDWG program such as "dwg2dxf" or "dwgread": the one on PATH, or else one
// next to the running program (where installers often bundle it, e.g. tools\dwg2dxf.exe next to tools\app.exe).
// A program that is only in the current directory is not used, like exec.LookPath.
func FindLibreDWGTool(name string) (string, error) {
	executableDirs := []string{}
	if executable, err := os.Executable(); err == nil {
		executableDirs = append(executableDirs, filepath.Dir(executable))
		if resolved, err := filepath.EvalSymlinks(executable); err == nil && filepath.Dir(resolved) != filepath.Dir(executable) {
			executableDirs = append(executableDirs, filepath.Dir(resolved))
		}
	}
	return findTool(name, executableDirs)
}

func findTool(name string, dirs []string) (string, error) {
	if path, err := exec.LookPath(name); err == nil {
		return path, nil
	}
	for _, dir := range dirs {
		for _, candidate := range []string{name, name + ".exe"} {
			path := filepath.Join(dir, candidate)
			if info, err := os.Stat(path); err == nil && !info.IsDir() {
				return path, nil
			}
		}
	}
	return "", fmt.Errorf("%s not found on PATH or next to the program", name)
}

// DWG2DXFWith returns a converter for XrefFileResolverOptions.ConvertDWG that runs the given dwg2dxf executable (an
// empty path finds it with FindLibreDWGTool) and writes the DXF files into outputDir. Note that dwg2dxf can silently
// drop entities; cmd/dxfcheck checks a conversion against the DWG.
//
// dwg2dxf only ever gets ASCII file names: Windows builds read their arguments in the system code page, which can't
// hold every name (e.g. names garbled by an archive). A DWG with a non-ASCII path is copied into outputDir under an
// ASCII name first, and the DXF file is named in ASCII too.
func DWG2DXFWith(executable, outputDir string) func(dwgPath string) (string, error) {
	return func(dwgPath string) (string, error) {
		program := executable
		if program == "" {
			found, err := FindLibreDWGTool("dwg2dxf")
			if err != nil {
				return "", err
			}
			program = found
		}
		if absolute, err := filepath.Abs(program); err == nil {
			program = absolute
		}
		if absolute, err := filepath.Abs(dwgPath); err == nil {
			dwgPath = absolute
		}
		if err := os.MkdirAll(outputDir, 0o755); err != nil {
			return "", err
		}

		// the hash keeps DWG files with the same name in different folders apart
		hash := fnv.New32a()
		hash.Write([]byte(dwgPath))
		name := fmt.Sprintf("%s-%08x", asciiFileName(strings.TrimSuffix(filepath.Base(dwgPath), filepath.Ext(dwgPath))), hash.Sum32())

		input := dwgPath
		if !isASCIIString(dwgPath) {
			input = name + ".dwg"
			if err := copyFile(dwgPath, filepath.Join(outputDir, input)); err != nil {
				return "", err
			}
			defer os.Remove(filepath.Join(outputDir, input))
		}
		output := name + ".dxf"

		// relative names inside outputDir keep its path (e.g. a user folder with accents) off the command line
		command := exec.Command(program, "-y", "-o", output, input)
		command.Dir = outputDir
		if result, err := command.CombinedOutput(); err != nil {
			return "", fmt.Errorf("%s: %v: %s", filepath.Base(program), err, strings.TrimSpace(string(result)))
		}
		return filepath.Join(outputDir, output), nil
	}
}

func isASCIIString(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

// asciiFileName replaces the non-ASCII characters of a file name with underscores.
func asciiFileName(name string) string {
	var builder strings.Builder
	for _, r := range name {
		if r < 0x80 {
			builder.WriteRune(r)
		} else {
			builder.WriteByte('_')
		}
	}
	return builder.String()
}

func copyFile(from, to string) error {
	source, err := os.Open(from)
	if err != nil {
		return err
	}
	defer source.Close()
	target, err := os.Create(to)
	if err != nil {
		return err
	}
	if _, err := io.Copy(target, source); err != nil {
		target.Close()
		return err
	}
	return target.Close()
}

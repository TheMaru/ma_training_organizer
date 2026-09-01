// Package archtest walks the module's Go files so a package can state a boundary
// as a test. Both boundaries this repo checks (docs/agents/analysis.md) parse
// every file outside the directories that are allowed across the line; only what
// they then look for differs, and that is what a caller supplies.
package archtest

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// File is one parsed Go file, with the path spelled the way a failure message
// should name it: relative to the module root, not to wherever the test ran.
type File struct {
	Rel    string
	Syntax *ast.File
	Fset   *token.FileSet
}

// EachFileOutside parses every .go file in the module that does not sit directly
// in one of the allowed directories, and hands it to check.
//
// Directly in: a subdirectory of an allowed one is on the outside of the line.
// Both callers rely on that — storetest, sessiontest and trainertest are fixtures
// other suites import, so they have no more business crossing a boundary than any
// other caller. trainertest is the case that shows it: it sits under
// internal/trainer, an allowed directory, and is checked anyway.
//
// The allowed paths are module-root-relative, e.g. "internal/session".
func EachFileOutside(t *testing.T, allowed []string, mode parser.Mode, check func(File)) {
	t.Helper()
	root := moduleRoot(t)
	skip := make(map[string]bool, len(allowed))
	for _, dir := range allowed {
		skip[filepath.Join(root, filepath.FromSlash(dir))] = true
	}

	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir():
			if d.Name() == ".git" {
				return fs.SkipDir
			}
			return nil
		case !strings.HasSuffix(d.Name(), ".go"), skip[filepath.Dir(path)]:
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, mode)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		check(File{Rel: rel, Syntax: f, Fset: fset})
		return nil
	})
	if err != nil {
		t.Fatalf("walk the module: %v", err)
	}
}

// ImportName is the name f refers to the package at path by, or empty when it
// does not import it at all. A check about calls needs it because an aliased
// import would otherwise slip past.
func ImportName(f *ast.File, path string) string {
	for _, imp := range f.Imports {
		if strings.Trim(imp.Path.Value, `"`) != path {
			continue
		}
		if imp.Name != nil {
			return imp.Name.Name
		}
		return path[strings.LastIndex(path, "/")+1:]
	}
	return ""
}

// moduleRoot is the directory holding go.mod, found by walking up rather than by
// counting "../.." from the caller — so a boundary test does not break by moving
// one directory deeper.
func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("locate the working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("no go.mod above %s", dir)
		}
		dir = parent
	}
}

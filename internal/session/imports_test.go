package session_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// scsPath is the import path this module hides. Both scs packages sit under it,
// so the prefix is what has to stay unimported elsewhere.
const scsPath = "github.com/alexedwards/scs/"

// The module's claim is that it is the only place scs is known, and a claim about
// which packages import something is checkable rather than merely true: a handler
// or a subcommand that reaches for a session store directly fails here, in the
// package whose whole point it was to make that unnecessary.
func TestSessionIsTheOnlyPackageThatImportsSCS(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("locate the module root: %v", err)
	}
	// The module's own directory is the one place the import belongs. Its test
	// files are excluded with it: a test of this package is part of it.
	allowed := filepath.Join(root, "internal", "session")

	fset := token.NewFileSet()
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir():
			if d.Name() == ".git" || path == allowed {
				return fs.SkipDir
			}
			return nil
		case !strings.HasSuffix(d.Name(), ".go"):
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range f.Imports {
			if strings.HasPrefix(strings.Trim(imp.Path.Value, `"`), scsPath) {
				rel, _ := filepath.Rel(root, path)
				t.Errorf("%s imports %s — ask internal/session instead", rel, imp.Path.Value)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk the module: %v", err)
	}
}

package trainer_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// storePath is the package these acts are built out of. The import name is read
// off each file rather than assumed, so an aliased import is caught too.
const storePath = "github.com/TheMaru/ma_training_organizer/internal/store"

// guarded are the store operations that make up an act on a Trainer account, and
// the count the refusals are made of.
//
// store.UpdateTrainerPassword is deliberately absent: the self-service change in
// the account area is the Trainer's own act, with its own rules, and it stays in
// internal/web.
var guarded = map[string]bool{
	"CreateTrainer":       true,
	"DeactivateTrainer":   true,
	"ReactivateTrainer":   true,
	"DeleteTrainer":       true,
	"CountActiveTrainers": true,
}

// The module's claim is that an act on a Trainer account happens here and nowhere
// else — test helpers included, since that is where the second definition of
// Deactivated came from last time (see this package's doc comment for why one
// definition is the point). A claim about which packages call something is
// checkable rather than merely true, so it is checked: otherwise the next helper
// reaches for the store again and nothing says so.
func TestTrainerIsTheOnlyPackageThatActsOnAnAccount(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("locate the module root: %v", err)
	}
	// This module owns the acts, and internal/store owns the operations they are
	// made of — its own tests exercise its API and are part of it. Exactly those two
	// directories, not their subtrees: internal/store/storetest is a fixture other
	// suites import, so it is on the outside of this line like any other caller.
	allowed := map[string]bool{
		filepath.Join(root, "internal", "trainer"): true,
		filepath.Join(root, "internal", "store"):   true,
	}

	fset := token.NewFileSet()
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir():
			if d.Name() == ".git" {
				return fs.SkipDir
			}
			return nil
		case !strings.HasSuffix(d.Name(), ".go"), allowed[filepath.Dir(path)]:
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		name := storeImportName(f)
		if name == "" {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		ast.Inspect(f, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok || !guarded[sel.Sel.Name] {
				return true
			}
			if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == name {
				t.Errorf("%s:%d calls %s.%s — ask internal/trainer for the act instead",
					rel, fset.Position(sel.Pos()).Line, name, sel.Sel.Name)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walk the module: %v", err)
	}
}

// storeImportName is the name the file refers to the store package by, or empty
// when it does not import it at all.
func storeImportName(f *ast.File) string {
	for _, imp := range f.Imports {
		if strings.Trim(imp.Path.Value, `"`) != storePath {
			continue
		}
		if imp.Name != nil {
			return imp.Name.Name
		}
		return "store"
	}
	return ""
}

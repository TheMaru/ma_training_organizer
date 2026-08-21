package trainer_test

import (
	"go/ast"
	"testing"

	"github.com/TheMaru/ma_training_organizer/internal/archtest"
)

// storePath is the package these acts are built out of.
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
//
// This module owns the acts, and internal/store owns the operations they are made
// of — its own tests exercise its API and are part of it. It walks the syntax tree
// rather than the import block because this is a claim about calls.
func TestTrainerIsTheOnlyPackageThatActsOnAnAccount(t *testing.T) {
	allowed := []string{"internal/trainer", "internal/store"}
	archtest.EachFileOutside(t, allowed, 0, func(f archtest.File) {
		name := archtest.ImportName(f.Syntax, storePath)
		if name == "" {
			return
		}
		ast.Inspect(f.Syntax, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok || !guarded[sel.Sel.Name] {
				return true
			}
			if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == name {
				t.Errorf("%s:%d calls %s.%s — ask internal/trainer for the act instead",
					f.Rel, f.Fset.Position(sel.Pos()).Line, name, sel.Sel.Name)
			}
			return true
		})
	})
}

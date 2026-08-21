package session_test

import (
	"go/parser"
	"strings"
	"testing"

	"github.com/TheMaru/ma_training_organizer/internal/archtest"
)

// scsPath is the import path this module hides. Both scs packages sit under it,
// so the prefix is what has to stay unimported elsewhere.
const scsPath = "github.com/alexedwards/scs/"

// The module's claim is that it is the only place scs is known, and a claim about
// which packages import something is checkable rather than merely true: a handler
// or a subcommand that reaches for a session store directly fails here, in the
// package whose whole point it was to make that unnecessary.
//
// The module's own directory is the one place the import belongs, its test files
// included: a test of this package is part of it.
func TestSessionIsTheOnlyPackageThatImportsSCS(t *testing.T) {
	archtest.EachFileOutside(t, []string{"internal/session"}, parser.ImportsOnly, func(f archtest.File) {
		for _, imp := range f.Syntax.Imports {
			if strings.HasPrefix(strings.Trim(imp.Path.Value, `"`), scsPath) {
				t.Errorf("%s imports %s — ask internal/session instead", f.Rel, imp.Path.Value)
			}
		}
	})
}

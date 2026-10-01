package rankview

import (
	"go/ast"
	"go/token"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/TheMaru/ma_training_organizer/internal/archtest"
)

var hexColour = regexp.MustCompile(`#[0-9a-fA-F]{6}\b`)

var catalogKeyPrefixes = []string{colourKeyPrefix, degreeKeyPrefix, rankSplitKey, systemKeyPrefix}

// The module's claim is that it is the one place a martial art's conventions live
// (ADR-0004): the belt colours and the catalog pieces a rank's name is composed of.
// A second copy of either has happened before — ADR-0009 records the belt test that
// kept its own colour list — so the claim is checked rather than trusted.
//
// It is a claim about literals, so it walks the string literals and not the
// comments: internal/i18n explains a degree key in a comment, which is no copy.
// Coarser than a check on imports or calls, it can be wrong: a Go file with a
// legitimate hex in it has to join the allowed list, and whoever adds it judges
// whether that is a real exception or the boundary being worked around.
func TestRankviewIsTheOnlyPackageThatSpellsABeltOrARankKey(t *testing.T) {
	archtest.EachFileOutside(t, []string{"internal/rankview"}, 0, func(f archtest.File) {
		ast.Inspect(f.Syntax, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			value, err := strconv.Unquote(lit.Value)
			if err != nil {
				t.Fatalf("%s: unquote %s: %v", f.Rel, lit.Value, err)
			}
			if spells(value) {
				t.Errorf("%s:%d spells %s — ask internal/rankview instead",
					f.Rel, f.Fset.Position(lit.Pos()).Line, lit.Value)
			}
			return true
		})
	})
}

func spells(value string) bool {
	if hexColour.MatchString(value) {
		return true
	}
	for _, prefix := range catalogKeyPrefixes {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}

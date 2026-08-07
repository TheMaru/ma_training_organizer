package web

// Unit and fuzz tests for returnPath. In-package for the same reason belt_test.go
// is: the check is unexported, and the property worth pinning is about the string
// it returns rather than about any page. That the language switcher actually
// applies it is tested over HTTP in i18n_test.go.

import (
	"strings"
	"testing"
)

// returnPathCases doubles as the fuzz seed corpus, so a shape worth naming here
// is also a starting point the fuzzer mutates from.
var returnPathCases = map[string]string{
	"/athletes":                        "/athletes",
	"/athletes?sort=lastName&dir=desc": "/athletes?sort=lastName&dir=desc",
	"https://evil.example/":            homePath,
	"//evil.example/":                  homePath,
	`/\evil.example/`:                  homePath,
	"mailto:someone@evil.example":      homePath,
	"athletes":                         homePath,
	"?sort=lastName":                   homePath,
	"":                                 homePath,
	"/athletes\r\nX: y":                homePath,
	"/\t/evil.example":                 homePath,
	"/athletes\x7f":                    homePath,
	"/athletes\x00":                    homePath,

	// Percent-encoded it would stay encoded in the header, so a browser reads it
	// as a path on this site. Rejected anyway: whether a given client strips before
	// or after decoding is not a thing to bet a redirect target on, and no
	// legitimate target carries a control character in either spelling.
	"/%09/evil.example": homePath,
}

func TestReturnPathKeepsOnlyBarePaths(t *testing.T) {
	for p, want := range returnPathCases {
		if got := returnPath(p); got != want {
			t.Errorf("returnPath(%q) = %q, want %q", p, got, want)
		}
	}
}

// asBrowserSees strips the characters a browser drops before it parses a URL —
// see returnPath for why that step is the whole difficulty.
var asBrowserSees = strings.NewReplacer("\t", "", "\n", "", "\r", "").Replace

// FuzzReturnPath asserts the docstring's claim the way a browser would test it:
// an accepted target stays on this site even after that stripping. A property,
// not a pattern, so the test can disagree with the implementation rather than
// restate it — which it did, finding "/\t/" against the prefix check this
// replaced.
func FuzzReturnPath(f *testing.F) {
	for p := range returnPathCases {
		f.Add(p)
	}

	f.Fuzz(func(t *testing.T, p string) {
		got := returnPath(p)
		if got == homePath {
			return
		}
		seen := asBrowserSees(got)
		switch {
		case !strings.HasPrefix(seen, "/"),
			strings.HasPrefix(seen, "//"),
			strings.HasPrefix(seen, `/\`):
			t.Errorf("returnPath(%q) = %q, which a browser reads as %q — off-site", p, got, seen)
		}
	})
}

package web

// Unit tests for the belt renderer. This is the one in-package test file in
// internal/web: the colour table and the parser are deliberately unexported
// view-layer detail (ADR-0004), and testing the geometry through rendered pages
// would say much less about it. Which surface draws a belt, and how it is
// labelled, is tested black-box like everything else — roster_test.go for the
// roster, promotions_test.go for the two athlete-detail surfaces.

import (
	"strings"
	"testing"
)

// countStripes counts the stripe rects in a rendered belt.
func countStripes(svg string) int {
	return strings.Count(svg, `class="belt-stripe"`)
}

func TestBeltSVGRendersAPlainBelt(t *testing.T) {
	svg := string(beltSVG("Blue", 0, "Blue (BJJ Adult)"))
	if svg == "" {
		t.Fatal("Blue rendered nothing, want a belt")
	}
	if !strings.Contains(svg, `class="belt-body" x="0" y="0" width="64" height="16" fill="`+beltColours["Blue"]) {
		t.Errorf("belt = %q, want a body rect in the blue fill", svg)
	}
	// A plain belt has no split bar and, at degree 0, no stripes.
	if strings.Contains(svg, "belt-bar") {
		t.Errorf("belt = %q, want no split bar on a plain belt", svg)
	}
	if got := countStripes(svg); got != 0 {
		t.Errorf("stripes = %d, want 0 at degree 0", got)
	}
	// The black friso is always there — it is what the stripes sit on.
	if !strings.Contains(svg, "belt-friso") {
		t.Errorf("belt = %q, want a friso", svg)
	}
}

func TestBeltSVGRendersSplitBelts(t *testing.T) {
	cases := []struct {
		group    string
		wantBody string
		wantBar  string
	}{
		{"Grey-White", beltColours["Grey"], beltColours["White"]},
		{"Yellow-Black", beltColours["Yellow"], beltColours["Black"]},
	}
	for _, c := range cases {
		svg := string(beltSVG(c.group, 0, c.group))
		if !strings.Contains(svg, `class="belt-body" x="0" y="0" width="64" height="16" fill="`+c.wantBody) {
			t.Errorf("%s = %q, want body fill %s", c.group, svg, c.wantBody)
		}
		if !strings.Contains(svg, `class="belt-bar"`) || !strings.Contains(svg, `class="belt-bar" x="0" y="6" width="64" height="4" fill="`+c.wantBar) {
			t.Errorf("%s = %q, want a bar in %s", c.group, svg, c.wantBar)
		}
	}
}

func TestBeltSVGDrawsOneStripePerDegree(t *testing.T) {
	for degree := 0; degree <= beltMaxStripes; degree++ {
		if got := countStripes(string(beltSVG("Black", degree, "x"))); got != degree {
			t.Errorf("degree %d drew %d stripes, want %d", degree, got, degree)
		}
	}
	// A split belt with stripes composes both features.
	svg := string(beltSVG("Grey-White", 3, "x"))
	if !strings.Contains(svg, "belt-bar") || countStripes(svg) != 3 {
		t.Errorf("Grey-White degree 3 = %q, want a bar and 3 stripes", svg)
	}
}

func TestBeltSVGStripesStayInsideTheFriso(t *testing.T) {
	// A degree beyond what the friso holds draws the maximum rather than spilling
	// out of the viewBox: the rank name still carries the true count.
	if got := countStripes(string(beltSVG("Black", beltMaxStripes+3, "x"))); got != beltMaxStripes {
		t.Errorf("stripes at degree %d = %d, want %d", beltMaxStripes+3, got, beltMaxStripes)
	}
	// A negative degree is not a rank the seed produces, but it must not produce
	// negative geometry either.
	if got := countStripes(string(beltSVG("Black", -2, "x"))); got != 0 {
		t.Errorf("stripes at a negative degree = %d, want 0", got)
	}
}

func TestBeltSVGFallsBackToNothingForUnknownGroups(t *testing.T) {
	// The caller renders the plain rank name whenever this is empty (ADR-0004), so
	// "no entry in the table" must never yield a half-drawn belt.
	for _, group := range []string{
		"",                 // ungraded athlete
		"Cyan",             // a colour nobody mapped
		"Cyan-White",       // known bar, unknown body
		"Blue-Green",       // second part is neither White nor Black
		"Grey-White-Black", // not a shape the parsing rule allows
		"7. Dan",           // a non-belt grading system
	} {
		if got := beltSVG(group, 2, "x"); got != "" {
			t.Errorf("beltSVG(%q) = %q, want empty", group, got)
		}
	}
}

func TestBeltSVGMatchesTheSeededSpellingOnly(t *testing.T) {
	// The lookup is exact: rank_group is only ever written by store.Seed, so a
	// variant spelling means an unmapped colour, which is the text fallback rather
	// than a guess. This pins that choice — loosening it is a decision, not a fix.
	for _, group := range []string{"blue", "BLUE", " Blue "} {
		if got := beltSVG(group, 0, "x"); got != "" {
			t.Errorf("beltSVG(%q) = %q, want empty — only the seeded spelling resolves", group, got)
		}
	}
}

func TestBeltSVGLabelsTheGraphicWhenItStandsAlone(t *testing.T) {
	// The roster shows the graphic only, so the rank name is its accessible name
	// and its tooltip (ADR-0001: the text always stays reachable).
	svg := string(beltSVG("Blue", 1, "Blue, 1 stripe (BJJ Adult)"))
	if !strings.Contains(svg, `role="img"`) {
		t.Errorf("belt = %q, want role=img", svg)
	}
	if !strings.Contains(svg, `aria-label="Blue, 1 stripe (BJJ Adult)"`) {
		t.Errorf("belt = %q, want the rank name as aria-label", svg)
	}
	if !strings.Contains(svg, `<title>Blue, 1 stripe (BJJ Adult)</title>`) {
		t.Errorf("belt = %q, want the rank name as a title (tooltip)", svg)
	}
	if strings.Contains(svg, "aria-hidden") {
		t.Errorf("belt = %q, want a labelled graphic exposed, not hidden", svg)
	}
}

func TestBeltSVGIsDecorativeWithoutALabel(t *testing.T) {
	// The detail surfaces print the rank name next to the graphic, so announcing it
	// again would be duplicate content.
	svg := string(beltSVG("Blue", 1, ""))
	if !strings.Contains(svg, `aria-hidden="true"`) {
		t.Errorf("belt = %q, want aria-hidden when unlabelled", svg)
	}
	if strings.Contains(svg, "aria-label") || strings.Contains(svg, "<title>") {
		t.Errorf("belt = %q, want no accessible name when unlabelled", svg)
	}
}

func TestBeltSVGEscapesItsLabel(t *testing.T) {
	// The label is rank + system name, both free text out of the database: it must
	// not be able to close the attribute or open a tag.
	svg := string(beltSVG("Blue", 0, `Blue" <script>x</script>`))
	if strings.Contains(svg, "<script>") {
		t.Errorf("belt = %q, want the label's markup escaped", svg)
	}
	if strings.Contains(svg, `aria-label="Blue"`) {
		t.Errorf("belt = %q, want the label's quote escaped rather than closing the attribute", svg)
	}
}

func TestBeltSVGCarriesTheStylingHook(t *testing.T) {
	// app.css sizes the graphic off this class, and the intrinsic width/height keep
	// it sane if the stylesheet never arrives.
	svg := string(beltSVG("Blue", 0, "x"))
	for _, want := range []string{`class="belt"`, `viewBox="0 0 64 16"`, `width="64"`, `height="16"`} {
		if !strings.Contains(svg, want) {
			t.Errorf("belt = %q, want %s", svg, want)
		}
	}
}

func TestBeltSVGCoversEverySeededColour(t *testing.T) {
	// The seeded systems' belts (store.seedSystems) must all render — a missing
	// entry would silently downgrade a real rank to text.
	for _, group := range []string{
		"White", "Grey-White", "Grey", "Grey-Black",
		"Yellow-White", "Yellow", "Yellow-Black",
		"Orange-White", "Orange", "Orange-Black",
		"Green-White", "Green", "Green-Black",
		"Blue", "Purple", "Brown", "Black",
	} {
		if beltSVG(group, 0, "x") == "" {
			t.Errorf("seeded group %q renders no belt, want one", group)
		}
	}
}

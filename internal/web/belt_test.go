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
	if !strings.Contains(svg, `class="belt-body" x="0" y="0" width="72" height="16" fill="`+beltColours["Blue"]) {
		t.Errorf("belt = %q, want a body rect in the blue fill", svg)
	}
	// A plain belt has no split bar and, at degree 0, no stripes.
	if strings.Contains(svg, "belt-bar") {
		t.Errorf("belt = %q, want no split bar on a plain belt", svg)
	}
	if got := countStripes(svg); got != 0 {
		t.Errorf("stripes = %d, want 0 at degree 0", got)
	}
	// The friso is always there — it is what the stripes sit on.
	if !strings.Contains(svg, `class="belt-friso" x="42" y="0" width="20" height="16" fill="`+beltBlack) {
		t.Errorf("belt = %q, want a black friso at x=42", svg)
	}
}

func TestBeltSVGLeavesATailPastTheFriso(t *testing.T) {
	// On a real belt the friso sits short of the end, so body colour runs on past
	// it. That tail is what stops the graphic looking like a belt cut in half.
	if tail := beltWidth - (beltFrisoX + beltFrisoWidth); tail <= 0 {
		t.Fatalf("friso ends at %d of %d — no tail left", beltFrisoX+beltFrisoWidth, beltWidth)
	}
	// A split belt's bar is drawn across the full width and only hidden where the
	// friso covers it, so the bar reappears in the tail for free.
	svg := string(beltSVG("Grey-White", 0, "x"))
	if !strings.Contains(svg, `class="belt-bar" x="0" y="6" width="72" height="4"`) {
		t.Errorf("belt = %q, want the bar spanning the full width so it shows in the tail", svg)
	}
	// The friso must be drawn after the bar, or it would not cover it at all.
	if strings.Index(svg, "belt-bar") > strings.Index(svg, "belt-friso") {
		t.Errorf("belt = %q, want the bar drawn before the friso", svg)
	}
}

func TestBeltSVGGivesTheBlackBeltARedFriso(t *testing.T) {
	// BJJ convention, and the only thing that makes the friso visible against a
	// black body at all.
	svg := string(beltSVG("Black", 0, "x"))
	if !strings.Contains(svg, `class="belt-friso" x="42" y="0" width="20" height="16" fill="`+beltRed) {
		t.Errorf("black belt = %q, want a red friso", svg)
	}
	// Only the friso's colour changes: the degree still shows, as white stripes on
	// the red, so Black and "Black, 4 stripes" stay tellable apart on the roster
	// where the graphic is all there is.
	if strings.Contains(svg, "belt-stripe") {
		t.Errorf("black belt at degree 0 = %q, want no stripes", svg)
	}
	for _, degree := range []int{1, 2, beltMaxStripes} {
		graded := string(beltSVG("Black", degree, "x"))
		if got := countStripes(graded); got != degree {
			t.Errorf("black belt at degree %d drew %d stripes, want %d", degree, got, degree)
		}
		// The x depends on how many stripes are centred, so only the fill is pinned.
		if !strings.Contains(graded, `width="2" height="12" fill="`+beltWhite+`"`) {
			t.Errorf("black belt at degree %d = %q, want white stripes", degree, graded)
		}
	}
	// A belt whose *bar* is black is not a black belt — it keeps the black friso.
	if !strings.Contains(string(beltSVG("Yellow-Black", 0, "x")), `class="belt-friso" x="42" y="0" width="20" height="16" fill="`+beltBlack) {
		t.Error("Yellow-Black should keep a black friso — only a black body turns it red")
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
		if !strings.Contains(svg, `class="belt-body" x="0" y="0" width="72" height="16" fill="`+c.wantBody) {
			t.Errorf("%s = %q, want body fill %s", c.group, svg, c.wantBody)
		}
		if !strings.Contains(svg, `class="belt-bar"`) || !strings.Contains(svg, `class="belt-bar" x="0" y="6" width="72" height="4" fill="`+c.wantBar) {
			t.Errorf("%s = %q, want a bar in %s", c.group, svg, c.wantBar)
		}
	}
}

func TestBeltSVGDrawsOneStripePerDegree(t *testing.T) {
	for degree := 0; degree <= beltMaxStripes; degree++ {
		if got := countStripes(string(beltSVG("Blue", degree, "x"))); got != degree {
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
	if got := countStripes(string(beltSVG("Blue", beltMaxStripes+3, "x"))); got != beltMaxStripes {
		t.Errorf("stripes at degree %d = %d, want %d", beltMaxStripes+3, got, beltMaxStripes)
	}
	// A negative degree is not a rank the seed produces, but it must not produce
	// negative geometry either.
	if got := countStripes(string(beltSVG("Blue", -2, "x"))); got != 0 {
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
	for _, want := range []string{`class="belt"`, `viewBox="0 0 72 16"`, `width="72"`, `height="16"`} {
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

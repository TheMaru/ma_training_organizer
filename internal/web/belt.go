package web

import (
	"fmt"
	"html"
	"html/template"
	"slices"
	"strings"
)

// beltColours maps a rank's `rank_group` onto the fill that draws it. This is
// the hardcoded, view-layer colour table of ADR-0004: the domain model keeps
// rank_group as free text and declares nothing about belts, so a rank becomes
// belt-renderable purely by having an entry here. Extending a belt system means
// adding colours; a grading system with none (numbered dan grades, say) has no
// entries and renders as its plain rank name instead.
//
// The keys are spelled exactly as store.Seed writes them, which is the only
// thing that writes rank_group today. A variant spelling simply misses and falls
// back to the rank name — matching more loosely would be guessing at input no
// code path produces.
var beltColours = map[string]string{
	"White":  "#f2f2f2", // off-white, so the belt still reads on a white surface
	"Grey":   "#9ca3af",
	"Yellow": "#facc15",
	"Orange": "#f97316",
	"Green":  "#16a34a",
	"Blue":   "#2563eb",
	"Purple": "#7e22ce",
	"Brown":  "#78350f",
	"Black":  "#1c1c1f",
}

// beltBarColours are the only colours a split belt's longitudinal bar may be:
// the kids systems split a body colour with white or black ("Grey-White",
// "Yellow-Black"). Anything else in that position is not a split belt at all.
var beltBarColours = []string{"White", "Black"}

// Belt geometry, in the user space of the rendered viewBox. The belt is a flat
// strip: the body across its whole width, an optional bar down its middle, and a
// black friso (end block) at the right carrying the stripes.
const (
	beltWidth        = 64
	beltHeight       = 16
	beltFrisoX       = 44 // where the friso starts; it runs to the right edge
	beltBarY         = 6
	beltBarHeight    = 4
	beltStripeWidth  = 2
	beltStripeGap    = 2
	beltStripeInset  = 2 // vertical inset of a stripe within the friso
	beltStripeMargin = 3 // horizontal breathing room at each end of the friso

	// beltMaxStripes is how many stripes the friso has room for — four, which is
	// exactly the range the seeded systems grade across. A higher degree draws the
	// maximum rather than spilling out of the viewBox: the rank name always carries
	// the true count, so the graphic clips instead of misleading.
	beltMaxStripes = (beltWidth - beltFrisoX - 2*beltStripeMargin + beltStripeGap) /
		(beltStripeWidth + beltStripeGap)
)

// belt is a rank's resolved visual description, everything the renderer needs.
type belt struct {
	body    string // hex fill of the belt body
	bar     string // hex fill of the split-belt bar, empty on a plain belt
	stripes int    // white stripes on the friso
}

// resolveBelt parses a rank_group and degree into a drawable belt, reporting
// false when the colour is not in beltColours. The caller then falls back to the
// plain rank name (ADR-0004), so the graphic is never the sole carrier of
// meaning and an unmapped system degrades gracefully instead of breaking.
//
// The parsing rule (spec): split rank_group on "-"; two parts whose second is
// White or Black are a body colour plus a bar, and otherwise the whole string is
// a single body colour — so a colour name that itself contains a "-" is looked up
// intact rather than mistaken for a split belt.
func resolveBelt(group string, degree int) (belt, bool) {
	body, bar := group, ""
	if before, after, split := strings.Cut(group, "-"); split && slices.Contains(beltBarColours, after) {
		body, bar = before, beltColours[after]
	}
	fill, ok := beltColours[body]
	if !ok {
		return belt{}, false
	}
	// The degree is clamped rather than trusted: seed.go leaves the rare 5th BJJ
	// stripe out but says a club may add it, and that rank would otherwise draw
	// stripes outside the viewBox.
	return belt{body: fill, bar: bar, stripes: min(max(degree, 0), beltMaxStripes)}, true
}

// beltSVG renders a rank as an inline SVG belt, the one helper behind all three
// rank surfaces (roster, detail current rank, promotion history) so their markup
// cannot drift apart. It returns an empty string when the rank's colour is
// unknown; templates treat that as "render the plain rank name instead".
//
// label is the rank as assistive tech should hear it. Pass it where the graphic
// stands alone (the roster) and it becomes the graphic's accessible name and its
// tooltip; pass an empty string where the rank name is printed right next to the
// belt (the detail surfaces) and the graphic is marked decorative, so the name is
// not announced twice.
//
// Every colour and coordinate comes from the table above or from arithmetic on
// the constants, so the only untrusted text in the output is label — escaped.
func beltSVG(group string, degree int, label string) template.HTML {
	b, ok := resolveBelt(group, degree)
	if !ok {
		return ""
	}

	var svg strings.Builder
	fmt.Fprintf(&svg,
		`<svg class="belt" viewBox="0 0 %d %d" width="%d" height="%d" xmlns="http://www.w3.org/2000/svg"`,
		beltWidth, beltHeight, beltWidth, beltHeight)
	if label == "" {
		svg.WriteString(` aria-hidden="true">`)
	} else {
		escaped := html.EscapeString(label)
		// role=img keeps assistive tech from walking into the rects; the <title>
		// doubles as the hover tooltip.
		fmt.Fprintf(&svg, ` role="img" aria-label="%s"><title>%s</title>`, escaped, escaped)
	}

	fmt.Fprintf(&svg, `<rect class="belt-body" x="0" y="0" width="%d" height="%d" fill="%s"/>`,
		beltWidth, beltHeight, b.body)
	if b.bar != "" {
		// Drawn before the friso, which covers its end — a real split belt's bar runs
		// the belt's length and disappears under the friso.
		fmt.Fprintf(&svg, `<rect class="belt-bar" x="0" y="%d" width="%d" height="%d" fill="%s"/>`,
			beltBarY, beltWidth, beltBarHeight, b.bar)
	}
	fmt.Fprintf(&svg, `<rect class="belt-friso" x="%d" y="0" width="%d" height="%d" fill="%s"/>`,
		beltFrisoX, beltWidth-beltFrisoX, beltHeight, beltColours["Black"])
	writeBeltStripes(&svg, b.stripes)
	// A hairline in the current text colour, so the two belts that come within a
	// shade of the page keep a silhouette: white on the light theme, black on the
	// dark one. currentColor flips with the theme, so one rule covers both.
	fmt.Fprintf(&svg,
		`<rect class="belt-edge" x="0.5" y="0.5" width="%d" height="%d" fill="none" stroke="currentColor" stroke-opacity="0.5"/>`,
		beltWidth-1, beltHeight-1)
	svg.WriteString(`</svg>`)

	return template.HTML(svg.String())
}

// writeBeltStripes draws n white stripes centred on the friso, evenly spaced.
func writeBeltStripes(svg *strings.Builder, n int) {
	if n == 0 {
		return
	}
	span := n*beltStripeWidth + (n-1)*beltStripeGap
	x := beltFrisoX + (beltWidth-beltFrisoX-span)/2
	for i := range n {
		fmt.Fprintf(svg, `<rect class="belt-stripe" x="%d" y="%d" width="%d" height="%d" fill="%s"/>`,
			x+i*(beltStripeWidth+beltStripeGap), beltStripeInset,
			beltStripeWidth, beltHeight-2*beltStripeInset, beltColours["White"])
	}
}

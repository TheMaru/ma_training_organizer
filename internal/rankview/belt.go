package rankview

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
// belt-renderable purely by having an entry here.
//
// The keys are spelled exactly as the store's seed writes them (see
// store.SeededRankGroups), which is the only thing that writes rank_group today. Lookup is exact — matching more loosely
// would be guessing at input no code path produces.
var beltColours = map[string]string{
	"White":  beltWhite,
	"Grey":   "#9ca3af",
	"Yellow": "#facc15",
	"Orange": "#f97316",
	"Green":  "#16a34a",
	"Blue":   "#2563eb",
	"Purple": "#7e22ce",
	"Brown":  "#78350f",
	"Black":  beltBlack,
}

// The three fills the renderer needs by name rather than by rank_group.
const (
	beltWhite = "#f2f2f2" // off-white, so the belt still reads on a white surface
	beltBlack = "#1c1c1f"
	beltRed   = "#b91c1c"
)

// beltBarColours are the only colours a split belt's longitudinal bar may be:
// the kids systems split a body colour with white or black ("Grey-White",
// "Yellow-Black"). Anything else in that position is not a split belt at all.
var beltBarColours = []string{"White", "Black"}

// Belt geometry, in the user space of the rendered viewBox. The belt is a flat
// strip: the body across its whole width, an optional bar down its middle, and
// the friso (end block) carrying the stripes. The friso sits a little short of
// the right edge, as it does on a real belt, so a stretch of body colour runs on
// past it.
const (
	beltWidth        = 72
	beltHeight       = 16
	beltFrisoX       = 38 // where the friso starts
	beltFrisoWidth   = 24 // it grows leftwards, so the tail past it stays put
	beltBarY         = 6
	beltBarHeight    = 4
	beltStripeWidth  = 2
	beltStripeGap    = 2
	beltStripeInset  = 2 // vertical inset of a stripe within the friso
	beltStripeMargin = 3 // horizontal breathing room at each end of the friso

	// beltMaxStripes is how many stripes the friso has room for — five, one more
	// than the seeded systems grade across, which covers the rare 5th BJJ stripe
	// seed.go leaves out but expects a club to add. A higher degree still draws the
	// maximum rather than spilling out of the friso: the rank name always carries
	// the true count, so the graphic clips instead of misleading.
	beltMaxStripes = (beltFrisoWidth - 2*beltStripeMargin + beltStripeGap) /
		(beltStripeWidth + beltStripeGap)
)

// belt is a rank's resolved visual description, everything the renderer needs.
type belt struct {
	body    string // hex fill of the belt body
	bar     string // hex fill of the split-belt bar, empty on a plain belt
	friso   string // hex fill of the end block
	stripes int    // white stripes on the friso
}

// splitRankGroup parses a rank_group into the colour *names* it is made of: a
// body colour and, on a split belt, the bar's. It reports false for a colour that
// has no entry in beltColours, which is what both the graphic and the composed
// label (ADR-0009) treat as "unresolvable" — so the two degrade together on one
// definition of a known colour rather than on two lists that could drift.
//
// Only a second part that is White or Black makes a split belt, so a colour name
// that itself contains a "-" is looked up intact rather than mistaken for one.
func splitRankGroup(group string) (body, bar string, ok bool) {
	body, bar = group, ""
	if before, after, split := strings.Cut(group, "-"); split && slices.Contains(beltBarColours, after) {
		body, bar = before, after
	}
	if _, known := beltColours[body]; !known {
		return "", "", false
	}
	return body, bar, true
}

func resolveBelt(group string, degree int) (belt, bool) {
	body, bar, ok := splitRankGroup(group)
	if !ok {
		return belt{}, false
	}
	drawn := belt{
		body:    beltColours[body],
		bar:     beltColours[bar],
		friso:   beltBlack,
		stripes: min(max(degree, 0), beltMaxStripes),
	}
	if body == "Black" {
		// A BJJ black belt's friso is red, not black — which is also the only thing
		// that makes the end block visible against the body at all. Its degree
		// stripes stay white and are drawn like any other belt's.
		drawn.friso = beltRed
	}
	return drawn, true
}

// beltSVG renders a rank as an inline SVG belt, the one renderer behind every
// surface that draws one, so their markup cannot drift apart. It returns an empty
// string for a rank resolveBelt cannot draw, and the caller writes the rank name
// instead — the graphic is never the sole carrier of meaning, so a grading system
// with no colours degrades gracefully rather than breaking (ADR-0004).
//
// label is the rank as assistive tech should hear it. Pass it where the graphic
// stands alone (the roster) and it becomes the graphic's accessible name and its
// tooltip; pass an empty string where the rank name is printed right next to the
// belt and the graphic is marked decorative, so the name is not announced twice.
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
		// Drawn before the friso, which hides the stretch of bar it covers — a real
		// split belt's bar runs the belt's whole length, so it disappears under the
		// friso and reappears in the tail past it.
		fmt.Fprintf(&svg, `<rect class="belt-bar" x="0" y="%d" width="%d" height="%d" fill="%s"/>`,
			beltBarY, beltWidth, beltBarHeight, b.bar)
	}
	fmt.Fprintf(&svg, `<rect class="belt-friso" x="%d" y="0" width="%d" height="%d" fill="%s"/>`,
		beltFrisoX, beltFrisoWidth, beltHeight, b.friso)
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

func writeBeltStripes(svg *strings.Builder, n int) {
	if n == 0 {
		return
	}
	span := n*beltStripeWidth + (n-1)*beltStripeGap
	x := beltFrisoX + (beltFrisoWidth-span)/2
	for i := range n {
		fmt.Fprintf(svg, `<rect class="belt-stripe" x="%d" y="%d" width="%d" height="%d" fill="%s"/>`,
			x+i*(beltStripeWidth+beltStripeGap), beltStripeInset,
			beltStripeWidth, beltHeight-2*beltStripeInset, beltWhite)
	}
}

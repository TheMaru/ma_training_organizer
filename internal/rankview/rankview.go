// Package rankview is how a rank is shown: the belt graphic (ADR-0004) and the
// name composed from catalog pieces (ADR-0009). It is the one place the
// conventions of a martial art live — belt colours, stripes, the split belt — so
// that the store keeps rank_group as free text and declares nothing about belts.
//
// Each exported function is one rendering, and each takes the one value it
// renders. What surrounds a rank on a page — a date, a table cell, an optgroup —
// stays in the template.
package rankview

import (
	"html"
	"html/template"

	"github.com/TheMaru/ma_training_organizer/internal/i18n"
	"github.com/TheMaru/ma_training_organizer/internal/store"
)

// RosterRank is empty for no rank at all.
func RosterRank(locale i18n.Locale, rank store.Rank) template.HTML {
	name, system := rosterNames(locale, rank)
	if belt := beltSVG(rank.Group, rank.Degree, rosterLabel(name, system)); belt != "" {
		return belt
	}
	text := html.EscapeString(name)
	if system != "" {
		text += ` <span class="muted">(` + html.EscapeString(system) + `)</span>`
	}
	return template.HTML(text)
}

// RankWithBelt marks its belt decorative because the surface prints the name anyway.
func RankWithBelt(locale i18n.Locale, rank store.Rank) template.HTML {
	name := html.EscapeString(RankName(locale, rank))
	if belt := beltSVG(rank.Group, rank.Degree, ""); belt != "" {
		return belt + template.HTML(" "+name)
	}
	return template.HTML(name)
}

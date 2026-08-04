package web

import (
	"strconv"
	"strings"

	"github.com/TheMaru/ma_training_organizer/internal/i18n"
)

// Catalog keys for the pieces a rank name is composed of (ADR-0009): a colour
// word per belt colour, one pattern per stripe degree, and the pattern joining a
// split belt's two colours. The colour key is derived from the rank_group the
// seed wrote, lowercased.
//
// rankSplitKey is a pattern rather than a separator so a language may reorder the
// two colours and choose its own joining character. The "-" splitRankGroup splits
// on is unrelated — that one belongs to the stored data's format.
const (
	colourKeyPrefix = "rank.colour."
	degreeKeyPrefix = "rank.degree."
	rankSplitKey    = "rank.split"

	// systemKeyPrefix takes the grading system's slug, which ADR-0006 made its
	// stable identity — the reason the name is free to be translated at all.
	systemKeyPrefix = "system."
)

// rankLabel names a rank in the trainer's language, composed from the colour
// group and the stripe degree rather than looked up whole (ADR-0009). stored is
// the rank's name as the seed wrote it, which is what anything the catalog cannot
// spell renders — see splitRankGroup for why an unresolvable colour is the same
// case for the label as it is for the graphic.
func rankLabel(locale i18n.Locale, group string, degree int, stored string) string {
	body, bar, ok := splitRankGroup(group)
	if !ok {
		return stored
	}
	colour, ok := colourPhrase(locale, body, bar)
	if !ok {
		return stored
	}
	if degree == 0 {
		return colour
	}
	name, ok := i18n.Lookup(locale, degreeKeyPrefix+strconv.Itoa(degree), colour)
	if !ok {
		return stored
	}
	return name
}

// systemLabel names a grading system in the trainer's language. stored is the
// name the database holds, which a system the catalog has no key for keeps — a
// club's own system stays usable rather than turning invisible (ADR-0009).
func systemLabel(locale i18n.Locale, slug, stored string) string {
	if slug == "" {
		return stored
	}
	if name, ok := i18n.Lookup(locale, systemKeyPrefix+slug); ok {
		return name
	}
	return stored
}

// rosterRankLabel is the rank as assistive tech and a tooltip get it on the
// roster, where the belt graphic stands alone (ADR-0004): the rank plus the system
// that disambiguates same-named ranks across cohorts — White exists in both the
// kids and the adult system. Empty for an ungraded athlete, who has no rank.
//
// It takes the whole row because of that conditional, which is Go's job rather
// than the template's.
func rosterRankLabel(locale i18n.Locale, line rosterLine) string {
	rank := rankLabel(locale, line.Group, line.Degree, line.RankName)
	if line.SystemName == "" {
		return rank
	}
	return rank + " (" + systemLabel(locale, line.SystemSlug, line.SystemName) + ")"
}

// colourPhrase names a belt's colour: one word on a plain belt, both words joined
// by the split pattern on a split one. bar is empty on a plain belt.
func colourPhrase(locale i18n.Locale, body, bar string) (string, bool) {
	word, ok := colourWord(locale, body)
	if !ok || bar == "" {
		return word, ok
	}
	barWord, ok := colourWord(locale, bar)
	if !ok {
		return "", false
	}
	return i18n.Lookup(locale, rankSplitKey, word, barWord)
}

func colourWord(locale i18n.Locale, colour string) (string, bool) {
	return i18n.Lookup(locale, colourKeyPrefix+strings.ToLower(colour))
}

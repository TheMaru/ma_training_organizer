package rankview

import (
	"strconv"
	"strings"

	"github.com/TheMaru/ma_training_organizer/internal/i18n"
	"github.com/TheMaru/ma_training_organizer/internal/store"
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

// RankName names a rank in the trainer's language, composed from the colour group
// and the stripe degree rather than looked up whole (ADR-0009). Anything the
// catalog cannot spell renders rank.Name, the stored English name — see
// splitRankGroup for why an unresolvable colour is the same case for the name as
// it is for the graphic.
//
// The two names differ on purpose: rank.Name is stored and English, RankName is
// what a trainer reads.
func RankName(locale i18n.Locale, rank store.Rank) string {
	body, bar, ok := splitRankGroup(rank.Group)
	if !ok {
		return rank.Name
	}
	colour, ok := colourPhrase(locale, body, bar)
	if !ok {
		return rank.Name
	}
	if rank.Degree == 0 {
		return colour
	}
	name, ok := i18n.Lookup(locale, degreeKeyPrefix+strconv.Itoa(rank.Degree), colour)
	if !ok {
		return rank.Name
	}
	return name
}

// SystemName names a grading system in the trainer's language. A system the
// catalog has no key for keeps system.Name, the name the database holds — a club's
// own system stays usable rather than turning invisible (ADR-0009).
func SystemName(locale i18n.Locale, system store.System) string {
	if system.Slug == "" {
		return system.Name
	}
	if name, ok := i18n.Lookup(locale, systemKeyPrefix+system.Slug); ok {
		return name
	}
	return system.Name
}

// rosterLabel is the rank as assistive tech and a tooltip get it on the roster,
// where the belt graphic stands alone (ADR-0004): the rank plus the system that
// disambiguates same-named ranks across cohorts — White exists in both the kids
// and the adult system. Empty for no rank at all.
func rosterLabel(locale i18n.Locale, rank store.Rank) string {
	name, system := rosterNames(locale, rank)
	if system == "" {
		return name
	}
	return name + " (" + system + ")"
}

// rosterNames is the rank and system the roster names, for both the label and the
// visible fallback, so the screen reader and the screen cannot disagree. system is
// empty when there is none to give: only a held rank gives an athlete a system, by
// the same test store.RosterRow.Ungraded applies.
func rosterNames(locale i18n.Locale, rank store.Rank) (name, system string) {
	name = RankName(locale, rank)
	if rank.IsZero() {
		return name, ""
	}
	return name, SystemName(locale, rank.System)
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

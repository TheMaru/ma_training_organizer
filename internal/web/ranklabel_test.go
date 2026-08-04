package web

// Unit tests for the composed rank and system labels (ADR-0009). In-package for
// the same reason belt_test.go is: the composition is unexported view-layer
// detail. This seam covers only exhaustiveness and the fallbacks — that the nine
// surfaces actually show the composed label is tested through rendered pages in
// i18n_domain_test.go, since driving the full colour × degree matrix over HTTP
// would mean one athlete per rank.

import (
	"testing"

	"github.com/TheMaru/ma_training_organizer/internal/i18n"
	"github.com/TheMaru/ma_training_organizer/internal/store"
)

// storedName is what every fallback path renders, spelled so a test can tell a
// composed label from a fallback at a glance.
const storedName = "STORED"

func TestRankLabelIsTheColourWordAloneAtDegreeZero(t *testing.T) {
	// Mirrors seed.rankName: a plain belt's name is its colour, not "Blue, 0 stripes".
	for locale, want := range map[i18n.Locale]string{
		i18n.German:  "Blau",
		i18n.English: "Blue",
	} {
		if got := rankLabel(locale, "Blue", 0, storedName); got != want {
			t.Errorf("%s rankLabel(Blue, 0) = %q, want %q", locale, got, want)
		}
	}
}

func TestRankLabelPhrasesEveryStripeDegree(t *testing.T) {
	// Degrees are enumerated in the catalog rather than interpolated, so one and
	// several stripes each read correctly (ADR-0009). The English column is the
	// spelling seed.rankName writes, which is what the stored name would have said.
	cases := []struct {
		degree  int
		german  string
		english string
	}{
		{1, "Blau, 1 Streifen", "Blue, 1 stripe"},
		{2, "Blau, 2 Streifen", "Blue, 2 stripes"},
		{3, "Blau, 3 Streifen", "Blue, 3 stripes"},
		{4, "Blau, 4 Streifen", "Blue, 4 stripes"},
		// No seeded rank uses five, but the graphic draws it and clubs may add it.
		{5, "Blau, 5 Streifen", "Blue, 5 stripes"},
	}
	for _, c := range cases {
		if got := rankLabel(i18n.German, "Blue", c.degree, storedName); got != c.german {
			t.Errorf("German rankLabel(Blue, %d) = %q, want %q", c.degree, got, c.german)
		}
		if got := rankLabel(i18n.English, "Blue", c.degree, storedName); got != c.english {
			t.Errorf("English rankLabel(Blue, %d) = %q, want %q", c.degree, got, c.english)
		}
	}
}

func TestRankLabelComposesSplitBelts(t *testing.T) {
	// Both colour words are translated and joined by the catalog's pattern, so a
	// split belt reads naturally rather than as a half-translated hybrid. The bar
	// is the seeded system's kids split, only ever white or black.
	cases := []struct {
		group   string
		degree  int
		german  string
		english string
	}{
		{"Grey-White", 0, "Grau-Weiß", "Grey-White"},
		{"Grey-White", 2, "Grau-Weiß, 2 Streifen", "Grey-White, 2 stripes"},
		{"Yellow-Black", 3, "Gelb-Schwarz, 3 Streifen", "Yellow-Black, 3 stripes"},
	}
	for _, c := range cases {
		if got := rankLabel(i18n.German, c.group, c.degree, storedName); got != c.german {
			t.Errorf("German rankLabel(%s, %d) = %q, want %q", c.group, c.degree, got, c.german)
		}
		if got := rankLabel(i18n.English, c.group, c.degree, storedName); got != c.english {
			t.Errorf("English rankLabel(%s, %d) = %q, want %q", c.group, c.degree, got, c.english)
		}
	}
}

func TestRankLabelFallsBackToTheStoredName(t *testing.T) {
	// One rule for everything the composition cannot spell: render what the seed
	// stored (ADR-0009). A catalog key would be worse than English as an athlete's
	// rank, and the belt graphic falls back on the very same cases.
	cases := []struct {
		group  string
		degree int
		why    string
	}{
		{"Cyan", 2, "a colour with no word"},
		{"Cyan-White", 2, "a split belt whose body has no word"},
		{"", 7, "a rank predating the descriptive columns"},
		{"Blue", 6, "a degree past the enumeration"},
		{"Blue", -1, "a degree that is not a count at all"},
		{"blue", 0, "a variant spelling the seed never writes"},
	}
	for _, c := range cases {
		for _, locale := range []i18n.Locale{i18n.German, i18n.English} {
			if got := rankLabel(locale, c.group, c.degree, storedName); got != storedName {
				t.Errorf("%s rankLabel(%q, %d) = %q, want the stored name (%s)", locale, c.group, c.degree, got, c.why)
			}
		}
	}
}

// TestRankLabelCoversEverySeededColour is the analogue of
// TestBeltSVGCoversEverySeededColour: a forgotten colour word degrades a real rank
// to English silently, so every group the seed writes must compose in both
// locales, across every degree the catalog enumerates.
func TestRankLabelCoversEverySeededColour(t *testing.T) {
	for _, group := range store.SeededRankGroups() {
		for degree := 0; degree <= beltMaxStripes; degree++ {
			for _, locale := range []i18n.Locale{i18n.German, i18n.English} {
				if got := rankLabel(locale, group, degree, storedName); got == storedName {
					t.Errorf("%s rankLabel(%q, %d) fell back to the stored name, want a composed label", locale, group, degree)
				}
			}
		}
	}
}

func TestSystemLabelIsKeyedOnTheSlug(t *testing.T) {
	// A system's name is a display label and its slug the identity (ADR-0006), so
	// the translation hangs off the slug.
	if got := systemLabel(i18n.German, "bjj-kids", storedName); got != "BJJ Kinder" {
		t.Errorf("German systemLabel(bjj-kids) = %q, want %q", got, "BJJ Kinder")
	}
	if got := systemLabel(i18n.English, "bjj-kids", storedName); got != "BJJ Kids" {
		t.Errorf("English systemLabel(bjj-kids) = %q, want %q", got, "BJJ Kids")
	}
	// A club's own system, and one predating the slug column: both keep the name
	// the database holds, so an untranslated system stays usable.
	for _, slug := range []string{"bjj-elderly", ""} {
		if got := systemLabel(i18n.German, slug, storedName); got != storedName {
			t.Errorf("systemLabel(%q) = %q, want the stored name", slug, got)
		}
	}
}

func TestRosterRankLabelNamesRankAndSystemTogether(t *testing.T) {
	// On the roster the belt graphic stands alone, so its accessible name carries
	// both the rank and the system that tells a Kids white belt from an Adult one.
	graded := rosterLine{RosterRow: store.RosterRow{
		RankName:   "Grey-White, 2 stripes",
		SystemName: "BJJ Kids",
		SystemSlug: "bjj-kids",
		Group:      "Grey-White",
		Degree:     2,
	}}
	if got := rosterRankLabel(i18n.German, graded); got != "Grau-Weiß, 2 Streifen (BJJ Kinder)" {
		t.Errorf("German roster label = %q, want %q", got, "Grau-Weiß, 2 Streifen (BJJ Kinder)")
	}
	if got := rosterRankLabel(i18n.English, graded); got != "Grey-White, 2 stripes (BJJ Kids)" {
		t.Errorf("English roster label = %q, want %q", got, "Grey-White, 2 stripes (BJJ Kids)")
	}

	// An ungraded athlete has no rank and no system, so there is nothing to name.
	if got := rosterRankLabel(i18n.German, rosterLine{}); got != "" {
		t.Errorf("ungraded roster label = %q, want it empty", got)
	}
}

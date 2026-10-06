package rankview

// Unit tests for the composed rank and system names (ADR-0009). In-package so
// they reach rosterLabel and beltMaxStripes, which are unexported. This seam covers
// exhaustiveness and the fallbacks — that the pages actually show the composed
// name is tested through rendered pages in internal/web's i18n_domain_test.go,
// since driving the full colour × degree matrix over HTTP would mean one athlete
// per rank.

import (
	"testing"

	"github.com/TheMaru/ma_training_organizer/internal/i18n"
	"github.com/TheMaru/ma_training_organizer/internal/store"
)

// storedName is what every fallback path renders, spelled so a test can tell a
// composed label from a fallback at a glance.
const storedName = "STORED"

func TestRankNameIsTheColourWordAloneAtDegreeZero(t *testing.T) {
	// Mirrors seed.rankName: a plain belt's name is its colour, not "Blue, 0 stripes".
	for locale, want := range map[i18n.Locale]string{
		i18n.German:  "Blau",
		i18n.English: "Blue",
	} {
		if got := RankName(locale, store.Rank{Group: "Blue", Degree: 0, Name: storedName}); got != want {
			t.Errorf("%s RankName(Blue, 0) = %q, want %q", locale, got, want)
		}
	}
}

func TestRankNamePhrasesEveryStripeDegree(t *testing.T) {
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
		if got := RankName(i18n.German, store.Rank{Group: "Blue", Degree: c.degree, Name: storedName}); got != c.german {
			t.Errorf("German RankName(Blue, %d) = %q, want %q", c.degree, got, c.german)
		}
		if got := RankName(i18n.English, store.Rank{Group: "Blue", Degree: c.degree, Name: storedName}); got != c.english {
			t.Errorf("English RankName(Blue, %d) = %q, want %q", c.degree, got, c.english)
		}
	}
}

func TestRankNameComposesSplitBelts(t *testing.T) {
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
		if got := RankName(i18n.German, store.Rank{Group: c.group, Degree: c.degree, Name: storedName}); got != c.german {
			t.Errorf("German RankName(%s, %d) = %q, want %q", c.group, c.degree, got, c.german)
		}
		if got := RankName(i18n.English, store.Rank{Group: c.group, Degree: c.degree, Name: storedName}); got != c.english {
			t.Errorf("English RankName(%s, %d) = %q, want %q", c.group, c.degree, got, c.english)
		}
	}
}

func TestRankNameFallsBackToTheStoredName(t *testing.T) {
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
			if got := RankName(locale, store.Rank{Group: c.group, Degree: c.degree, Name: storedName}); got != storedName {
				t.Errorf("%s RankName(%q, %d) = %q, want the stored name (%s)", locale, c.group, c.degree, got, c.why)
			}
		}
	}
}

// TestRankNameCoversEverySeededColour is the analogue of
// TestBeltSVGCoversEverySeededColour: a forgotten colour word degrades a real rank
// to English silently, so every group the seed writes must compose in both
// locales, across every degree the catalog enumerates.
func TestRankNameCoversEverySeededColour(t *testing.T) {
	for _, group := range store.SeededRankGroups() {
		for degree := 0; degree <= beltMaxStripes; degree++ {
			for _, locale := range []i18n.Locale{i18n.German, i18n.English} {
				if got := RankName(locale, store.Rank{Group: group, Degree: degree, Name: storedName}); got == storedName {
					t.Errorf("%s RankName(%q, %d) fell back to the stored name, want a composed label", locale, group, degree)
				}
			}
		}
	}
}

func TestSystemNameIsKeyedOnTheSlug(t *testing.T) {
	// A system's name is a display label and its slug the identity (ADR-0006), so
	// the translation hangs off the slug.
	if got := SystemName(i18n.German, store.System{Slug: "bjj-kids", Name: storedName}); got != "BJJ Kinder" {
		t.Errorf("German SystemName(bjj-kids) = %q, want %q", got, "BJJ Kinder")
	}
	if got := SystemName(i18n.English, store.System{Slug: "bjj-kids", Name: storedName}); got != "BJJ Kids" {
		t.Errorf("English SystemName(bjj-kids) = %q, want %q", got, "BJJ Kids")
	}
	// A club's own system, and one predating the slug column: both keep the name
	// the database holds, so an untranslated system stays usable.
	for _, slug := range []string{"bjj-elderly", ""} {
		if got := SystemName(i18n.German, store.System{Slug: slug, Name: storedName}); got != storedName {
			t.Errorf("SystemName(%q) = %q, want the stored name", slug, got)
		}
	}
}

func TestRosterLabelNamesRankAndSystemTogether(t *testing.T) {
	// On the roster the belt graphic stands alone, so its accessible name carries
	// both the rank and the system that tells a Kids white belt from an Adult one.
	graded := store.Rank{
		ID:     1,
		Name:   "Grey-White, 2 stripes",
		Group:  "Grey-White",
		Degree: 2,
		System: store.System{Name: "BJJ Kids", Slug: "bjj-kids"},
	}
	if got := rosterLabel(rosterNames(i18n.German, graded)); got != "Grau-Weiß, 2 Streifen (BJJ Kinder)" {
		t.Errorf("German roster label = %q, want %q", got, "Grau-Weiß, 2 Streifen (BJJ Kinder)")
	}
	if got := rosterLabel(rosterNames(i18n.English, graded)); got != "Grey-White, 2 stripes (BJJ Kids)" {
		t.Errorf("English roster label = %q, want %q", got, "Grey-White, 2 stripes (BJJ Kids)")
	}

	// An ungraded athlete has no rank and no system, so there is nothing to name.
	if got := rosterLabel(rosterNames(i18n.German, store.Rank{})); got != "" {
		t.Errorf("ungraded roster label = %q, want it empty", got)
	}
}

// TestRosterLabelNamesTheSystemOfAnyRank pins which question the bracket asks. A
// held rank is what gives an athlete a system to name (store.Rank.IsZero); neither
// the slug nor the stored name decides it.
func TestRosterLabelNamesTheSystemOfAnyRank(t *testing.T) {
	// No slug means no catalog key, so the stored name renders (ADR-0009) — a
	// club's own system stays usable rather than turning invisible.
	rank := store.Rank{
		ID:     7,
		Name:   "Club White",
		Group:  "White",
		System: store.System{Name: "Club System"},
	}
	if got := rosterLabel(rosterNames(i18n.German, rank)); got != "Weiß (Club System)" {
		t.Errorf("slugless system label = %q, want %q", got, "Weiß (Club System)")
	}

	// "BJJ Kinder" comes from the catalog, keyed on the slug (ADR-0009).
	rank.System = store.System{Slug: "bjj-kids"}
	if got := rosterLabel(rosterNames(i18n.German, rank)); got != "Weiß (BJJ Kinder)" {
		t.Errorf("unnamed system label = %q, want %q", got, "Weiß (BJJ Kinder)")
	}

	// Neither: the empty-label guard in rosterLabel.
	rank.System = store.System{}
	if got := rosterLabel(rosterNames(i18n.German, rank)); got != "Weiß" {
		t.Errorf("nameless system label = %q, want %q", got, "Weiß")
	}
}

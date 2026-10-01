package rankview_test

// Tests of the module through its exported names alone, which is what proves it
// is usable from outside. The colour × degree matrices and the belt geometry are
// tested in-package, in belt_internal_test.go and ranklabel_internal_test.go.

import (
	"strings"
	"testing"

	"github.com/TheMaru/ma_training_organizer/internal/i18n"
	"github.com/TheMaru/ma_training_organizer/internal/rankview"
	"github.com/TheMaru/ma_training_organizer/internal/store"
)

var (
	kidsGreyWhite = store.Rank{
		ID: 1, Name: "Grey-White, 2 stripes", Group: "Grey-White", Degree: 2,
		System: store.System{Name: "BJJ Kids", Slug: "bjj-kids"},
	}
	// A club's own rank in a club's own system: no belt colour the view knows and
	// no catalog key, so every name in it is the stored one.
	clubCyan = store.Rank{
		ID: 2, Name: "Cyan, 2 stripes", Group: "Cyan", Degree: 2,
		System: store.System{Name: "Club System"},
	}
)

func TestRosterRankIsTheBeltAloneWhenItCanBeDrawn(t *testing.T) {
	got := string(rankview.RosterRank(i18n.German, kidsGreyWhite))
	if !strings.HasPrefix(got, "<svg") || !strings.HasSuffix(got, "</svg>") {
		t.Errorf("roster rank = %q, want the belt graphic and nothing beside it", got)
	}
	if want := `role="img" aria-label="Grau-Weiß, 2 Streifen (BJJ Kinder)"`; !strings.Contains(got, want) {
		t.Errorf("roster rank = %q, want it labelled %s", got, want)
	}
}

func TestRosterRankWritesTheWordsWhenNoBeltCanBeDrawn(t *testing.T) {
	got := string(rankview.RosterRank(i18n.German, clubCyan))
	if want := `Cyan, 2 stripes <span class="muted">(Club System)</span>`; got != want {
		t.Errorf("roster rank = %q, want %q", got, want)
	}
}

func TestRosterRankOfNoRankIsEmpty(t *testing.T) {
	if got := rankview.RosterRank(i18n.German, store.Rank{}); got != "" {
		t.Errorf("roster rank of an ungraded athlete = %q, want it empty", got)
	}
}

func TestRankWithBeltDrawsADecorativeBeltBesideTheName(t *testing.T) {
	got := string(rankview.RankWithBelt(i18n.German, kidsGreyWhite))
	if !strings.HasPrefix(got, "<svg") || !strings.Contains(got, `aria-hidden="true"`) {
		t.Errorf("rank with belt = %q, want a decorative belt first", got)
	}
	if want := "</svg> Grau-Weiß, 2 Streifen"; !strings.HasSuffix(got, want) {
		t.Errorf("rank with belt = %q, want it to end %q", got, want)
	}

	if got := rankview.RankWithBelt(i18n.German, clubCyan); got != "Cyan, 2 stripes" {
		t.Errorf("rank with belt, no colour = %q, want the name alone", got)
	}
}

// TestStoredTextIsEscaped pins that the names the database holds reach the page as
// text, now that the module wraps them in markup it builds itself.
func TestStoredTextIsEscaped(t *testing.T) {
	hostile := store.Rank{
		ID: 3, Name: `<b>Club</b>`, Group: "Cyan",
		System: store.System{Name: `<i>"Club"</i>`},
	}
	for name, got := range map[string]string{
		"RosterRank":   string(rankview.RosterRank(i18n.German, hostile)),
		"RankWithBelt": string(rankview.RankWithBelt(i18n.German, hostile)),
	} {
		if strings.Contains(got, "<b>") || strings.Contains(got, "<i>") {
			t.Errorf("%s = %q, want the stored names escaped", name, got)
		}
		if !strings.Contains(got, "&lt;b&gt;Club&lt;/b&gt;") {
			t.Errorf("%s = %q, want the stored rank name as text", name, got)
		}
	}

	// A drawable belt puts the system name into its label instead.
	hostile.Group = "Blue"
	if got := string(rankview.RosterRank(i18n.German, hostile)); strings.Contains(got, "<i>") {
		t.Errorf("RosterRank = %q, want the stored system name escaped in the label", got)
	}
}

func TestRankNameAndSystemNameAreInTheTrainersLanguage(t *testing.T) {
	if got := rankview.RankName(i18n.German, kidsGreyWhite); got != "Grau-Weiß, 2 Streifen" {
		t.Errorf("RankName = %q, want %q", got, "Grau-Weiß, 2 Streifen")
	}
	if got := rankview.SystemName(i18n.German, kidsGreyWhite.System); got != "BJJ Kinder" {
		t.Errorf("SystemName = %q, want %q", got, "BJJ Kinder")
	}
	if got := rankview.SystemName(i18n.German, clubCyan.System); got != "Club System" {
		t.Errorf("SystemName of a club's own system = %q, want the stored name", got)
	}
}

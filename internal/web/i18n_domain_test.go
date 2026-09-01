package web_test

// The rendered-page seam for localized domain data (ADR-0009): rank and
// grading-system names as a trainer actually reads them, in both languages,
// through the real templates. The exhaustive colour × degree matrix and the
// fallbacks are unit-tested in-package instead — see ranklabel_test.go.

import (
	"database/sql"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/TheMaru/ma_training_organizer/internal/store"
	"github.com/TheMaru/ma_training_organizer/internal/store/storetest"
	"github.com/TheMaru/ma_training_organizer/internal/trainer/trainertest"
)

func TestRosterRankLabelFollowsTheLocale(t *testing.T) {
	ts, client, db := newAuthTestServer(t)
	login(t, ts, client, testUsername, trainertest.Password).Body.Close()
	mixedRoster(t, db)

	// The roster's belt graphic stands alone, so the composed label is what a screen
	// reader announces and what the tooltip shows.
	german := beltIn(t, cellWithLabel(t, rosterRow(t, readBody(t, get(t, ts, client, "/athletes")), "Kind"), "Aktueller Rang"))
	for _, want := range []string{
		`aria-label="Grau-Weiß, 2 Streifen (BJJ Kinder)"`,
		`<title>Grau-Weiß, 2 Streifen (BJJ Kinder)</title>`,
	} {
		if !strings.Contains(german, want) {
			t.Errorf("German roster belt = %q, want %s", german, want)
		}
	}

	// Switching language switches the rank with everything else: English is composed
	// from the English catalog too, so nothing on the page lags behind the switcher.
	switchTo(t, ts, client, "en", "/athletes").Body.Close()
	english := beltIn(t, cellWithLabel(t, rosterRow(t, readBody(t, get(t, ts, client, "/athletes")), "Kind"), "Current rank"))
	if !strings.Contains(english, `aria-label="Grey-White, 2 stripes (BJJ Kids)"`) {
		t.Errorf("English roster belt = %q, want the English rank and system", english)
	}
}

func TestRosterFilterChipsCarryLocalizedSystemNames(t *testing.T) {
	ts, client, db := newAuthTestServer(t)
	login(t, ts, client, testUsername, trainertest.Password).Body.Close()
	mixedRoster(t, db)

	// The chips must name the systems the way the rows they filter do, or the filter
	// row would read as a different vocabulary than the table below it.
	body := readBody(t, get(t, ts, client, "/athletes"))
	want := []string{"Alle", "BJJ Kinder", "BJJ Erwachsene", "Ohne Graduierung"}
	if got := filterChipLabels(t, body); !slices.Equal(got, want) {
		t.Errorf("German filter chips = %v, want %v", got, want)
	}

	switchTo(t, ts, client, "en", "/athletes").Body.Close()
	body = readBody(t, get(t, ts, client, "/athletes"))
	want = []string{"All", "BJJ Kids", "BJJ Adult", "Ungraded"}
	if got := filterChipLabels(t, body); !slices.Equal(got, want) {
		t.Errorf("English filter chips = %v, want %v", got, want)
	}
}

func TestAthleteDetailLocalizesRankAndSystem(t *testing.T) {
	ts, client, db := newAuthTestServer(t)
	login(t, ts, client, testUsername, trainertest.Password).Body.Close()
	id := promoteTo(t, db, "Mia", "Kind", "BJJ Kids", "Grey-White, 2 stripes", "2026-02-02")
	path := fmt.Sprintf("/athletes/%d", id)

	// All four of this page's surfaces: the current-rank line, the dropdown's ranks
	// and its system groups, and the history table's rank and system columns.
	body := readBody(t, get(t, ts, client, path))
	if current := definitionValue(t, body, "Aktueller Rang"); !strings.Contains(current, "Grau-Weiß, 2 Streifen") ||
		!strings.Contains(current, "BJJ Kinder") {
		t.Errorf("German current rank = %q, want the localized rank and system", current)
	}
	if !strings.Contains(body, `<optgroup label="BJJ Kinder">`) {
		t.Error("the promotion form's system groups stayed English on a German page")
	}
	if !strings.Contains(body, ">Grau-Weiß, 2 Streifen</option>") {
		t.Error("the promotion form's rank options stayed English on a German page")
	}
	row := historyRow(t, body, "2026-02-02")
	if rank := cellWithLabel(t, row, "Rang"); !strings.Contains(rank, "Grau-Weiß, 2 Streifen") {
		t.Errorf("German history rank = %q, want the localized rank", rank)
	}
	if system := cellWithLabel(t, row, "System"); !strings.Contains(system, "BJJ Kinder") {
		t.Errorf("German history system = %q, want the localized system", system)
	}

	switchTo(t, ts, client, "en", path).Body.Close()
	body = readBody(t, get(t, ts, client, path))
	if current := definitionValue(t, body, "Current rank"); !strings.Contains(current, "Grey-White, 2 stripes") ||
		!strings.Contains(current, "BJJ Kids") {
		t.Errorf("English current rank = %q, want the English rank and system", current)
	}
	if !strings.Contains(body, `<optgroup label="BJJ Kids">`) || !strings.Contains(body, ">Grey-White, 2 stripes</option>") {
		t.Error("the promotion form did not follow the switch to English")
	}
	row = historyRow(t, body, "2026-02-02")
	if rank := cellWithLabel(t, row, "Rank"); !strings.Contains(rank, "Grey-White, 2 stripes") {
		t.Errorf("English history rank = %q, want the English rank", rank)
	}
	if system := cellWithLabel(t, row, "System"); !strings.Contains(system, "BJJ Kids") {
		t.Errorf("English history system = %q, want the English system", system)
	}
}

// An athlete with no promotion has no rank and no system to compose, a case that
// was already correct in both languages: composition must not disturb it.
func TestUngradedAthleteKeepsTheNoRankWording(t *testing.T) {
	ts, client, db := newAuthTestServer(t)
	login(t, ts, client, testUsername, trainertest.Password).Body.Close()
	mixedRoster(t, db)

	body := readBody(t, get(t, ts, client, "/athletes"))
	cell := cellWithLabel(t, rosterRow(t, body, "Unbelted"), "Aktueller Rang")
	if strings.TrimSpace(cell) != "" {
		t.Errorf("ungraded rank cell = %q, want it blank", cell)
	}
	detail := readBody(t, get(t, ts, client, ungradedPath(t, body)))
	if current := definitionValue(t, detail, "Aktueller Rang"); !strings.Contains(current, "noch keine Graduierung") {
		t.Errorf("German ungraded detail = %q, want the ungraded note", current)
	}

	switchTo(t, ts, client, "en", "/athletes").Body.Close()
	body = readBody(t, get(t, ts, client, "/athletes"))
	if cell := cellWithLabel(t, rosterRow(t, body, "Unbelted"), "Current rank"); strings.TrimSpace(cell) != "" {
		t.Errorf("English ungraded rank cell = %q, want it blank", cell)
	}
	detail = readBody(t, get(t, ts, client, ungradedPath(t, body)))
	if current := definitionValue(t, detail, "Current rank"); !strings.Contains(current, "no promotion yet") {
		t.Errorf("English ungraded detail = %q, want the ungraded note", current)
	}
}

// A club's own rank in a colour the app has no word for keeps its stored name —
// but it sits in a system the catalog does know, so the graphic falls back to text
// while the system beside it still localizes. The two fall back independently.
func TestUnresolvableRankKeepsItsNameBesideALocalizedSystem(t *testing.T) {
	ts, client, db := newAuthTestServer(t)
	login(t, ts, client, testUsername, trainertest.Password).Body.Close()

	kids := seededSystemID(t, db, "BJJ Kids")
	rankID := storetest.MustInsert(t, db,
		`INSERT INTO ranks (grading_system_id, name, rank_group, degree, sort_order) VALUES (?, ?, ?, ?, ?)`,
		kids, "Rainbow", "Rainbow", 0, 99)
	id, err := store.CreateAthlete(db, store.Athlete{FirstName: "Robin", LastName: "Regenbogen"})
	if err != nil {
		t.Fatalf("CreateAthlete: %v", err)
	}
	if _, err := store.CreatePromotion(db, store.Promotion{AthleteID: id, RankID: rankID, PromotedOn: "2026-03-01"}); err != nil {
		t.Fatalf("CreatePromotion: %v", err)
	}

	cell := cellWithLabel(t, rosterRow(t, readBody(t, get(t, ts, client, "/athletes")), "Regenbogen"), "Aktueller Rang")
	if hasBelt(cell) {
		t.Errorf("rank cell = %q, want no graphic for an unmapped colour", cell)
	}
	if !strings.Contains(cell, "Rainbow") || !strings.Contains(cell, "BJJ Kinder") {
		t.Errorf("rank cell = %q, want the stored rank name and the localized system", cell)
	}
}

// seededSystemID returns a seeded grading system's id by name.
func seededSystemID(t *testing.T, db *sql.DB, name string) int64 {
	t.Helper()
	var id int64
	if err := db.QueryRow(`SELECT id FROM grading_systems WHERE name = ?`, name).Scan(&id); err != nil {
		t.Fatalf("lookup grading system %q: %v", name, err)
	}
	return id
}

// ungradedPath returns the detail link of the mixed roster's ungraded athlete.
func ungradedPath(t *testing.T, body string) string {
	t.Helper()
	return attrValue(t, rosterRow(t, body, "Unbelted"), "href")
}

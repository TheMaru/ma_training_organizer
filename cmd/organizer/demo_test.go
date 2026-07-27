package main

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/TheMaru/ma_training_organizer/internal/store"
)

// currentRankOf looks up an athlete by name and returns their derived current
// rank name (empty if none), for asserting demo-data outcomes.
func currentRankOf(t *testing.T, db *sql.DB, first, last string) string {
	t.Helper()
	names, err := athleteNames(db)
	if err != nil {
		t.Fatalf("athleteNames: %v", err)
	}
	id := names[nameKey(first, last)]
	if id == 0 {
		t.Fatalf("athlete %s %s not found", first, last)
	}
	promotions, err := store.ListPromotions(db, id)
	if err != nil {
		t.Fatalf("ListPromotions: %v", err)
	}
	cur, ok := store.CurrentRank(promotions)
	if !ok {
		return ""
	}
	return cur.RankName
}

func TestSeedDemoIsIdempotent(t *testing.T) {
	db := newTestDB(t)

	if err := seedDemo(db); err != nil {
		t.Fatalf("first seedDemo: %v", err)
	}
	if err := seedDemo(db); err != nil {
		t.Fatalf("second seedDemo: %v", err)
	}

	athletes, err := store.ListAthletes(db, false)
	if err != nil {
		t.Fatalf("ListAthletes: %v", err)
	}
	if len(athletes) != len(demoAthletes) {
		t.Errorf("athlete count = %d, want %d (re-seed must not duplicate)", len(athletes), len(demoAthletes))
	}

	// Promotions must not be duplicated either.
	var promotions int
	if err := db.QueryRow(`SELECT COUNT(*) FROM promotions`).Scan(&promotions); err != nil {
		t.Fatalf("count promotions: %v", err)
	}
	if want := demoPromotionCount(); promotions != want {
		t.Errorf("promotion count = %d, want %d", promotions, want)
	}
}

func TestSeedDemoDerivesCrossSystemCurrentRank(t *testing.T) {
	db := newTestDB(t)
	if err := seedDemo(db); err != nil {
		t.Fatalf("seedDemo: %v", err)
	}

	// Elias Keller graduated kids Green (2022) then adult White (2024): the adult
	// rank is latest, so it is current across systems (ADR-0001).
	if got := currentRankOf(t, db, "Elias", "Keller"); got != "White" {
		t.Errorf("Elias current rank = %q, want White (adult, latest)", got)
	}
	// Sophie has no promotion — blank current rank.
	if got := currentRankOf(t, db, "Sophie", "Neumann"); got != "" {
		t.Errorf("Sophie current rank = %q, want empty", got)
	}
}

// TestSeedDemoCoversTheBeltVisuals pins the demo roster to the job it exists for:
// a by-hand scan of the belt graphic (spec, ADR-0004). The demo data is the only
// place that combination of belts is asserted, so an edit that quietly drops a
// bar variant or the top stripe count would otherwise cost that confidence
// silently. The belts are read back through the ranks table, so it checks what the
// promotions actually resolve to rather than restating the rank names; only the
// ungraded athlete is counted off the slice, having no promotion to read.
func TestSeedDemoCoversTheBeltVisuals(t *testing.T) {
	db := newTestDB(t)
	if err := seedDemo(db); err != nil {
		t.Fatalf("seedDemo: %v", err)
	}

	// ~12 athletes: enough to cover the belts, few enough to scan by eye.
	if n := len(demoAthletes); n < 10 || n > 14 {
		t.Errorf("demo athletes = %d, want roughly 12", n)
	}

	rows, err := db.Query(`
		SELECT r.rank_group, r.degree
		FROM promotions p JOIN ranks r ON r.id = p.rank_id`)
	if err != nil {
		t.Fatalf("query demo ranks: %v", err)
	}
	defer rows.Close()

	bodies := map[string]bool{} // body colour, i.e. the part before any bar
	bars := map[string]bool{}   // the bar colour, "" for a plain belt
	degrees := map[int]bool{}   // stripe counts reached
	splitStriped, whiteStriped := false, false
	for rows.Next() {
		var (
			group  string
			degree int
		)
		if err := rows.Scan(&group, &degree); err != nil {
			t.Fatalf("scan demo rank: %v", err)
		}
		body, bar, split := strings.Cut(group, "-")
		bodies[body] = true
		bars[bar] = true
		degrees[degree] = true
		splitStriped = splitStriped || (split && degree > 0)
		whiteStriped = whiteStriped || (body == "White" && degree > 0)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate demo ranks: %v", err)
	}

	// Every body colour the view knows how to draw.
	for _, colour := range []string{"White", "Grey", "Yellow", "Orange", "Green", "Blue", "Purple", "Brown", "Black"} {
		if !bodies[colour] {
			t.Errorf("no demo promotion reaches a %s belt", colour)
		}
	}
	// Both split-belt bars, plus plain belts that have none.
	for _, bar := range []string{"White", "Black", ""} {
		if !bars[bar] {
			t.Errorf("no demo promotion reaches a belt with bar %q", bar)
		}
	}
	// The full stripe range: 0 vs 1 vs many, up to the maximum the friso holds.
	for degree := range 5 {
		if !degrees[degree] {
			t.Errorf("no demo promotion reaches degree %d", degree)
		}
	}
	if !splitStriped {
		t.Error("no demo promotion combines a split belt with stripes")
	}
	if !whiteStriped {
		t.Error("no demo promotion is a white belt with stripes (friso on a light body)")
	}

	// Exactly one athlete with no graduation at all: the text/blank fallback.
	ungraded := 0
	for _, d := range demoAthletes {
		if len(d.promotions) == 0 {
			ungraded++
		}
	}
	if ungraded != 1 {
		t.Errorf("ungraded demo athletes = %d, want exactly 1", ungraded)
	}
}

func TestClearDemoRemovesOnlyDemoAthletes(t *testing.T) {
	db := newTestDB(t)

	// A real athlete that is not part of the demo set must survive a clear.
	if _, err := store.CreateAthlete(db, store.Athlete{FirstName: "Real", LastName: "Person"}); err != nil {
		t.Fatalf("CreateAthlete: %v", err)
	}
	if err := seedDemo(db); err != nil {
		t.Fatalf("seedDemo: %v", err)
	}
	if err := clearDemo(db); err != nil {
		t.Fatalf("clearDemo: %v", err)
	}

	athletes, err := store.ListAthletes(db, false)
	if err != nil {
		t.Fatalf("ListAthletes: %v", err)
	}
	if len(athletes) != 1 || athletes[0].LastName != "Person" {
		t.Errorf("after clear, athletes = %+v, want only the non-demo Real Person", athletes)
	}

	// Grading systems are reference data and must remain after a clear.
	var systems int
	if err := db.QueryRow(`SELECT COUNT(*) FROM grading_systems`).Scan(&systems); err != nil {
		t.Fatalf("count systems: %v", err)
	}
	if systems == 0 {
		t.Error("clearDemo removed grading systems, want them left intact")
	}
}

func TestClearDemoIsIdempotent(t *testing.T) {
	db := newTestDB(t)
	// Clearing when nothing was seeded must be a clean no-op.
	if err := clearDemo(db); err != nil {
		t.Fatalf("clearDemo on empty db: %v", err)
	}
}

func demoPromotionCount() int {
	n := 0
	for _, d := range demoAthletes {
		n += len(d.promotions)
	}
	return n
}

package main

import (
	"database/sql"
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

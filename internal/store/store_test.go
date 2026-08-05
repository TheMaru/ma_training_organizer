package store_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/TheMaru/ma_training_organizer/internal/store"
	"github.com/TheMaru/ma_training_organizer/internal/store/storetest"
)

// TestOpenYieldsAMigratedAndSeededDatabase is the interface's whole promise: one
// call, and what comes back is ready — every table present and the built-in
// GradingSystems in it. No caller has an order to get right, and none can reach a
// database that is only half prepared.
func TestOpenYieldsAMigratedAndSeededDatabase(t *testing.T) {
	db := storetest.NewDB(t)

	// Every table declared in the initial migration must be queryable.
	tables := []string{
		"trainers", "sessions", "grading_systems",
		"ranks", "athletes", "promotions",
	}
	for _, table := range tables {
		if _, err := db.Exec("SELECT * FROM " + table + " WHERE 1 = 0"); err != nil {
			t.Errorf("table %q not usable after Open: %v", table, err)
		}
	}

	systems, err := store.ListGradingSystems(db)
	if err != nil {
		t.Fatalf("ListGradingSystems: %v", err)
	}
	if len(systems) == 0 {
		t.Fatal("no grading systems after Open, want the built-in seed")
	}
	// A system without ranks is nothing a Promotion could target.
	for _, s := range systems {
		if len(s.Ranks) == 0 {
			t.Errorf("seeded system %q has no ranks", s.Name)
		}
	}
}

// TestOpeningAPreparedDatabaseAgainChangesNothing is what licenses migrating and
// seeding on every open: the second open of a live database must neither
// duplicate the reference data nor disturb what a Trainer has recorded against it.
func TestOpeningAPreparedDatabaseAgainChangesNothing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	first, err := store.Open(path)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	athleteID, err := store.CreateAthlete(first, store.Athlete{FirstName: "Ada", LastName: "Lovelace"})
	if err != nil {
		t.Fatalf("CreateAthlete: %v", err)
	}
	if _, err := store.CreatePromotion(first, store.Promotion{
		AthleteID: athleteID, RankID: storetest.RankID(t, first, "BJJ Adult", "White"), PromotedOn: "2026-01-01",
	}); err != nil {
		t.Fatalf("CreatePromotion: %v", err)
	}
	first.Close()

	db, err := store.Open(path)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	// The reference data is unchanged: kids 13 belts × 4 degrees, adult 5 × 5.
	counts := map[string]int{"grading_systems": 2, "ranks": 13*4 + 5*5, "athletes": 1, "promotions": 1}
	for table, want := range counts {
		var got int
		if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&got); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if got != want {
			t.Errorf("%s after reopen = %d, want %d", table, got, want)
		}
	}
}

// TestOpenAcceptsAPathContainingAQuerySeparator pins the escaping: the pragmas
// travel as a query string, so a path holding a "?" must not be read as one —
// which would open a different file and silently drop foreign-key enforcement.
func TestOpenAcceptsAPathContainingAQuerySeparator(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "who? 100%.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatalf("Open %q: %v", path, err)
	}
	t.Cleanup(func() { db.Close() })

	// The file is the one that was asked for, not a prefix of it.
	if _, err := os.Stat(path); err != nil {
		t.Errorf("stat %q: %v", path, err)
	}
	// And the pragmas arrived: a dangling foreign key is still refused.
	_, err = db.Exec(
		`INSERT INTO promotions (athlete_id, rank_id, promoted_on) VALUES (?, ?, ?)`,
		999, 999, "2026-01-01",
	)
	if err == nil {
		t.Error("dangling foreign key accepted, want the pragma applied")
	}
}

func TestForeignKeysEnforced(t *testing.T) {
	db := storetest.NewDB(t)

	// promotions.rank_id / athlete_id reference rows that do not exist; with
	// foreign_keys enforcement on, the insert must be rejected.
	_, err := db.Exec(
		`INSERT INTO promotions (athlete_id, rank_id, promoted_on) VALUES (?, ?, ?)`,
		999, 999, "2026-01-01",
	)
	if err == nil {
		t.Fatal("expected foreign-key violation, got nil error")
	}
}

func TestDeleteAthleteCascadesToPromotions(t *testing.T) {
	db := storetest.NewDB(t)

	rankID := storetest.RankID(t, db, "BJJ Adult", "White")
	athleteID := storetest.MustInsert(t, db, `INSERT INTO athletes (first_name, last_name) VALUES (?, ?)`, "Ada", "Lovelace")
	storetest.MustInsert(t, db, `INSERT INTO promotions (athlete_id, rank_id, promoted_on) VALUES (?, ?, ?)`, athleteID, rankID, "2026-01-01")

	if _, err := db.Exec(`DELETE FROM athletes WHERE id = ?`, athleteID); err != nil {
		t.Fatalf("delete athlete: %v", err)
	}

	var remaining int
	if err := db.QueryRow(`SELECT COUNT(*) FROM promotions WHERE athlete_id = ?`, athleteID).Scan(&remaining); err != nil {
		t.Fatalf("count promotions: %v", err)
	}
	if remaining != 0 {
		t.Errorf("promotions after cascade = %d, want 0", remaining)
	}
}

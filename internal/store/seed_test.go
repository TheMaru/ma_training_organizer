package store_test

import (
	"database/sql"
	"testing"

	"github.com/TheMaru/ma_training_organizer/internal/store"
)

// seededDB returns a migrated database with the built-in grading systems seeded.
func seededDB(t *testing.T) *sql.DB {
	t.Helper()
	db := newTestDB(t)
	if err := store.Seed(db); err != nil {
		t.Fatalf("Seed: %v", err)
	}
	return db
}

func TestSeedCreatesSystemsAndRanks(t *testing.T) {
	db := seededDB(t)

	// Kids: 13 belts × 4 ranks (0–3 stripes); Adult: 5 belts × 5 (0–4 stripes).
	wantCounts := map[string]int{
		"BJJ Kids":  13 * 4,
		"BJJ Adult": 5 * 5,
	}
	for system, want := range wantCounts {
		var got int
		err := db.QueryRow(`
			SELECT COUNT(*) FROM ranks r
			JOIN grading_systems g ON g.id = r.grading_system_id
			WHERE g.name = ?`, system).Scan(&got)
		if err != nil {
			t.Fatalf("count ranks for %q: %v", system, err)
		}
		if got != want {
			t.Errorf("rank count for %q = %d, want %d", system, got, want)
		}
	}
}

// TestSeedRanksOrderedWithMetadata checks the seam the acceptance calls out:
// ranks are queryable in order per system and each carries group + degree.
func TestSeedRanksOrderedWithMetadata(t *testing.T) {
	db := seededDB(t)

	type rank struct {
		name   string
		group  string
		degree int
	}
	rows, err := db.Query(`
		SELECT r.name, r.rank_group, r.degree
		FROM ranks r
		JOIN grading_systems g ON g.id = r.grading_system_id
		WHERE g.name = 'BJJ Adult'
		ORDER BY r.sort_order`)
	if err != nil {
		t.Fatalf("query ranks: %v", err)
	}
	defer rows.Close()

	var got []rank
	for rows.Next() {
		var r rank
		if err := rows.Scan(&r.name, &r.group, &r.degree); err != nil {
			t.Fatalf("scan rank: %v", err)
		}
		got = append(got, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}

	// Spot-check the ordered boundaries and the acceptance example.
	if len(got) != 25 {
		t.Fatalf("BJJ Adult ranks = %d, want 25", len(got))
	}
	first := rank{name: "White", group: "White", degree: 0}
	if got[0] != first {
		t.Errorf("first rank = %+v, want %+v", got[0], first)
	}
	last := rank{name: "Black, 4 stripes", group: "Black", degree: 4}
	if got[len(got)-1] != last {
		t.Errorf("last rank = %+v, want %+v", got[len(got)-1], last)
	}
	// Acceptance: "White, 2 stripes" → group=White, degree=2.
	want := rank{name: "White, 2 stripes", group: "White", degree: 2}
	if got[2] != want {
		t.Errorf("ranks[2] = %+v, want %+v", got[2], want)
	}
}

func TestSeedIsIdempotent(t *testing.T) {
	db := seededDB(t) // first seed

	if err := store.Seed(db); err != nil {
		t.Fatalf("second Seed: %v", err)
	}

	var systems, ranks int
	if err := db.QueryRow(`SELECT COUNT(*) FROM grading_systems`).Scan(&systems); err != nil {
		t.Fatalf("count systems: %v", err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM ranks`).Scan(&ranks); err != nil {
		t.Fatalf("count ranks: %v", err)
	}
	if systems != 2 {
		t.Errorf("grading_systems after re-seed = %d, want 2", systems)
	}
	if ranks != 13*4+5*5 {
		t.Errorf("ranks after re-seed = %d, want %d", ranks, 13*4+5*5)
	}
}

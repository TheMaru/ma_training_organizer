package store_test

import (
	"path/filepath"
	"testing"

	"github.com/TheMaru/ma_training_organizer/internal/store"
	"github.com/TheMaru/ma_training_organizer/internal/store/storetest"
)

func TestSeedCreatesSystemsAndRanks(t *testing.T) {
	db := storetest.NewDB(t)

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
	db := storetest.NewDB(t)

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

// TestSeedGivesEverySystemItsSlug pins ADR-0006: the slug, not the name, is what
// identifies a system outside the database, so it must exist for every seeded
// system and be the hand-authored value rather than anything derived.
func TestSeedGivesEverySystemItsSlug(t *testing.T) {
	db := storetest.NewDB(t)

	want := map[string]string{"BJJ Kids": "bjj-kids", "BJJ Adult": "bjj-adult"}
	for name, wantSlug := range want {
		var got string
		if err := db.QueryRow(`SELECT slug FROM grading_systems WHERE name = ?`, name).Scan(&got); err != nil {
			t.Fatalf("read slug for %q: %v", name, err)
		}
		if got != wantSlug {
			t.Errorf("slug for %q = %q, want %q", name, got, wantSlug)
		}
	}
}

// TestSeedFillsTheSlugOfASystemThatHasNone is the migration path: a system that
// predates the slug column carries the column default, and the next boot must
// correct it — the same way ensureGradingSystem already corrects sort_order
// (migration 00003). Blanking the slug is how that state is reached now that
// every open seeds; reopening the file is the boot.
func TestSeedFillsTheSlugOfASystemThatHasNone(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := db.Exec(`UPDATE grading_systems SET slug = '' WHERE name = ?`, "BJJ Kids"); err != nil {
		t.Fatalf("blank the slug: %v", err)
	}
	db.Close()

	db, err = store.Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	var got string
	if err := db.QueryRow(`SELECT slug FROM grading_systems WHERE name = ?`, "BJJ Kids").Scan(&got); err != nil {
		t.Fatalf("read slug: %v", err)
	}
	if got != "bjj-kids" {
		t.Errorf("slug after reopen = %q, want %q", got, "bjj-kids")
	}
}

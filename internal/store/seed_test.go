package store_test

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TheMaru/ma_training_organizer/internal/store"
	"github.com/TheMaru/ma_training_organizer/internal/store/storetest"

	_ "modernc.org/sqlite" // for the one test that must read the file without seeding it
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

// TestSeedRepairsARespelledRankName is the regression for the rename trap: with
// the name as the lookup key, respelling one made the next seed miss and insert a
// second row for a rank promotions already pointed at. The natural key finds the
// row, and the name is corrected rather than duplicated — while sort_order, whose
// contract is that a re-seed never reorders, stays as it was found.
func TestSeedRepairsARespelledRankName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	// A respelling, plus a sort_order the seed must not touch.
	if _, err := db.Exec(`
		UPDATE ranks SET name = 'White I', sort_order = 999
		WHERE rank_group = 'White' AND degree = 1
		  AND grading_system_id = (SELECT id FROM grading_systems WHERE slug = 'bjj-adult')`); err != nil {
		t.Fatalf("respell the rank: %v", err)
	}
	db.Close()

	db, err = store.Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	rows, err := db.Query(`
		SELECT r.name, r.sort_order FROM ranks r
		JOIN grading_systems g ON g.id = r.grading_system_id
		WHERE g.slug = 'bjj-adult' AND r.rank_group = 'White' AND r.degree = 1`)
	if err != nil {
		t.Fatalf("query ranks: %v", err)
	}
	defer rows.Close()

	type rank struct {
		name  string
		order int
	}
	var got []rank
	for rows.Next() {
		var r rank
		if err := rows.Scan(&r.name, &r.order); err != nil {
			t.Fatalf("scan rank: %v", err)
		}
		got = append(got, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("rows for (White, 1) = %d, want 1: %+v", len(got), got)
	}
	if want := (rank{name: "White, 1 stripe", order: 999}); got[0] != want {
		t.Errorf("rank after re-seed = %+v, want %+v", got[0], want)
	}
}

// TestSeedFailsWhenARepairWouldCollide pins the loud failure. Correcting a name
// can run into the unique index on (grading_system_id, name) if another rank has
// meanwhile been given the name being restored. Two rows meaning one rank is the
// state this whole lookup exists to prevent, so the seed must refuse to open the
// database rather than pick a winner.
func TestSeedFailsWhenARepairWouldCollide(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	// Vacate "White, 1 stripe", then park it on the rank one degree up: the
	// re-seed now wants it back for the row it belongs to.
	for _, up := range []struct {
		degree int
		name   string
	}{
		{1, "White I"},
		{2, "White, 1 stripe"},
	} {
		if _, err := db.Exec(`
			UPDATE ranks SET name = ?
			WHERE rank_group = 'White' AND degree = ?
			  AND grading_system_id = (SELECT id FROM grading_systems WHERE slug = 'bjj-adult')`,
			up.name, up.degree); err != nil {
			t.Fatalf("rename degree %d: %v", up.degree, err)
		}
	}
	db.Close()

	db, err = store.Open(path)
	if err == nil {
		db.Close()
		t.Fatal("Open succeeded, want the colliding rename to fail the seed")
	}
	if !strings.Contains(err.Error(), "rename rank") {
		t.Errorf("Open error = %v, want the failing rename named in it", err)
	}

	// Loud is only half of it: the seed's transaction must leave the rows as it
	// found them. store.Open cannot report that — it fails on every attempt — so
	// the check goes through a plain connection with no seed behind it.
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open without seeding: %v", err)
	}
	defer raw.Close()
	var name string
	if err := raw.QueryRow(`
		SELECT r.name FROM ranks r
		JOIN grading_systems g ON g.id = r.grading_system_id
		WHERE g.slug = 'bjj-adult' AND r.rank_group = 'White' AND r.degree = 1`).Scan(&name); err != nil {
		t.Fatalf("read the rank back: %v", err)
	}
	if name != "White I" {
		t.Errorf("name after the failed seed = %q, want the rollback to have kept %q", name, "White I")
	}
}

// TestSeedIsIdempotent is the contract that lets Open seed on every boot: a
// second run over a database the first one seeded writes nothing. It covers the
// whole of the reference data rather than one column, so a hit path that starts
// correcting more than it should — sort_order above all — fails here.
func TestSeedIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	before := readReferenceData(t, db)
	db.Close()

	db, err = store.Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	after := readReferenceData(t, db)
	if len(before) != len(after) {
		t.Fatalf("reference rows after re-seeding = %d, want %d", len(after), len(before))
	}
	// 77 ranks make a full dump unreadable, and the first divergence is the one
	// that says what the second seed did.
	for i := range before {
		if before[i] != after[i] {
			t.Fatalf("re-seeding changed row %d:\nbefore %s\nafter  %s", i, before[i], after[i])
		}
	}
}

// readReferenceData renders every seeded system and rank, ids included, as
// comparable lines. The ids are what make an insert-then-delete churn visible
// where a count would not.
func readReferenceData(t *testing.T, db *sql.DB) []string {
	t.Helper()
	rows, err := db.Query(`
		SELECT g.id, g.name, g.slug, g.sort_order, r.id, r.name, r.rank_group, r.degree, r.sort_order
		FROM grading_systems g
		JOIN ranks r ON r.grading_system_id = g.id
		ORDER BY g.id, r.id`)
	if err != nil {
		t.Fatalf("read reference data: %v", err)
	}
	defer rows.Close()

	var got []string
	for rows.Next() {
		var (
			gsID, rankID   int64
			gsName, gsSlug string
			gsOrder        int
			name, group    string
			degree, order  int
		)
		if err := rows.Scan(&gsID, &gsName, &gsSlug, &gsOrder, &rankID, &name, &group, &degree, &order); err != nil {
			t.Fatalf("scan reference data: %v", err)
		}
		got = append(got, fmt.Sprintf("%d/%s/%s/%d %d/%s/%s/%d/%d",
			gsID, gsName, gsSlug, gsOrder, rankID, name, group, degree, order))
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
	return got
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

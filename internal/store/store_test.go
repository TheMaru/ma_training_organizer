package store_test

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/TheMaru/ma_training_organizer/internal/store"
)

// newTestDB opens a migrated database on a throwaway file. A file (not
// :memory:) is required because the connection pool may open several
// connections, and each in-memory connection would otherwise be a separate,
// unmigrated database.
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return db
}

func TestMigrateCreatesSchema(t *testing.T) {
	db := newTestDB(t)

	// Every table declared in the initial migration must be queryable.
	tables := []string{
		"trainers", "sessions", "grading_systems",
		"ranks", "athletes", "promotions",
	}
	for _, table := range tables {
		if _, err := db.Exec("SELECT * FROM " + table + " WHERE 1 = 0"); err != nil {
			t.Errorf("table %q not usable after migrate: %v", table, err)
		}
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	db := newTestDB(t) // first migration ran here

	// Running migrations again on an already-migrated DB must be a clean no-op.
	if err := store.Migrate(db); err != nil {
		t.Fatalf("second Migrate: %v", err)
	}
}

func TestForeignKeysEnforced(t *testing.T) {
	db := newTestDB(t)

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
	db := newTestDB(t)

	gsID := mustInsert(t, db, `INSERT INTO grading_systems (name) VALUES (?)`, "BJJ Adult")
	rankID := mustInsert(t, db, `INSERT INTO ranks (grading_system_id, name, sort_order) VALUES (?, ?, ?)`, gsID, "White", 0)
	athleteID := mustInsert(t, db, `INSERT INTO athletes (first_name, last_name) VALUES (?, ?)`, "Ada", "Lovelace")
	mustInsert(t, db, `INSERT INTO promotions (athlete_id, rank_id, promoted_on) VALUES (?, ?, ?)`, athleteID, rankID, "2026-01-01")

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

func mustInsert(t *testing.T, db *sql.DB, query string, args ...any) int64 {
	t.Helper()
	res, err := db.Exec(query, args...)
	if err != nil {
		t.Fatalf("insert (%s): %v", query, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("last insert id: %v", err)
	}
	return id
}

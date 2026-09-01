// Package storetest provides the database fixture the test suites share. It
// exists as its own package rather than as a helper inside one of them because
// the CLI tests live in package main and cannot import another suite's
// unexported helper — a shared package is the only way for "what a test
// database looks like" to have one answer.
//
// This is a fixture, not a second adapter at the store's seam: there is one
// adapter, the file-backed SQLite database, and this opens exactly that.
package storetest

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/TheMaru/ma_training_organizer/internal/store"
)

// NewDB returns a ready database on a throwaway file, closed when the test ends.
// A file rather than :memory: because the connection pool may open several
// connections, and each in-memory connection would otherwise be a separate,
// empty database.
func NewDB(t *testing.T) *sql.DB {
	t.Helper()
	return NewDBOn(t, filepath.Join(t.TempDir(), "test.db"))
}

// NewDBOn is NewDB on a named file, for a test that has to open a second handle
// on the same database and therefore has to know which file it is.
func NewDBOn(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := store.Open(path)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// MustInsert runs an insert and returns the new row's id, for the reference data
// a club would add beyond the built-in seed.
func MustInsert(t *testing.T, db *sql.DB, query string, args ...any) int64 {
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

// RankID looks up a seeded rank by its system and rank name, so a test can target
// a real Rank without hardcoding an auto-increment id.
func RankID(t *testing.T, db *sql.DB, system, name string) int64 {
	t.Helper()
	var id int64
	err := db.QueryRow(`
		SELECT r.id FROM ranks r
		JOIN grading_systems g ON g.id = r.grading_system_id
		WHERE g.name = ? AND r.name = ?`, system, name).Scan(&id)
	if err != nil {
		t.Fatalf("lookup rank %q/%q: %v", system, name, err)
	}
	return id
}

package store_test

import (
	"database/sql"
	"testing"

	"github.com/TheMaru/ma_training_organizer/internal/store"
)

// rosterFixture seeds two grading systems in a known order (kids before adult)
// with a beginner and an advanced rank each, so the cross-system rank ordering
// can be asserted without depending on the built-in seed data.
type rosterFixture struct {
	kidsBeginner, kidsAdvanced   int64
	adultBeginner, adultAdvanced int64
}

func newRosterFixture(t *testing.T, db *sql.DB) rosterFixture {
	t.Helper()
	kids := mustInsert(t, db, `INSERT INTO grading_systems (name, sort_order) VALUES (?, ?)`, "BJJ Kids", 0)
	adult := mustInsert(t, db, `INSERT INTO grading_systems (name, sort_order) VALUES (?, ?)`, "BJJ Adult", 1)
	rank := func(gsID int64, name string, order int) int64 {
		return mustInsert(t, db,
			`INSERT INTO ranks (grading_system_id, name, sort_order) VALUES (?, ?, ?)`, gsID, name, order)
	}
	return rosterFixture{
		kidsBeginner:  rank(kids, "White", 0),
		kidsAdvanced:  rank(kids, "Green", 12),
		adultBeginner: rank(adult, "White", 0),
		adultAdvanced: rank(adult, "Black", 4),
	}
}

// addAthlete creates an athlete and, when rankID is non-zero, promotes them to
// that rank on the given date.
func addAthlete(t *testing.T, db *sql.DB, a store.Athlete, rankID int64, promotedOn string) int64 {
	t.Helper()
	id, err := store.CreateAthlete(db, a)
	if err != nil {
		t.Fatalf("CreateAthlete %s: %v", a.FirstName, err)
	}
	if rankID != 0 {
		if _, err := store.CreatePromotion(db, store.Promotion{AthleteID: id, RankID: rankID, PromotedOn: promotedOn}); err != nil {
			t.Fatalf("CreatePromotion for %s: %v", a.FirstName, err)
		}
	}
	return id
}

func rosterFirstNames(rows []store.RosterRow) []string {
	names := make([]string, len(rows))
	for i, r := range rows {
		names[i] = r.FirstName
	}
	return names
}

func listRoster(t *testing.T, db *sql.DB, sort string, descending bool) []store.RosterRow {
	t.Helper()
	rows, err := store.ListRoster(db, sort, descending)
	if err != nil {
		t.Fatalf("ListRoster(%q, %v): %v", sort, descending, err)
	}
	return rows
}

// threeAthletes seeds an unsorted roster whose four name/date sort keys each
// produce a different order, so a test can tell the columns apart.
func threeAthletes(t *testing.T, db *sql.DB) {
	t.Helper()
	addAthlete(t, db, store.Athlete{FirstName: "Carol", LastName: "Adler", BirthDate: "2001-05-05", JoinedOn: "2026-01-01"}, 0, "")
	addAthlete(t, db, store.Athlete{FirstName: "Alice", LastName: "Zeder", BirthDate: "2003-01-01", JoinedOn: "2026-02-01"}, 0, "")
	addAthlete(t, db, store.Athlete{FirstName: "Bob", LastName: "Meier", BirthDate: "1999-09-09", JoinedOn: "2026-03-01"}, 0, "")
}

func TestListRosterSortsByEachColumn(t *testing.T) {
	db := newTestDB(t)
	threeAthletes(t, db)

	cases := []struct {
		sort string
		asc  []string
		desc []string
	}{
		{store.RosterSortFirstName, []string{"Alice", "Bob", "Carol"}, []string{"Carol", "Bob", "Alice"}},
		{store.RosterSortLastName, []string{"Carol", "Bob", "Alice"}, []string{"Alice", "Bob", "Carol"}},
		{store.RosterSortBirthDate, []string{"Bob", "Carol", "Alice"}, []string{"Alice", "Carol", "Bob"}},
		{store.RosterSortJoinedOn, []string{"Carol", "Alice", "Bob"}, []string{"Bob", "Alice", "Carol"}},
	}
	for _, c := range cases {
		if got := rosterFirstNames(listRoster(t, db, c.sort, false)); !equal(got, c.asc) {
			t.Errorf("sort %q ascending = %v, want %v", c.sort, got, c.asc)
		}
		if got := rosterFirstNames(listRoster(t, db, c.sort, true)); !equal(got, c.desc) {
			t.Errorf("sort %q descending = %v, want %v", c.sort, got, c.desc)
		}
	}
}

func TestListRosterFallsBackToFirstNameAscending(t *testing.T) {
	db := newTestDB(t)
	threeAthletes(t, db)

	// An unknown column is not an error — sorting is a view concern, so it falls
	// back silently to the default (Vorname ascending).
	want := []string{"Alice", "Bob", "Carol"}
	for _, sort := range []string{"", "unknown", "a.last_name; DROP TABLE athletes"} {
		if got := rosterFirstNames(listRoster(t, db, sort, false)); !equal(got, want) {
			t.Errorf("sort %q = %v, want default %v", sort, got, want)
		}
	}
}

func TestListRosterSortsMissingDatesLast(t *testing.T) {
	db := newTestDB(t)
	addAthlete(t, db, store.Athlete{FirstName: "Alice", LastName: "Adler"}, 0, "")
	addAthlete(t, db, store.Athlete{FirstName: "Bob", LastName: "Baum", BirthDate: "2000-01-01"}, 0, "")

	// A blank date is no value at all, so it sorts last in both directions rather
	// than counting as the earliest possible date.
	for _, descending := range []bool{false, true} {
		got := rosterFirstNames(listRoster(t, db, store.RosterSortBirthDate, descending))
		if !equal(got, []string{"Bob", "Alice"}) {
			t.Errorf("birth-date sort (descending=%v) = %v, want [Bob Alice]", descending, got)
		}
	}
}

func TestListRosterRankSortUsesSystemThenRankOrder(t *testing.T) {
	db := newTestDB(t)
	f := newRosterFixture(t, db)

	addAthlete(t, db, store.Athlete{FirstName: "Adam", LastName: "Adult"}, f.adultBeginner, "2026-01-01")
	addAthlete(t, db, store.Athlete{FirstName: "Bea", LastName: "Black"}, f.adultAdvanced, "2026-01-01")
	addAthlete(t, db, store.Athlete{FirstName: "Kai", LastName: "Kind"}, f.kidsBeginner, "2026-01-01")
	addAthlete(t, db, store.Athlete{FirstName: "Kim", LastName: "Kraft"}, f.kidsAdvanced, "2026-01-01")
	addAthlete(t, db, store.Athlete{FirstName: "Uwe", LastName: "Unbelted"}, 0, "")

	// Ascending = beginners first, kids block before adult block (the only
	// domain-honest cross-system ordering, spec). Ungraded athletes are not a
	// lowest rank — they sit after everyone, in both directions.
	wantAsc := []string{"Kai", "Kim", "Adam", "Bea", "Uwe"}
	if got := rosterFirstNames(listRoster(t, db, store.RosterSortRank, false)); !equal(got, wantAsc) {
		t.Errorf("rank ascending = %v, want %v", got, wantAsc)
	}
	wantDesc := []string{"Bea", "Adam", "Kim", "Kai", "Uwe"}
	if got := rosterFirstNames(listRoster(t, db, store.RosterSortRank, true)); !equal(got, wantDesc) {
		t.Errorf("rank descending = %v, want %v", got, wantDesc)
	}
}

func TestListRosterTieBreakStaysAlphabetical(t *testing.T) {
	db := newTestDB(t)
	f := newRosterFixture(t, db)

	addAthlete(t, db, store.Athlete{FirstName: "Carol", LastName: "Adler"}, f.kidsBeginner, "2026-01-01")
	addAthlete(t, db, store.Athlete{FirstName: "Alice", LastName: "Zeder"}, f.kidsBeginner, "2026-01-01")
	addAthlete(t, db, store.Athlete{FirstName: "Alice", LastName: "Meier"}, f.kidsBeginner, "2026-01-01")

	// Only the primary axis reverses with the direction: within one rank the names
	// stay A→Z, first name then last name.
	want := []string{"Alice Meier", "Alice Zeder", "Carol Adler"}
	for _, descending := range []bool{false, true} {
		rows := listRoster(t, db, store.RosterSortRank, descending)
		got := make([]string, len(rows))
		for i, r := range rows {
			got[i] = r.FirstName + " " + r.LastName
		}
		if !equal(got, want) {
			t.Errorf("tie order (descending=%v) = %v, want %v", descending, got, want)
		}
	}
}

func TestListRosterReportsCurrentRank(t *testing.T) {
	db := newTestDB(t)
	f := newRosterFixture(t, db)

	graded := addAthlete(t, db, store.Athlete{FirstName: "Kai", LastName: "Kind"}, f.kidsBeginner, "2024-01-01")
	if _, err := store.CreatePromotion(db, store.Promotion{AthleteID: graded, RankID: f.kidsAdvanced, PromotedOn: "2026-01-01"}); err != nil {
		t.Fatalf("CreatePromotion: %v", err)
	}
	addAthlete(t, db, store.Athlete{FirstName: "Uwe", LastName: "Unbelted"}, 0, "")

	rows := listRoster(t, db, store.RosterSortFirstName, false)
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	// The most recent promotion is the current rank, not the first or the last row
	// inserted.
	if rows[0].RankName != "Green" || rows[0].SystemName != "BJJ Kids" || rows[0].RankID != f.kidsAdvanced {
		t.Errorf("Kai's current rank = %+v, want Green / BJJ Kids", rows[0])
	}
	// Ungraded athletes survive the join with an empty rank.
	if rows[1].RankName != "" || rows[1].SystemName != "" || rows[1].RankID != 0 {
		t.Errorf("Uwe's rank = %+v, want empty", rows[1])
	}
}

// TestListRosterCurrentRankMatchesCurrentRank pins the two encodings of "current
// rank = latest promotion, higher id wins a tied date" together: the SQL window
// function used by the roster and the pure CurrentRank used by the detail page.
// The fixture includes a same-date tie, which is exactly where they could drift.
func TestListRosterCurrentRankMatchesCurrentRank(t *testing.T) {
	db := newTestDB(t)
	f := newRosterFixture(t, db)

	tied := addAthlete(t, db, store.Athlete{FirstName: "Tina", LastName: "Tie"}, f.kidsBeginner, "2026-01-01")
	// Same date, recorded later: the higher id must win for both encodings.
	if _, err := store.CreatePromotion(db, store.Promotion{AthleteID: tied, RankID: f.kidsAdvanced, PromotedOn: "2026-01-01"}); err != nil {
		t.Fatalf("CreatePromotion tie: %v", err)
	}
	addAthlete(t, db, store.Athlete{FirstName: "Adam", LastName: "Adult"}, f.adultBeginner, "2020-06-01")
	addAthlete(t, db, store.Athlete{FirstName: "Uwe", LastName: "Unbelted"}, 0, "")

	for _, row := range listRoster(t, db, store.RosterSortFirstName, false) {
		promotions, err := store.ListPromotions(db, row.ID)
		if err != nil {
			t.Fatalf("ListPromotions for %d: %v", row.ID, err)
		}
		current, ok := store.CurrentRank(promotions)
		switch {
		case !ok:
			if row.RankID != 0 || row.RankName != "" {
				t.Errorf("%s: roster rank = %+v, want none (CurrentRank found none)", row.FirstName, row)
			}
		case row.RankID != current.RankID || row.RankName != current.RankName || row.SystemName != current.SystemName:
			t.Errorf("%s: roster rank = (%d, %q, %q), CurrentRank = (%d, %q, %q)",
				row.FirstName, row.RankID, row.RankName, row.SystemName,
				current.RankID, current.RankName, current.SystemName)
		}
	}
}

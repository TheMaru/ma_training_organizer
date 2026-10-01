package store_test

import (
	"database/sql"
	"testing"

	"github.com/TheMaru/ma_training_organizer/internal/store"
	"github.com/TheMaru/ma_training_organizer/internal/store/storetest"
)

// rosterFixture names a beginner and an advanced rank in each of the two grading
// systems the seed writes, in the order seedSystems lists them.
type rosterFixture struct {
	kidsBeginner, kidsAdvanced   int64
	adultBeginner, adultAdvanced int64
}

func newRosterFixture(t *testing.T, db *sql.DB) rosterFixture {
	t.Helper()
	return rosterFixture{
		kidsBeginner:  storetest.RankID(t, db, "BJJ Kids", "White"),
		kidsAdvanced:  storetest.RankID(t, db, "BJJ Kids", "Green"),
		adultBeginner: storetest.RankID(t, db, "BJJ Adult", "White"),
		adultAdvanced: storetest.RankID(t, db, "BJJ Adult", "Black"),
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

func loadRoster(t *testing.T, db *sql.DB, query store.RosterQuery) store.RosterView {
	t.Helper()
	view, err := store.LoadRoster(db, query)
	if err != nil {
		t.Fatalf("LoadRoster(%+v): %v", query, err)
	}
	return view
}

// listRoster loads the unfiltered roster, for the tests that are about the SQL
// ordering rather than about the partition.
func listRoster(t *testing.T, db *sql.DB, sort string, descending bool) []store.RosterRow {
	t.Helper()
	return loadRoster(t, db, store.RosterQuery{Sort: sort, Descending: descending}).Rows
}

// threeAthletes seeds an unsorted roster whose four name/date sort keys each
// produce a different order, so a test can tell the columns apart.
func threeAthletes(t *testing.T, db *sql.DB) {
	t.Helper()
	addAthlete(t, db, store.Athlete{FirstName: "Carol", LastName: "Adler", BirthDate: "2001-05-05", JoinedOn: "2026-01-01"}, 0, "")
	addAthlete(t, db, store.Athlete{FirstName: "Alice", LastName: "Zeder", BirthDate: "2003-01-01", JoinedOn: "2026-02-01"}, 0, "")
	addAthlete(t, db, store.Athlete{FirstName: "Bob", LastName: "Meier", BirthDate: "1999-09-09", JoinedOn: "2026-03-01"}, 0, "")
}

// partitionedRoster seeds the smallest roster in which every cell of the partition
// holds someone: one kid, one adult, one ungraded.
func partitionedRoster(t *testing.T, db *sql.DB) {
	t.Helper()
	f := newRosterFixture(t, db)
	addAthlete(t, db, store.Athlete{FirstName: "Kai", LastName: "Kind"}, f.kidsBeginner, "2026-01-01")
	addAthlete(t, db, store.Athlete{FirstName: "Adam", LastName: "Adult"}, f.adultBeginner, "2026-01-01")
	addAthlete(t, db, store.Athlete{FirstName: "Uwe", LastName: "Unbelted"}, 0, "")
}

func TestListRosterSortsByEachColumn(t *testing.T) {
	db := storetest.NewDB(t)
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
	db := storetest.NewDB(t)
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
	db := storetest.NewDB(t)
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
	db := storetest.NewDB(t)
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
	db := storetest.NewDB(t)
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
	db := storetest.NewDB(t)
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
	if rows[0].Rank.Name != "Green" || rows[0].Rank.System.Name != "BJJ Kids" || rows[0].Rank.ID != f.kidsAdvanced {
		t.Errorf("Kai's current rank = %+v, want Green / BJJ Kids", rows[0])
	}
	// Ungraded athletes survive the join with an empty rank.
	if rows[1].Rank.Name != "" || rows[1].Rank.System.Name != "" || rows[1].Rank.ID != 0 {
		t.Errorf("Uwe's rank = %+v, want empty", rows[1])
	}
}

// TestListRosterCarriesTheSystemSlug pins the row's side of the roster filter:
// the filter identifies a system by its slug (ADR-0006) and derives its options
// from the rows themselves (ADR-0007), so the slug has to ride along rather than
// be looked up per athlete.
func TestListRosterCarriesTheSystemSlug(t *testing.T) {
	db := storetest.NewDB(t)
	f := newRosterFixture(t, db)

	addAthlete(t, db, store.Athlete{FirstName: "Kai", LastName: "Kind"}, f.kidsBeginner, "2026-01-01")
	addAthlete(t, db, store.Athlete{FirstName: "Uwe", LastName: "Unbelted"}, 0, "")

	rows := listRoster(t, db, store.RosterSortFirstName, false)
	if rows[0].Rank.System.Slug != "bjj-kids" {
		t.Errorf("Kai's system slug = %q, want %q", rows[0].Rank.System.Slug, "bjj-kids")
	}
	// An ungraded athlete is in no system at all, which is a distinct state from
	// being in one — not a slug the filter could match.
	if rows[1].Rank.System.Slug != "" {
		t.Errorf("Uwe's system slug = %q, want empty", rows[1].Rank.System.Slug)
	}
}

func TestListRosterCarriesRankGroupAndDegree(t *testing.T) {
	db := storetest.NewDB(t)
	striped := storetest.RankID(t, db, "BJJ Kids", "Grey-White, 2 stripes")

	addAthlete(t, db, store.Athlete{FirstName: "Kai", LastName: "Kind"}, striped, "2026-01-01")
	addAthlete(t, db, store.Athlete{FirstName: "Uwe", LastName: "Unbelted"}, 0, "")

	rows := listRoster(t, db, store.RosterSortFirstName, false)
	// The descriptive breakdown rides along so the roster can render the rank
	// without a second query (ADR-0004).
	if rows[0].Rank.Group != "Grey-White" || rows[0].Rank.Degree != 2 {
		t.Errorf("Kai's group/degree = %q/%d, want Grey-White/2", rows[0].Rank.Group, rows[0].Rank.Degree)
	}
	// Ungraded: the LEFT JOIN's NULLs must land as zero values, not an error.
	if rows[1].Rank.Group != "" || rows[1].Rank.Degree != 0 {
		t.Errorf("Uwe's group/degree = %q/%d, want empty/0", rows[1].Rank.Group, rows[1].Rank.Degree)
	}
}

// TestListRosterCurrentRankMatchesCurrentRank pins the two encodings of "current
// rank = latest promotion, higher id wins a tied date" together: the SQL window
// function used by the roster and the pure CurrentRank used by the detail page.
// The fixture includes a same-date tie, which is exactly where they could drift.
func TestListRosterCurrentRankMatchesCurrentRank(t *testing.T) {
	db := storetest.NewDB(t)
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
			if row.Rank != (store.Rank{}) {
				t.Errorf("%s: roster rank = %+v, want none (CurrentRank found none)", row.FirstName, row.Rank)
			}
		case row.Rank != current.Rank:
			t.Errorf("%s: roster rank = %+v, CurrentRank = %+v", row.FirstName, row.Rank, current.Rank)
		}
	}
}

func optionValues(options []store.RosterOption) []string {
	values := make([]string, len(options))
	for i, o := range options {
		values[i] = o.Value
	}
	return values
}

// TestLoadRosterResolvesAnUnrepresentedFilter is ADR-0007b's guarantee asserted at
// the store's own seam rather than three layers up: a filter for a system nobody
// is in is not a data error but a view that no longer exists, so it resolves to
// the unfiltered roster — and the resolution comes back in the query, because
// every link on the page is built from it.
func TestLoadRosterResolvesAnUnrepresentedFilter(t *testing.T) {
	db := storetest.NewDB(t)
	partitionedRoster(t, db)

	view := loadRoster(t, db, store.RosterQuery{Sort: store.RosterSortFirstName, Filter: "bjj-elderly"})

	if view.Query.Filter != "" {
		t.Errorf("resolved filter = %q, want empty (Alle)", view.Query.Filter)
	}
	if got := rosterFirstNames(view.Rows); !equal(got, []string{"Adam", "Kai", "Uwe"}) {
		t.Errorf("rows under an unrepresented filter = %v, want the whole roster", got)
	}
}

// TestLoadRosterDerivesOptionsFromTheUnfilteredRoster is the other half of
// ADR-0007a: narrowing the roster must not narrow the chips with it, or the way
// back to Alle disappears. LoadRoster is the only caller of the derivation now, so
// this is the only place the rule can be stated.
func TestLoadRosterDerivesOptionsFromTheUnfilteredRoster(t *testing.T) {
	db := storetest.NewDB(t)
	partitionedRoster(t, db)

	view := loadRoster(t, db, store.RosterQuery{Sort: store.RosterSortFirstName, Filter: "bjj-kids"})

	if got := rosterFirstNames(view.Rows); !equal(got, []string{"Kai"}) {
		t.Errorf("filtered rows = %v, want [Kai]", got)
	}
	want := []string{"bjj-kids", "bjj-adult", store.RosterFilterUngraded}
	if got := optionValues(view.Options); !equal(got, want) {
		t.Errorf("options under a filter = %v, want the unfiltered %v", got, want)
	}
	if view.Query.Filter != "bjj-kids" {
		t.Errorf("resolved filter = %q, want it kept", view.Query.Filter)
	}
}

// TestEveryOfferedOptionHasAthletes pins the invariant that licenses the absence
// of a zero-hit UI: no chip a trainer can click leads to an empty table. After the
// interface change this is a statement about the module rather than about two
// functions agreeing, so it is asserted by asking LoadRoster again for each option
// it just offered.
func TestEveryOfferedOptionHasAthletes(t *testing.T) {
	db := storetest.NewDB(t)
	partitionedRoster(t, db)

	offered := loadRoster(t, db, store.RosterQuery{Sort: store.RosterSortFirstName})
	if len(offered.Options) == 0 {
		t.Fatal("the fixture roster offers no options at all")
	}
	for _, option := range offered.Options {
		view := loadRoster(t, db, store.RosterQuery{Sort: store.RosterSortFirstName, Filter: option.Value})
		if len(view.Rows) == 0 {
			t.Errorf("offered option %q matches no athlete", option.Value)
		}
		// An option that came back resolved away would mean the same load offered a
		// filter it then refused — the drift this interface exists to rule out.
		if view.Query.Filter != option.Value {
			t.Errorf("offered option %q resolved to %q", option.Value, view.Query.Filter)
		}
	}
}

// sluglessRank inserts a grading system the seed does not know and one rank in
// it, returning the rank's id. That builds the row store.RosterRow.Ungraded's doc
// describes: a real rank behind an empty slug. Raw SQL because no store function
// can produce one (ADR-0007, Update 2026-09-01).
func sluglessRank(t *testing.T, db *sql.DB) int64 {
	t.Helper()
	system := storetest.MustInsert(t, db,
		`INSERT INTO grading_systems (name, sort_order) VALUES (?, ?)`, "Club System", 2)
	return storetest.MustInsert(t, db,
		`INSERT INTO ranks (grading_system_id, name, rank_group, degree, sort_order) VALUES (?, ?, ?, ?, ?)`,
		system, "Club White", "White", 0, 0)
}

// TestLoadRosterDecidesUngradedByTheRank is the regression test for that row. Why
// it also expects no chip for Clara's system is on rosterFilterOptions.
func TestLoadRosterDecidesUngradedByTheRank(t *testing.T) {
	db := storetest.NewDB(t)
	f := newRosterFixture(t, db)

	addAthlete(t, db, store.Athlete{FirstName: "Kai", LastName: "Kind"}, f.kidsBeginner, "2026-01-01")
	addAthlete(t, db, store.Athlete{FirstName: "Clara", LastName: "Club"}, sluglessRank(t, db), "2026-01-01")
	addAthlete(t, db, store.Athlete{FirstName: "Uwe", LastName: "Unbelted"}, 0, "")

	view := loadRoster(t, db, store.RosterQuery{Sort: store.RosterSortFirstName, Filter: store.RosterFilterUngraded})

	if got := rosterFirstNames(view.Rows); !equal(got, []string{"Uwe"}) {
		t.Errorf("the Ungraded filter returned %v, want [Uwe] — Clara holds a rank", got)
	}
	want := []string{"bjj-kids", store.RosterFilterUngraded}
	if got := optionValues(view.Options); !equal(got, want) {
		t.Errorf("options = %v, want %v", got, want)
	}
	all := loadRoster(t, db, store.RosterQuery{Sort: store.RosterSortFirstName})
	if got := rosterFirstNames(all.Rows); !equal(got, []string{"Clara", "Kai", "Uwe"}) {
		t.Errorf("unfiltered roster = %v, want Clara on it", got)
	}
}

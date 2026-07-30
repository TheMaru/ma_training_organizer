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
	kids := mustInsert(t, db, `INSERT INTO grading_systems (name, slug, sort_order) VALUES (?, ?, ?)`, "BJJ Kids", "bjj-kids", 0)
	adult := mustInsert(t, db, `INSERT INTO grading_systems (name, slug, sort_order) VALUES (?, ?, ?)`, "BJJ Adult", "bjj-adult", 1)
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

// TestListRosterCarriesTheSystemSlug pins the row's side of the roster filter:
// the filter identifies a system by its slug (ADR-0006) and derives its options
// from the rows themselves (ADR-0007), so the slug has to ride along rather than
// be looked up per athlete.
func TestListRosterCarriesTheSystemSlug(t *testing.T) {
	db := newTestDB(t)
	f := newRosterFixture(t, db)

	addAthlete(t, db, store.Athlete{FirstName: "Kai", LastName: "Kind"}, f.kidsBeginner, "2026-01-01")
	addAthlete(t, db, store.Athlete{FirstName: "Uwe", LastName: "Unbelted"}, 0, "")

	rows := listRoster(t, db, store.RosterSortFirstName, false)
	if rows[0].SystemSlug != "bjj-kids" {
		t.Errorf("Kai's system slug = %q, want %q", rows[0].SystemSlug, "bjj-kids")
	}
	// An ungraded athlete is in no system at all, which is a distinct state from
	// being in one — not a slug the filter could match.
	if rows[1].SystemSlug != "" {
		t.Errorf("Uwe's system slug = %q, want empty", rows[1].SystemSlug)
	}
}

func TestListRosterCarriesRankGroupAndDegree(t *testing.T) {
	db := newTestDB(t)
	kids := mustInsert(t, db, `INSERT INTO grading_systems (name, sort_order) VALUES (?, ?)`, "BJJ Kids", 0)
	striped := mustInsert(t, db,
		`INSERT INTO ranks (grading_system_id, name, rank_group, degree, sort_order) VALUES (?, ?, ?, ?, ?)`,
		kids, "Grey-White, 2 stripes", "Grey-White", 2, 1)

	addAthlete(t, db, store.Athlete{FirstName: "Kai", LastName: "Kind"}, striped, "2026-01-01")
	addAthlete(t, db, store.Athlete{FirstName: "Uwe", LastName: "Unbelted"}, 0, "")

	rows := listRoster(t, db, store.RosterSortFirstName, false)
	// The descriptive breakdown rides along so the roster can render the rank
	// without a second query (ADR-0004).
	if rows[0].Group != "Grey-White" || rows[0].Degree != 2 {
		t.Errorf("Kai's group/degree = %q/%d, want Grey-White/2", rows[0].Group, rows[0].Degree)
	}
	// Ungraded: the LEFT JOIN's NULLs must land as zero values, not an error.
	if rows[1].Group != "" || rows[1].Degree != 0 {
		t.Errorf("Uwe's group/degree = %q/%d, want empty/0", rows[1].Group, rows[1].Degree)
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

// partitionedRoster is a roster as the filter functions see it: the fields that
// decide which cell of the partition an athlete falls into, plus a name to
// identify them by in a failure message. One kid, one adult, one ungraded — the
// smallest roster in which every cell is populated.
func partitionedRoster() []store.RosterRow {
	row := func(first, slug, system string, order int) store.RosterRow {
		return store.RosterRow{
			Athlete:     store.Athlete{FirstName: first},
			SystemSlug:  slug,
			SystemName:  system,
			SystemOrder: order,
		}
	}
	return []store.RosterRow{
		row("Adam", "bjj-adult", "BJJ Adult", 1),
		row("Kai", "bjj-kids", "BJJ Kids", 0),
		row("Uwe", "", "", 0),
		row("Kim", "bjj-kids", "BJJ Kids", 0),
	}
}

func optionValues(options []store.RosterOption) []string {
	values := make([]string, len(options))
	for i, o := range options {
		values[i] = o.Value
	}
	return values
}

// TestRosterFilterOptionsPartitionTheRoster checks that every athlete falls into
// exactly one cell — their current rank's system, or ungraded — and that the
// offered options are the cells which actually hold someone (ADR-0007a).
func TestRosterFilterOptionsPartitionTheRoster(t *testing.T) {
	options := store.RosterFilterOptions(partitionedRoster())

	// Systems in progression order (kids before adult, the same order the rank
	// sort blocks them in), ungraded last.
	want := []string{"bjj-kids", "bjj-adult", store.RosterFilterUngraded}
	if got := optionValues(options); !equal(got, want) {
		t.Errorf("options = %v, want %v", got, want)
	}
	// The display name rides along; the ungraded cell is in no system and so has
	// none to show.
	if options[0].Name != "BJJ Kids" {
		t.Errorf("first option name = %q, want %q", options[0].Name, "BJJ Kids")
	}
	if options[2].Name != "" {
		t.Errorf("ungraded option name = %q, want empty", options[2].Name)
	}
}

// TestRosterFilterOptionsOmitEmptyCells is the whole point of deriving the
// options from the roster rather than from the seed: a system nobody is in gets
// no chip, so no offered option can yield an empty roster.
func TestRosterFilterOptionsOmitEmptyCells(t *testing.T) {
	rows := []store.RosterRow{
		{Athlete: store.Athlete{FirstName: "Kai"}, SystemSlug: "bjj-kids", SystemName: "BJJ Kids"},
		{Athlete: store.Athlete{FirstName: "Kim"}, SystemSlug: "bjj-kids", SystemName: "BJJ Kids"},
	}
	if got := optionValues(store.RosterFilterOptions(rows)); !equal(got, []string{"bjj-kids"}) {
		t.Errorf("options for a kids-only roster = %v, want [bjj-kids]", got)
	}
	// An all-ungraded roster offers only that cell — never a system nobody holds.
	ungraded := []store.RosterRow{{Athlete: store.Athlete{FirstName: "Uwe"}}}
	if got := optionValues(store.RosterFilterOptions(ungraded)); !equal(got, []string{store.RosterFilterUngraded}) {
		t.Errorf("options for an ungraded roster = %v, want [none]", got)
	}
	if got := store.RosterFilterOptions(nil); len(got) != 0 {
		t.Errorf("options for an empty roster = %v, want none", got)
	}
}

// TestFilterRosterRestrictsToOneCell checks the filter's whole promise: a system
// filter shows exactly the athletes whose current rank is in it. Ungraded athletes are hidden by it —
// a filter reading "BJJ Kids" that showed athletes in no system would not be
// telling the truth (ADR-0007) — and are reachable under their own option.
func TestFilterRosterRestrictsToOneCell(t *testing.T) {
	rows := partitionedRoster()

	cases := map[string][]string{
		"":                         {"Adam", "Kai", "Uwe", "Kim"},
		"bjj-kids":                 {"Kai", "Kim"},
		"bjj-adult":                {"Adam"},
		store.RosterFilterUngraded: {"Uwe"},
	}
	for option, want := range cases {
		if got := rosterFirstNames(store.FilterRoster(rows, option)); !equal(got, want) {
			t.Errorf("FilterRoster(%q) = %v, want %v", option, got, want)
		}
	}
}

// TestFilterRosterKeepsTheOrderItWasGiven pins that filtering does not reorder
// what it restricts — the sort happens in SQL, upstream of this.
func TestFilterRosterKeepsTheOrderItWasGiven(t *testing.T) {
	rows := []store.RosterRow{
		{Athlete: store.Athlete{FirstName: "Zoe"}, SystemSlug: "bjj-kids"},
		{Athlete: store.Athlete{FirstName: "Uwe"}},
		{Athlete: store.Athlete{FirstName: "Ada"}, SystemSlug: "bjj-kids"},
	}
	if got := rosterFirstNames(store.FilterRoster(rows, "bjj-kids")); !equal(got, []string{"Zoe", "Ada"}) {
		t.Errorf("filtered order = %v, want [Zoe Ada]", got)
	}
}

// TestEveryOfferedOptionHasAthletes pins the invariant that licenses the absence
// of a zero-hit UI: the options come from the same slice the filter restricts, so
// no chip a trainer can click leads to an empty table.
func TestEveryOfferedOptionHasAthletes(t *testing.T) {
	rows := partitionedRoster()
	for _, option := range store.RosterFilterOptions(rows) {
		if len(store.FilterRoster(rows, option.Value)) == 0 {
			t.Errorf("offered option %q matches no athlete", option.Value)
		}
	}
}

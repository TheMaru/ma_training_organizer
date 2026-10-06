package store

import (
	"slices"
	"testing"
)

// The roster's partition mechanics are reached from inside the package because
// LoadRoster deliberately hides them: which cells are non-empty, the order they
// come in, and the fact that narrowing does not reorder. Prior art for an internal
// test file: internal/i18n/fallback_internal_test.go, for the same reason.
//
// These construct rows directly and touch no database, which is what keeps the
// cheap cases — a system nobody is in, two systems tied on sort order — cheap to
// write.

// partitionRows is a roster as the partition functions see it: the fields that
// decide which cell an athlete falls into, plus a name to identify them by in a
// failure message. One kid, one adult, one ungraded — the smallest roster in which
// every cell is populated.
func partitionRows() []RosterRow {
	return []RosterRow{
		partitionRow("Adam", 1, "bjj-adult", "BJJ Adult", 1),
		partitionRow("Kai", 2, "bjj-kids", "BJJ Kids", 0),
		partitionRow("Uwe", 0, "", "", 0),
		partitionRow("Kim", 3, "bjj-kids", "BJJ Kids", 0),
	}
}

func partitionRow(first string, rankID int64, slug, system string, order int) RosterRow {
	return RosterRow{
		Athlete:     Athlete{FirstName: first},
		Rank:        Rank{ID: rankID, System: System{Name: system, Slug: slug}},
		SystemOrder: order,
	}
}

func firstNames(rows []RosterRow) []string {
	names := make([]string, len(rows))
	for i, r := range rows {
		names[i] = r.FirstName
	}
	return names
}

func values(options []RosterOption) []string {
	vs := make([]string, len(options))
	for i, o := range options {
		vs[i] = o.Value
	}
	return vs
}

// TestRosterFilterOptionsPartitionTheRoster checks that every athlete falls into
// exactly one cell — their current rank's system, or ungraded — and that the
// offered options are the cells which actually hold someone (ADR-0007a).
func TestRosterFilterOptionsPartitionTheRoster(t *testing.T) {
	options := rosterFilterOptions(partitionRows())

	// Systems in progression order (kids before adult, the same order the rank
	// sort blocks them in), ungraded last.
	want := []string{"bjj-kids", "bjj-adult", RosterFilterUngraded}
	if got := values(options); !slices.Equal(got, want) {
		t.Errorf("options = %v, want %v", got, want)
	}
	// The display name rides along; the ungraded cell is in no system and so has
	// none to show.
	if options[0].System.Name != "BJJ Kids" {
		t.Errorf("first option name = %q, want %q", options[0].System.Name, "BJJ Kids")
	}
	if options[2].System.Name != "" {
		t.Errorf("ungraded option name = %q, want empty", options[2].System.Name)
	}
}

// TestRosterFilterOptionsOmitEmptyCells is the whole point of deriving the
// options from the roster rather than from the seed: a system nobody is in gets
// no chip, so no offered option can yield an empty roster.
func TestRosterFilterOptionsOmitEmptyCells(t *testing.T) {
	rows := []RosterRow{
		partitionRow("Kai", 1, "bjj-kids", "BJJ Kids", 0),
		partitionRow("Kim", 2, "bjj-kids", "BJJ Kids", 0),
	}
	if got := values(rosterFilterOptions(rows)); !slices.Equal(got, []string{"bjj-kids"}) {
		t.Errorf("options for a kids-only roster = %v, want [bjj-kids]", got)
	}
	// An all-ungraded roster offers only that cell — never a system nobody holds.
	ungraded := []RosterRow{partitionRow("Uwe", 0, "", "", 0)}
	if got := values(rosterFilterOptions(ungraded)); !slices.Equal(got, []string{RosterFilterUngraded}) {
		t.Errorf("options for an ungraded roster = %v, want [none]", got)
	}
	if got := rosterFilterOptions(nil); len(got) != 0 {
		t.Errorf("options for an empty roster = %v, want none", got)
	}
}

// TestRosterFilterOptionsBreakASortOrderTieOnTheSlug pins the fallback that keeps
// the chip row from reshuffling between requests: SortFunc is not stable and
// grading_systems.sort_order defaults to 0 (migration 00003), so two systems
// seeded before that column existed are tied and must order by slug instead.
func TestRosterFilterOptionsBreakASortOrderTieOnTheSlug(t *testing.T) {
	rows := []RosterRow{
		partitionRow("Zoe", 1, "zzz-system", "Zzz", 0),
		partitionRow("Ada", 2, "aaa-system", "Aaa", 0),
	}
	want := []string{"aaa-system", "zzz-system"}
	if got := values(rosterFilterOptions(rows)); !slices.Equal(got, want) {
		t.Errorf("options for two systems tied on sort order = %v, want %v", got, want)
	}
}

// TestFilterRosterRestrictsToOneCell checks the filter's whole promise: a system
// filter shows exactly the athletes whose current rank is in it. Ungraded athletes
// are hidden by it — a filter reading "BJJ Kids" that showed athletes in no system
// would not be telling the truth (ADR-0007) — and are reachable under their own
// option.
func TestFilterRosterRestrictsToOneCell(t *testing.T) {
	rows := partitionRows()

	cases := map[string][]string{
		"":                   {"Adam", "Kai", "Uwe", "Kim"},
		"bjj-kids":           {"Kai", "Kim"},
		"bjj-adult":          {"Adam"},
		RosterFilterUngraded: {"Uwe"},
	}
	for option, want := range cases {
		if got := firstNames(filterRoster(rows, option)); !slices.Equal(got, want) {
			t.Errorf("filterRoster(%q) = %v, want %v", option, got, want)
		}
	}
}

// TestFilterRosterKeepsTheOrderItWasGiven pins that filtering does not reorder
// what it restricts — the sort happens in SQL, upstream of this.
func TestFilterRosterKeepsTheOrderItWasGiven(t *testing.T) {
	rows := []RosterRow{
		{Athlete: Athlete{FirstName: "Zoe"}, Rank: Rank{ID: 1, System: System{Slug: "bjj-kids"}}},
		{Athlete: Athlete{FirstName: "Uwe"}},
		{Athlete: Athlete{FirstName: "Ada"}, Rank: Rank{ID: 2, System: System{Slug: "bjj-kids"}}},
	}
	if got := firstNames(filterRoster(rows, "bjj-kids")); !slices.Equal(got, []string{"Zoe", "Ada"}) {
		t.Errorf("filtered order = %v, want [Zoe Ada]", got)
	}
}

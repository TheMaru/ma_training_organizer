package store

import (
	"database/sql"
	"fmt"
	"slices"
	"strings"
)

// Roster sort columns, as they appear in the ?sort= query parameter. They are the
// whitelist keys of rosterOrderBy: anything else falls back to the default, so a
// sort column never reaches the SQL as user input.
const (
	RosterSortLastName  = "lastName"
	RosterSortFirstName = "firstName"
	RosterSortBirthDate = "birthDate"
	RosterSortJoinedOn  = "joinedOn"
	RosterSortRank      = "rank"

	// RosterSortDefault is what an unknown or missing column falls back to: the
	// kids trainer recalls first names more readily than last names (spec).
	RosterSortDefault = RosterSortFirstName
)

// rosterOrderBy maps each sortable column onto the ORDER BY keys it sorts by.
// The rank column has two: (system order, rank order) is the only domain-honest
// cross-system ordering — there is no meaningful "how advanced" comparison
// between a kids and an adult rank (spec, ADR-0001).
var rosterOrderBy = map[string][]string{
	RosterSortLastName:  {"a.last_name"},
	RosterSortFirstName: {"a.first_name"},
	RosterSortBirthDate: {"a.birth_date"},
	RosterSortJoinedOn:  {"a.joined_on"},
	RosterSortRank:      {"cur.system_order", "cur.rank_order"},
}

// rosterTieBreak is the fixed secondary order for every sort: alphabetical by
// first then last name. It never reverses — within "all blue belts" A→Z is more
// natural than Z→A whichever way the primary axis points (spec).
var rosterTieBreak = []string{"a.first_name", "a.last_name"}

// RosterRow is one line of the athlete roster: the athlete plus their derived
// current rank (ADR-0001), joined in SQL rather than derived per row in Go. Rank
// fields are empty/zero for an ungraded athlete, who has no rank at all — that is
// distinct from the lowest rank, which is a graduation.
// Group and Degree are the rank's descriptive breakdown (ADR-0001), carried so
// the roster can render the rank as a belt graphic (ADR-0004) from the same row.
// SystemSlug is the system's stable identity (ADR-0006), carried so the roster
// filter can partition these rows without a second query (ADR-0007); SystemName
// remains the display label.
type RosterRow struct {
	Athlete
	RankID      int64
	RankName    string
	SystemName  string
	SystemSlug  string
	SystemOrder int
	Group       string
	Degree      int
}

// RosterFilterUngraded is the roster filter's value for the athletes who are in
// no grading system at all. It is reserved rather than derived (ADR-0006), so a
// hand-authored slug cannot collide with it.
const RosterFilterUngraded = "none"

// RosterOption is one cell of the roster's partition and one chip in the filter
// row: the value that identifies it in ?system=, and the system's display name —
// empty on the ungraded cell, which is in no system and so has no name of its
// own to show.
type RosterOption struct {
	Value string
	Name  string
}

// RosterFilterOptions returns the cells of the roster's partition that actually
// hold someone: one per grading system present among the current ranks, plus the
// ungraded cell if anyone is ungraded. Callers must pass the *unfiltered* roster
// — deriving the options from an already filtered one would leave only the
// active option standing and delete the way back to Alle (ADR-0007a).
//
// Because every returned option matches at least one of the rows it was derived
// from, no offered filter can produce an empty roster. That is what licenses the
// absence of a zero-hit UI, so it holds by construction: FilterRoster restricts
// the very same slice.
//
// Systems come in progression order (kids before adult, as the rank sort blocks
// them), ungraded last — an order that is a property of the reference data, so
// the chips do not reshuffle when the trainer changes the sort column.
func RosterFilterOptions(rows []RosterRow) []RosterOption {
	var (
		systems  []RosterOption
		orders   = map[string]int{}
		ungraded bool
	)
	for _, row := range rows {
		if row.SystemSlug == "" {
			ungraded = true
			continue
		}
		if _, seen := orders[row.SystemSlug]; seen {
			continue
		}
		orders[row.SystemSlug] = row.SystemOrder
		systems = append(systems, RosterOption{Value: row.SystemSlug, Name: row.SystemName})
	}
	// Systems that share a sort_order fall back to the slug, because SortFunc is
	// not stable and grading_systems.sort_order defaults to 0 (migration 00003):
	// two systems seeded before that column existed would otherwise order at
	// random between requests until the next re-seed.
	slices.SortFunc(systems, func(a, b RosterOption) int {
		if d := orders[a.Value] - orders[b.Value]; d != 0 {
			return d
		}
		return strings.Compare(a.Value, b.Value)
	})
	if ungraded {
		systems = append(systems, RosterOption{Value: RosterFilterUngraded})
	}
	return systems
}

// FilterRoster restricts a roster to one cell of its partition; an empty option
// is Alle. The given order is preserved — the roster is sorted in SQL, and
// narrowing it is not allowed to reorder it.
//
// An option nobody holds yields nothing; the roster handler keeps that state
// unreachable by offering only RosterFilterOptions and falling back to Alle for
// anything else.
func FilterRoster(rows []RosterRow, option string) []RosterRow {
	if option == "" {
		return rows
	}
	slug := option
	if option == RosterFilterUngraded {
		slug = ""
	}
	filtered := make([]RosterRow, 0, len(rows))
	for _, row := range rows {
		if row.SystemSlug == slug {
			filtered = append(filtered, row)
		}
	}
	return filtered
}

// NormalizeRosterSort maps a requested sort column onto the whitelist, returning
// RosterSortDefault for anything unknown. Callers use it to learn which column is
// actually active (for the header indicator) before passing it to ListRoster.
func NormalizeRosterSort(sort string) string {
	if _, ok := rosterOrderBy[sort]; ok {
		return sort
	}
	return RosterSortDefault
}

// ListRoster returns the whole roster with each athlete's current rank, ordered
// by the named column. An unknown column silently sorts by RosterSortDefault:
// sorting is a view concern, not a data error. descending reverses the primary
// axis only; the first/last-name tie-break stays ascending.
//
// The current rank comes from a window function that ranks each athlete's
// promotions by date (higher id winning a tied date) and keeps the first — the
// same rule CurrentRank applies in Go, pinned to it by a test. The LEFT JOIN
// keeps ungraded athletes on the roster with NULL rank keys, which NULLS LAST
// then sorts to the end in both directions. Window functions and NULLS LAST are
// standard SQL, so the Postgres escape hatch stays open (ADR-0002).
func ListRoster(db *sql.DB, sort string, descending bool) ([]RosterRow, error) {
	rows, err := db.Query(fmt.Sprintf(`
		SELECT a.id, a.first_name, a.last_name, a.birth_date, a.joined_on, a.notes,
		       cur.rank_id, cur.rank_name, cur.system_name, cur.system_slug,
		       cur.system_order, cur.rank_group, cur.degree
		FROM athletes a
		LEFT JOIN (
			SELECT p.athlete_id, p.rank_id, r.name AS rank_name, g.name AS system_name,
			       g.slug AS system_slug, r.rank_group, r.degree,
			       g.sort_order AS system_order, r.sort_order AS rank_order,
			       ROW_NUMBER() OVER (
			           PARTITION BY p.athlete_id ORDER BY p.promoted_on DESC, p.id DESC
			       ) AS recency
			FROM promotions p
			JOIN ranks r ON r.id = p.rank_id
			JOIN grading_systems g ON g.id = r.grading_system_id
		) cur ON cur.athlete_id = a.id AND cur.recency = 1
		ORDER BY %s`, rosterOrderClause(sort, descending)))
	if err != nil {
		return nil, fmt.Errorf("list roster: %w", err)
	}
	defer rows.Close()

	var roster []RosterRow
	for rows.Next() {
		row, err := scanRosterRow(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("scan roster row: %w", err)
		}
		roster = append(roster, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate roster: %w", err)
	}
	return roster, nil
}

// rosterOrderClause builds the ORDER BY body for one column and direction. Both
// the column keys and the direction come from fixed sets (the rosterOrderBy
// whitelist and the two literals), so interpolating them is injection-safe.
// A tie-break key that is already the primary key is dropped as redundant.
func rosterOrderClause(sort string, descending bool) string {
	dir := "ASC"
	if descending {
		dir = "DESC"
	}
	primary := rosterOrderBy[NormalizeRosterSort(sort)]

	keys := make([]string, 0, len(primary)+len(rosterTieBreak))
	for _, key := range primary {
		// NULLS LAST keeps rows without a value — ungraded athletes, blank dates —
		// at the end whichever way the sort points.
		keys = append(keys, key+" "+dir+" NULLS LAST")
	}
	for _, key := range rosterTieBreak {
		if !slices.Contains(primary, key) {
			keys = append(keys, key+" ASC")
		}
	}
	return strings.Join(keys, ", ")
}

// scanRosterRow maps one joined row into a RosterRow. The rank columns are
// nullable (LEFT JOIN, ungraded athlete), so they scan through Null wrappers and
// land as zero values.
func scanRosterRow(scan func(dest ...any) error) (RosterRow, error) {
	var (
		row                                     RosterRow
		birth, joined                           sql.NullTime
		rankID, systemOrder, degree             sql.NullInt64
		rankName, systemName, systemSlug, group sql.NullString
	)
	dest := append(athleteDest(&row.Athlete, &birth, &joined),
		&rankID, &rankName, &systemName, &systemSlug, &systemOrder, &group, &degree)
	if err := scan(dest...); err != nil {
		return RosterRow{}, err
	}
	row.BirthDate = formatDate(birth)
	row.JoinedOn = formatDate(joined)
	row.RankID = rankID.Int64
	row.RankName = rankName.String
	row.SystemName = systemName.String
	row.SystemSlug = systemSlug.String
	row.SystemOrder = int(systemOrder.Int64)
	row.Group = group.String
	row.Degree = int(degree.Int64)
	return row, nil
}

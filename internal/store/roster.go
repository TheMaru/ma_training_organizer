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
type RosterRow struct {
	Athlete
	RankID     int64
	RankName   string
	SystemName string
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
		       cur.rank_id, cur.rank_name, cur.system_name
		FROM athletes a
		LEFT JOIN (
			SELECT p.athlete_id, p.rank_id, r.name AS rank_name, g.name AS system_name,
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
		row                  RosterRow
		birth, joined        sql.NullTime
		rankID               sql.NullInt64
		rankName, systemName sql.NullString
	)
	dest := append(athleteDest(&row.Athlete, &birth, &joined), &rankID, &rankName, &systemName)
	if err := scan(dest...); err != nil {
		return RosterRow{}, err
	}
	row.BirthDate = formatDate(birth)
	row.JoinedOn = formatDate(joined)
	row.RankID = rankID.Int64
	row.RankName = rankName.String
	row.SystemName = systemName.String
	return row, nil
}

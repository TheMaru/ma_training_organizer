package store

import (
	"database/sql"
	"fmt"
	"slices"
	"strings"
)

// Roster sort columns, as they appear in the ?sort= query parameter.
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

// rosterOrderBy is the sort whitelist, mapping each column onto its ORDER BY keys.
// The rank column takes two: (system order, rank order) is the only domain-honest
// cross-system ordering — there is no meaningful "how advanced" comparison
// between a kids and an adult rank (spec, ADR-0001).
var rosterOrderBy = map[string][]string{
	RosterSortLastName:  {"a.last_name"},
	RosterSortFirstName: {"a.first_name"},
	RosterSortBirthDate: {"a.birth_date"},
	RosterSortJoinedOn:  {"a.joined_on"},
	RosterSortRank:      {"cur.system_order", "cur.rank_order"},
}

// rosterTieBreak is the fixed secondary order for every sort. It never reverses —
// within "all blue belts" A→Z is more natural than Z→A whichever way the primary
// axis points (spec).
var rosterTieBreak = []string{"a.first_name", "a.last_name"}

// RosterRow is one line of the athlete roster: the athlete plus their derived
// current rank (ADR-0001), joined in SQL rather than derived per row in Go. Rank
// is the zero Rank for an ungraded athlete, who has no rank at all — that is
// distinct from the lowest rank, which is a graduation.
//
// SystemOrder is the system's sort key and only orders the filter's
// chips, so it stays off the Rank, which is a value for display.
type RosterRow struct {
	Athlete
	Rank        Rank
	SystemOrder int
}

// Ungraded reports whether the athlete is in no grading system at all
// (CONTEXT.md) — distinct from holding the lowest rank, which is a graduation.
func (r RosterRow) Ungraded() bool {
	return r.Rank.IsZero()
}

// RosterFilterUngraded is the roster filter's value for the athletes who are in
// no grading system at all. It is reserved rather than derived (ADR-0006), so a
// hand-authored slug cannot collide with it.
const RosterFilterUngraded = "none"

// RosterOption is one cell of the roster's partition and one chip in the filter
// row: the value that identifies it in ?system=, and the system it stands for —
// zero on the ungraded cell, which is in no system.
type RosterOption struct {
	Value  string
	System System
}

// RosterQuery is what a caller asks of the roster: the view of it they want. The
// same three fields come back resolved in RosterView.Query, so a caller that
// renders links renders them from the answer rather than from the question.
//
// Resolving narrows, it never substitutes: the resolved Sort is a whitelist value
// and the resolved Filter is either empty or exactly the one that was asked. So a
// caller that has already checked a value's form does not have to check it again on
// the way back out.
type RosterQuery struct {
	Sort       string // a NormalizeRosterSort value; anything else normalises
	Descending bool
	Filter     string // "" = all, RosterFilterUngraded, or a system slug
}

// RosterView is one view of the roster: the rows to show, the filter options the
// roster actually offers, and the query that produced them.
//
// Rows is already filtered. Options is derived from the *unfiltered* roster, so
// the way back to Alle never disappears (ADR-0007a). Query is resolved — a filter
// nobody is in has already fallen back to Alle. The unfiltered slice itself does
// not leave this module, which is what makes those three statements invariants
// rather than a protocol the caller has to obey.
type RosterView struct {
	Query   RosterQuery
	Rows    []RosterRow
	Options []RosterOption
}

// LoadRoster answers one roster query. It is the only way in: it fetches the
// roster once and derives the options, the shown rows and the resolved filter
// from that single slice, so there is nothing for a second source to drift from
// (ADR-0007b).
func LoadRoster(db *sql.DB, query RosterQuery) (RosterView, error) {
	rows, err := listRoster(db, query.Sort, query.Descending)
	if err != nil {
		return RosterView{}, err
	}
	options := rosterFilterOptions(rows)
	resolved := RosterQuery{
		Sort:       NormalizeRosterSort(query.Sort),
		Descending: query.Descending,
		Filter:     representedFilter(query.Filter, options),
	}
	return RosterView{
		Query:   resolved,
		Rows:    filterRoster(rows, resolved.Filter),
		Options: options,
	}, nil
}

// representedFilter keeps a filter only if the roster actually offers it. A
// bookmarked system the last athlete has since left is not a data error, it is a
// view that no longer exists, so it resolves to Alle — in the spirit of
// NormalizeRosterSort, where a column nobody can sort by is a view concern too.
//
// A kept filter is returned as given rather than as the matching option's value,
// which is RosterQuery's promise that resolving never substitutes.
func representedFilter(filter string, options []RosterOption) string {
	offered := slices.ContainsFunc(options, func(o RosterOption) bool {
		return o.Value == filter
	})
	if offered {
		return filter
	}
	return ""
}

// rosterFilterOptions returns the cells of the roster's partition that actually
// hold someone: one per slugged grading system present among the current ranks,
// plus the ungraded cell if anyone is ungraded. It is given the *unfiltered*
// roster, or only the active option would be left standing and the way back to
// Alle would disappear (ADR-0007a).
//
// Because every returned option matches at least one of the rows it was derived
// from, no offered filter can produce an empty roster. That is what licenses the
// absence of a zero-hit UI, and it holds by construction: LoadRoster hands
// filterRoster the very same slice.
//
// Systems come in progression order (kids before adult, as the rank sort blocks
// them), ungraded last — an order that is a property of the reference data, so
// the chips do not reshuffle when the trainer changes the sort column.
func rosterFilterOptions(rows []RosterRow) []RosterOption {
	var (
		systems  []RosterOption
		orders   = map[string]int{}
		ungraded bool
	)
	for _, row := range rows {
		if row.Ungraded() {
			ungraded = true
			continue
		}
		// A system without a slug has no identity in a URL (ADR-0006), and the empty
		// value already means Alle, so it can carry no chip of its own
		// (ADR-0007, Update 2026-09-01).
		system := row.Rank.System
		if system.Slug == "" {
			continue
		}
		if _, seen := orders[system.Slug]; seen {
			continue
		}
		orders[system.Slug] = row.SystemOrder
		systems = append(systems, RosterOption{Value: system.Slug, System: system})
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

// filterRoster restricts a roster to one cell of its partition; an empty option
// is Alle. The given order is preserved — the roster is sorted in SQL, and
// narrowing it is not allowed to reorder it.
//
// An option nobody holds yields nothing; see rosterFilterOptions for why no
// offered option can reach that state.
func filterRoster(rows []RosterRow, option string) []RosterRow {
	if option == "" {
		return rows
	}
	filtered := make([]RosterRow, 0, len(rows))
	for _, row := range rows {
		if inRosterCell(row, option) {
			filtered = append(filtered, row)
		}
	}
	return filtered
}

// Every cell other than the ungraded one is a system, and a system is its slug
// (ADR-0006).
func inRosterCell(row RosterRow, option string) bool {
	if option == RosterFilterUngraded {
		return row.Ungraded()
	}
	return row.Rank.System.Slug == option
}

// NormalizeRosterSort maps a requested sort column onto the whitelist, returning
// RosterSortDefault for anything unknown.
func NormalizeRosterSort(sort string) string {
	if _, ok := rosterOrderBy[sort]; ok {
		return sort
	}
	return RosterSortDefault
}

// listRoster returns the whole roster with each athlete's current rank, ordered
// by the named column. An unknown column silently sorts by RosterSortDefault:
// sorting is a view concern, not a data error.
//
// The window function applies the same recency rule as CurrentRank does in Go,
// pinned to it by a test. The LEFT JOIN keeps ungraded athletes on the roster with
// NULL rank keys. Both the window function and NULLS LAST are standard SQL, so the
// Postgres escape hatch stays open (ADR-0002).
func listRoster(db *sql.DB, sort string, descending bool) ([]RosterRow, error) {
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
	row.Rank = Rank{
		ID:     rankID.Int64,
		Name:   rankName.String,
		Group:  group.String,
		Degree: int(degree.Int64),
		System: System{Name: systemName.String, Slug: systemSlug.String},
	}
	row.SystemOrder = int(systemOrder.Int64)
	return row, nil
}

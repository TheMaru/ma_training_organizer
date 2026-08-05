package web

import (
	"net/http"
	"slices"

	"github.com/TheMaru/ma_training_organizer/internal/i18n"
	"github.com/TheMaru/ma_training_organizer/internal/store"
)

// rosterColumns are the sortable roster columns in display order: the catalog key
// of the header a trainer clicks, and the store sort key it selects. The actions
// column is not in here — it holds no data to sort by.
//
// The label is a key, not text: athletes.html repeats these same keys in each
// cell's data-label, which is what keeps the phone cards labelled exactly like
// the desktop headers they replace (ADR-0008).
var rosterColumns = []struct {
	Key      string
	LabelKey string
}{
	{store.RosterSortLastName, "roster.column.lastName"},
	{store.RosterSortFirstName, "roster.column.firstName"},
	{store.RosterSortBirthDate, "roster.column.birthDate"},
	{store.RosterSortJoinedOn, "roster.column.joinedOn"},
	{store.RosterSortRank, "roster.column.rank"},
}

// filterAllKey is the chip for the unfiltered roster, and filterUngradedKey the
// one for the athletes who are in no grading system at all (CONTEXT.md's
// Ungraded).
const (
	filterAllKey      = "roster.filter.all"
	filterUngradedKey = "roster.filter.ungraded"
)

// rosterHeader is one rendered column header: the label, where its link points
// next, and the direction it currently sorts in (empty on inactive columns). Both
// sort surfaces render from this — the desktop <th> links and the phone chips
// (ADR-0005) — so Active is a field rather than each surface inferring it from
// one of the two presentation strings.
type rosterHeader struct {
	Label      string
	Href       string
	Active     bool   // the column the roster is currently sorted by
	Descending bool   // its direction, meaningful while Active
	Indicator  string // ▲/▼, on the active column only
	AriaSort   string // ascending/descending/none; valid on a columnheader only
}

// rosterLine is one rendered roster row: the stored row plus the links leading
// out of it. Those links are built from the query, so opening or deleting an
// athlete comes back to the roster the trainer was actually looking at.
type rosterLine struct {
	store.RosterRow
	Href         string // the athlete's detail page
	DeleteAction string
}

// rosterFilter is one rendered filter chip: a cell of the roster's partition (or
// Alle, their union) and the roster it narrows to. Only one is ever Active, so
// the unfiltered roster is a selection like any other rather than the absence of
// one.
type rosterFilter struct {
	Label  string
	Href   string
	Active bool
}

// handleAthletesList renders the shared roster, sorted server-side by
// ?sort=<col>&dir=<asc|desc> and narrowed by ?system=<slug|none>.
//
// This is the one handler that knows which filters are *represented* (ADR-0007b):
// it loads the roster unfiltered once and derives both the options and the shown
// rows from that same slice, which is what store.RosterFilterOptions asks of its
// caller and what the absent empty-result state rests on.
func (s *Server) handleAthletesList(w http.ResponseWriter, r *http.Request) {
	query := rosterQueryFrom(r)

	athletes, err := store.ListRoster(s.db, query.sort, query.descending)
	if err != nil {
		serverError(w)
		return
	}
	options := store.RosterFilterOptions(athletes)
	// Resolved before anything builds a URL, so a filter nobody is in cannot
	// survive into the page's links either.
	query.system = representedFilter(query.system, options)
	locale := localeOf(r.Context())

	s.tmpl.render(w, r, http.StatusOK, "athletes.html", map[string]any{
		"Authenticated": true,
		"Athletes":      rosterLines(store.FilterRoster(athletes, query.system), query),
		"Headers":       rosterHeaders(locale, query),
		"Filters":       rosterFilters(locale, query, options),
		"NewHref":       query.path(rosterPath + "/new"),
		// The one page whose canonical URL the request cannot supply: a filter nobody
		// is in was just resolved away, and only the query built here knows that.
		"Return": query.path(rosterPath),
	})
}

// representedFilter keeps a filter only if the roster actually offers it. A
// bookmarked system the last athlete has since left is not a data error, it is a
// view that no longer exists, so it resolves to Alle.
func representedFilter(system string, options []store.RosterOption) string {
	offered := slices.ContainsFunc(options, func(o store.RosterOption) bool {
		return o.Value == system
	})
	if offered {
		return system
	}
	return ""
}

// rosterFilters builds the filter chips: Alle followed by one per non-empty cell
// of the partition. Below two options it builds none — Alle and a single option
// would show the same roster, so in a homogeneous roster the row is absent and
// surfaces by itself once there is something to partition (ADR-0007a).
func rosterFilters(locale i18n.Locale, query rosterQuery, options []store.RosterOption) []rosterFilter {
	if len(options) < 2 {
		return nil
	}
	chips := make([]rosterFilter, 0, len(options)+1)
	chip := func(value, label string) {
		chips = append(chips, rosterFilter{
			Label:  label,
			Href:   query.filteredBy(value).path(rosterPath),
			Active: value == query.system,
		})
	}
	chip("", i18n.T(locale, filterAllKey))
	for _, option := range options {
		chip(option.Value, filterLabel(locale, option))
	}
	return chips
}

// filterLabel names one option's chip: the ungraded cell has a label of its own,
// since it is in no system and so has no name to render, and a system cell carries
// the same localized name as the rows it filters. The option's value is the slug
// the label is keyed on (ADR-0009).
func filterLabel(locale i18n.Locale, option store.RosterOption) string {
	if option.Value == store.RosterFilterUngraded {
		return i18n.T(locale, filterUngradedKey)
	}
	return systemLabel(locale, option.Value, option.Name)
}

func rosterLines(rows []store.RosterRow, query rosterQuery) []rosterLine {
	lines := make([]rosterLine, 0, len(rows))
	for _, row := range rows {
		lines = append(lines, rosterLine{
			RosterRow:    row,
			Href:         query.path(athletePath(row.ID)),
			DeleteAction: query.path(deletePath(row.ID)),
		})
	}
	return lines
}

// rosterHeaders builds the clickable column headers for the active query. Each
// links to the query rosterQuery.sortedBy produces, which is where the rule for
// what a click does lives.
func rosterHeaders(locale i18n.Locale, query rosterQuery) []rosterHeader {
	headers := make([]rosterHeader, 0, len(rosterColumns))
	for _, col := range rosterColumns {
		h := rosterHeader{
			Label:    i18n.T(locale, col.LabelKey),
			AriaSort: "none",
			Href:     query.sortedBy(col.Key).path(rosterPath),
		}
		if col.Key == query.sort {
			h.Active, h.Descending = true, query.descending
			if query.descending {
				h.Indicator, h.AriaSort = "▼", "descending"
			} else {
				h.Indicator, h.AriaSort = "▲", "ascending"
			}
		}
		headers = append(headers, h)
	}
	return headers
}

package web

import (
	"net/http"

	"github.com/TheMaru/ma_training_organizer/internal/i18n"
	"github.com/TheMaru/ma_training_organizer/internal/rankview"
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

// rosterPage is everything athletes.html renders, built from one store answer.
type rosterPage struct {
	Athletes []rosterLine
	Headers  []rosterHeader
	Filters  []rosterFilter
	NewHref  string
	// Return is the one canonical URL the request cannot supply: a filter nobody
	// is in was resolved away in the store, and only the resolved query knows that.
	Return string
}

// rosterPageOf links everything from the resolved query; see store.RosterView for
// what "resolved" guarantees.
func rosterPageOf(locale i18n.Locale, view store.RosterView) rosterPage {
	query := rosterQueryOf(view.Query)
	return rosterPage{
		Athletes: rosterLines(view.Rows, query),
		Headers:  rosterHeaders(locale, query),
		Filters:  rosterFilters(locale, query, view.Options),
		NewHref:  query.path(rosterPath + "/new"),
		Return:   query.path(rosterPath),
	}
}

// handleAthletesList renders the shared roster, sorted server-side by
// ?sort=<col>&dir=<asc|desc> and narrowed by ?system=<slug|none>.
//
// Which filters are *represented* is settled inside store.LoadRoster (ADR-0007b),
// so this handler renders an answer rather than assembling one.
func (s *Server) handleAthletesList(w http.ResponseWriter, r *http.Request) {
	view, err := store.LoadRoster(s.db, rosterQueryFrom(r).storeQuery())
	if err != nil {
		serverError(w)
		return
	}
	page := rosterPageOf(localeOf(r.Context()), view)
	s.tmpl.render(w, r, http.StatusOK, "athletes.html", map[string]any{
		"Authenticated": true,
		"Roster":        page,
		// base.html reads Return at the top level, and renderer.render keeps it there.
		"Return": page.Return,
	})
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
// the same localized name as the rows it filters (ADR-0009).
func filterLabel(locale i18n.Locale, option store.RosterOption) string {
	if option.Value == store.RosterFilterUngraded {
		return i18n.T(locale, filterUngradedKey)
	}
	return rankview.SystemName(locale, option.System)
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

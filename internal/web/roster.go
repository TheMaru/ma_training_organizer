package web

import (
	"net/http"

	"github.com/TheMaru/ma_training_organizer/internal/store"
)

// rosterColumns are the sortable roster columns in display order: the German
// header label a trainer clicks, and the store sort key it selects. The actions
// column is not in here — it holds no data to sort by.
var rosterColumns = []struct {
	Key   string
	Label string
}{
	{store.RosterSortLastName, "Nachname"},
	{store.RosterSortFirstName, "Vorname"},
	{store.RosterSortBirthDate, "Geburtsdatum"},
	{store.RosterSortJoinedOn, "Eintritt"},
	{store.RosterSortRank, "Aktueller Rang"},
}

// rosterHeader is one rendered column header: the label, where its link points
// next, and the direction it currently sorts in (empty on inactive columns).
type rosterHeader struct {
	Label     string
	Href      string
	Indicator string // ▲/▼, on the active column only
	AriaSort  string // ascending/descending/none, for assistive tech
}

// rosterLine is one rendered roster row: the stored row plus the links leading
// out of it. Those links are built from the view state, so opening or deleting an
// athlete comes back to the roster the trainer was actually looking at.
type rosterLine struct {
	store.RosterRow
	Href         string // the athlete's detail page
	DeleteAction string
}

// handleAthletesList renders the shared roster, sorted server-side by
// ?sort=<col>&dir=<asc|desc>. Unknown values fall back to the default view
// (Vorname ascending) rather than erroring: sorting is a view concern.
func (s *Server) handleAthletesList(w http.ResponseWriter, r *http.Request) {
	view := rosterViewFrom(r)

	athletes, err := store.ListRoster(s.db, view.sort, view.descending)
	if err != nil {
		serverError(w)
		return
	}

	s.tmpl.render(w, http.StatusOK, "athletes.html", map[string]any{
		"Authenticated": true,
		"Athletes":      rosterLines(athletes, view),
		"Headers":       rosterHeaders(view),
		"NewHref":       view.path(rosterPath + "/new"),
	})
}

// rosterLines pairs each roster row with its outgoing links under the given view.
func rosterLines(rows []store.RosterRow, view rosterView) []rosterLine {
	lines := make([]rosterLine, 0, len(rows))
	for _, row := range rows {
		lines = append(lines, rosterLine{
			RosterRow:    row,
			Href:         view.path(athletePath(row.ID)),
			DeleteAction: view.path(deletePath(row.ID)),
		})
	}
	return lines
}

// rosterHeaders builds the clickable column headers for the active view. Every
// column links to itself ascending — a first click always sorts A→Z — except the
// active one, whose link flips the direction so a click toggles it. Only the
// active column carries an indicator.
func rosterHeaders(view rosterView) []rosterHeader {
	headers := make([]rosterHeader, 0, len(rosterColumns))
	for _, col := range rosterColumns {
		h := rosterHeader{
			Label:    col.Label,
			AriaSort: "none",
			Href:     view.sortedBy(col.Key).path(rosterPath),
		}
		if col.Key == view.sort {
			if view.descending {
				h.Indicator, h.AriaSort = "▼", "descending"
			} else {
				h.Indicator, h.AriaSort = "▲", "ascending"
			}
		}
		headers = append(headers, h)
	}
	return headers
}

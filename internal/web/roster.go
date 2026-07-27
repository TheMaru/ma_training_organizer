package web

import (
	"fmt"
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

// handleAthletesList renders the shared roster, sorted server-side by
// ?sort=<col>&dir=<asc|desc>. Unknown values fall back to the default view
// (Vorname ascending) rather than erroring: sorting is a view concern.
func (s *Server) handleAthletesList(w http.ResponseWriter, r *http.Request) {
	sort := store.NormalizeRosterSort(r.URL.Query().Get("sort"))
	descending := r.URL.Query().Get("dir") == "desc"

	athletes, err := store.ListRoster(s.db, sort, descending)
	if err != nil {
		serverError(w)
		return
	}

	s.tmpl.render(w, http.StatusOK, "athletes.html", map[string]any{
		"Authenticated": true,
		"Athletes":      athletes,
		"Headers":       rosterHeaders(sort, descending),
	})
}

// rosterHeaders builds the clickable column headers for the active sort. Every
// column links to itself ascending — a first click always sorts A→Z — except the
// active one, whose link flips the direction so a click toggles it. Only the
// active column carries an indicator.
func rosterHeaders(sort string, descending bool) []rosterHeader {
	headers := make([]rosterHeader, 0, len(rosterColumns))
	for _, col := range rosterColumns {
		h := rosterHeader{Label: col.Label, AriaSort: "none"}
		next := "asc"
		if col.Key == sort {
			if descending {
				h.Indicator, h.AriaSort = "▼", "descending"
			} else {
				h.Indicator, h.AriaSort, next = "▲", "ascending", "desc"
			}
		}
		h.Href = fmt.Sprintf("/athletes?sort=%s&dir=%s", col.Key, next)
		headers = append(headers, h)
	}
	return headers
}

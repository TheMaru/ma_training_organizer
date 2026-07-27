package web

import (
	"net/http"
	"strings"

	"github.com/TheMaru/ma_training_organizer/internal/store"
)

// The two roster sort directions, as they appear in the ?dir= query parameter.
// Together with store.NormalizeRosterSort they are the only values that may end
// up in a rendered URL or a Location header.
const (
	dirAscending  = "asc"
	dirDescending = "desc"
)

// rosterView is the roster's resolved view state: everything the roster's URL
// says about what the trainer is looking at, already normalised. It is the single
// carrier of that state — the sort controls, every link out of the roster and
// every mutation redirect build their target from one of these, so the state
// travels by construction instead of each call site remembering it. Adding a key
// ([[roster-cohort-filter]]'s ?system=) touches this file only.
type rosterView struct {
	sort       string // a store.NormalizeRosterSort value
	descending bool
}

// defaultRosterView is what a bare /athletes renders: Vorname ascending.
var defaultRosterView = rosterView{sort: store.RosterSortDefault}

// rosterViewFrom resolves the view state from one request's own query string —
// every handler reads it from the request it is answering, so a mutation knows
// the roster it was triggered from. This is the whitelist: an unknown column
// falls back to the default and any direction but "desc" reads as ascending, so
// no user-controlled string survives into a rendered URL or a Location header.
func rosterViewFrom(r *http.Request) rosterView {
	return rosterView{
		sort:       store.NormalizeRosterSort(r.URL.Query().Get("sort")),
		descending: r.URL.Query().Get("dir") == dirDescending,
	}
}

// sortedBy returns the view state a click on the given column produces: a column
// sorts ascending on the first click and only flips while it is already active,
// so clicking the active column toggles its direction. Everything else about the
// view — the parked cohort filter next — is carried over untouched. The column
// runs through the same whitelist as an incoming request, so building a view
// state here cannot get around it.
func (v rosterView) sortedBy(column string) rosterView {
	next := v
	next.sort = store.NormalizeRosterSort(column)
	next.descending = next.sort == v.sort && !v.descending
	return next
}

// path appends the view state to a path, yielding a URL that restores the view.
// The default view carries no query at all: a trainer who never sorted keeps the
// plain URLs, and a request whose parameters were junk normalises to a redirect
// target with nothing in it.
func (v rosterView) path(p string) string {
	query := v.query()
	if query == "" {
		return p
	}
	return p + "?" + query
}

// query renders the view state as query parameters, empty for the default view.
// The keys are listed in a fixed order (url.Values.Encode would sort them
// alphabetically instead) and every value comes from a fixed set, so the result
// never needs escaping — see rosterViewFrom.
func (v rosterView) query() string {
	if v == defaultRosterView {
		return ""
	}
	return strings.Join([]string{
		"sort=" + v.sort,
		"dir=" + v.dir(),
	}, "&")
}

// dir is the view's direction as it appears in the URL.
func (v rosterView) dir() string {
	if v.descending {
		return dirDescending
	}
	return dirAscending
}

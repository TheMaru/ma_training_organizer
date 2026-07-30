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

// maxSystemSlugLen bounds the ?system= value. Slugs are hand-authored in the
// seed and far shorter than this; the limit is here so a well-formed but absurd
// value cannot be carried around.
const maxSystemSlugLen = 64

// rosterView is the roster's resolved view state: everything the roster's URL
// says about what the trainer is looking at, already normalised. It is the single
// carrier of that state — the sort controls, every link out of the roster and
// every mutation redirect build their target from one of these, so the state
// travels by construction instead of each call site remembering it.
//
// The struct stays comparable, which query() relies on to spot the default view.
// That is also why the filter is a string and not a system id: an integer would
// collapse "Alle" and "ungraded" onto the same zero value, and the zero value has
// to be exactly the default view.
type rosterView struct {
	sort       string // a store.NormalizeRosterSort value
	descending bool
	system     string // "" = all, store.RosterFilterUngraded, or a slug-shaped value
}

// defaultRosterView is what a bare /athletes renders: Vorname ascending.
var defaultRosterView = rosterView{sort: store.RosterSortDefault}

// rosterViewFrom resolves the view state from one request's own query string —
// every handler reads it from the request it is answering, so a mutation knows
// the roster it was triggered from. This is the whitelist: an unknown column
// falls back to the default and any direction but "desc" reads as ascending, so
// no user-controlled string survives into a rendered URL or a Location header.
//
// The filter is checked for its *form* only, and this function stays pure and
// database-free (ADR-0007b): whether a slug is well-formed is a property of the
// URL, while whether anyone is actually in that system is a property of the data
// that only the roster handler needs and only it looks up. The eight other call
// sites carry the value through untouched.
func rosterViewFrom(r *http.Request) rosterView {
	return rosterView{
		sort:       store.NormalizeRosterSort(r.URL.Query().Get("sort")),
		descending: r.URL.Query().Get("dir") == dirDescending,
		system:     normalizeSystemFilter(r.URL.Query().Get("system")),
	}
}

// normalizeSystemFilter keeps a requested filter only if it is slug-shaped, which
// store.RosterFilterUngraded ("none") is too. Anything else — a display name, a
// header injection attempt, an unbounded string — reads as no filter, which is
// the default view rather than an error.
func normalizeSystemFilter(system string) string {
	if system == "" || len(system) > maxSystemSlugLen {
		return ""
	}
	for _, c := range system {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
			return ""
		}
	}
	return system
}

// sortedBy returns the view state a click on the given column produces: a column
// sorts ascending on the first click and only flips while it is already active,
// so clicking the active column toggles its direction. Everything else about the
// view — the filter — is carried over untouched, so sorting never widens it. The column
// runs through the same whitelist as an incoming request, so building a view
// state here cannot get around it.
func (v rosterView) sortedBy(column string) rosterView {
	next := v
	next.sort = store.NormalizeRosterSort(column)
	next.descending = next.sort == v.sort && !v.descending
	return next
}

// filteredBy returns the view state a click on a filter chip produces: the
// roster narrowed to that option, ordered exactly as it is now. An empty option
// is the Alle chip, which widens the roster without disturbing the sort.
func (v rosterView) filteredBy(option string) rosterView {
	next := v
	next.system = normalizeSystemFilter(option)
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
// alphabetically instead) and every value has a fixed shape — a whitelisted
// column, one of two directions, or a value made of [a-z0-9-] — so the result
// never needs escaping. Shape rather than set is the weaker-sounding but equally
// sufficient guarantee: those characters rule out escaping and header injection
// just as a closed set does. See rosterViewFrom.
//
// An absent filter renders no key at all, so it stays part of the default view
// and a bare /athletes keeps working.
func (v rosterView) query() string {
	if v == defaultRosterView {
		return ""
	}
	keys := []string{
		"sort=" + v.sort,
		"dir=" + v.dir(),
	}
	if v.system != "" {
		keys = append(keys, "system="+v.system)
	}
	return strings.Join(keys, "&")
}

// dir is the view's direction as it appears in the URL.
func (v rosterView) dir() string {
	if v.descending {
		return dirDescending
	}
	return dirAscending
}

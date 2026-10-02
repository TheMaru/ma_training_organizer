package web

import (
	"net/http"
	"strings"

	"github.com/TheMaru/ma_training_organizer/internal/store"
)

// The two roster sort directions, as they appear in the ?dir= query parameter.
const (
	dirAscending  = "asc"
	dirDescending = "desc"
)

// maxSystemSlugLen bounds the ?system= value. Slugs are hand-authored in the
// seed and far shorter than this; the limit is here so a well-formed but absurd
// value cannot be carried around.
const maxSystemSlugLen = 64

// rosterQuery is what a trainer's URL asks of the roster: everything it says
// about the view they want, already normalised. It is the single carrier of that
// state, so the state travels by construction instead of each call site
// remembering it.
//
// The struct stays comparable, which params() relies on to spot the default
// query. That is also why the filter is a string and not a system id: an integer
// would collapse "Alle" and "ungraded" onto the same zero value, and the zero
// value has to be exactly the default query.
type rosterQuery struct {
	sort       string // a store.NormalizeRosterSort value
	descending bool
	system     string // "" = all, store.RosterFilterUngraded, or a slug-shaped value
}

// defaultRosterQuery is what a bare /athletes renders: Vorname ascending.
var defaultRosterQuery = rosterQuery{sort: store.RosterSortDefault}

// rosterQueryFrom resolves the query from one request's own query string, so a
// mutation knows the roster it was triggered from. Only the edges that turn the
// query into a URL call it — renderAthleteForm, renderAthleteDetail,
// redirectWithinRoster, returnTarget and the roster handler — so no handler has
// to remember to carry it. This is the whitelist: no user-controlled string
// survives it into a rendered URL or a Location header.
//
// The filter is checked for its *form* only, and this function stays pure and
// database-free: whether anyone is actually in a given system is the roster
// handler's question, not the URL's (ADR-0007b).
func rosterQueryFrom(r *http.Request) rosterQuery {
	return rosterQuery{
		sort:       store.NormalizeRosterSort(r.URL.Query().Get("sort")),
		descending: r.URL.Query().Get("dir") == dirDescending,
		system:     normalizeSystemFilter(r.URL.Query().Get("system")),
	}
}

// redirectWithinRoster returns the trainer to the sorted, filtered view a roster
// mutation came from. redirect itself appends nothing: /login and / must carry no
// roster query, and the language switcher's target already carries its own.
func redirectWithinRoster(w http.ResponseWriter, r *http.Request, bare string) {
	redirect(w, r, rosterQueryFrom(r).path(bare))
}

// storeQuery is this query as the roster takes it. The two types carry the same
// three fields on purpose: this one is what a URL asks, store.RosterQuery is what
// a caller asks of the roster, and neither package has to speak the other's
// vocabulary to name a view.
func (q rosterQuery) storeQuery() store.RosterQuery {
	return store.RosterQuery{Sort: q.sort, Descending: q.descending, Filter: q.system}
}

// rosterQueryOf carries the roster's resolved query back into the URL vocabulary.
// Nothing is re-checked on the way back: store.RosterQuery documents the resolved
// filter as either empty or the very value that was asked, and that value came
// through rosterQueryFrom. Re-normalising it here would be the one way the rendered
// links could disagree with the rows the roster actually returned.
func rosterQueryOf(q store.RosterQuery) rosterQuery {
	return rosterQuery{sort: q.Sort, descending: q.Descending, system: q.Filter}
}

// normalizeSystemFilter keeps a requested filter only if it is slug-shaped, which
// store.RosterFilterUngraded ("none") is too. Anything else reads as no filter —
// the default query rather than an error.
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

// sortedBy returns the query a click on the given column produces: a column
// sorts ascending on the first click and only flips while it is already active.
// The filter carries over untouched, so sorting never widens the roster.
func (q rosterQuery) sortedBy(column string) rosterQuery {
	next := q
	next.sort = store.NormalizeRosterSort(column)
	next.descending = next.sort == q.sort && !q.descending
	return next
}

// filteredBy returns the query a click on a filter chip produces: the roster
// narrowed to that option, ordered exactly as it is now. An empty option is the
// Alle chip, which widens the roster again.
func (q rosterQuery) filteredBy(option string) rosterQuery {
	next := q
	next.system = normalizeSystemFilter(option)
	return next
}

// path appends the query to a path, yielding a URL that restores the view. The
// default query carries no parameters, so a trainer who never sorted keeps the
// plain URLs and a junk-parameter request normalises to a bare redirect target.
func (q rosterQuery) path(p string) string {
	params := q.params()
	if params == "" {
		return p
	}
	return p + "?" + params
}

// params renders the query as URL query parameters, empty for the default query.
// The keys are listed in a fixed order (url.Values.Encode would sort them
// alphabetically instead) and every value has a fixed shape — a whitelisted
// column, one of two directions, or [a-z0-9-] — so the result never needs escaping.
// Shape rather than set is equally sufficient: those characters rule out escaping
// and header injection just as a closed set does (ADR-0007b).
//
// An absent filter renders no key at all, so it stays part of the default query
// and a bare /athletes keeps working.
func (q rosterQuery) params() string {
	if q == defaultRosterQuery {
		return ""
	}
	keys := []string{
		"sort=" + q.sort,
		"dir=" + q.dir(),
	}
	if q.system != "" {
		keys = append(keys, "system="+q.system)
	}
	return strings.Join(keys, "&")
}

func (q rosterQuery) dir() string {
	if q.descending {
		return dirDescending
	}
	return dirAscending
}

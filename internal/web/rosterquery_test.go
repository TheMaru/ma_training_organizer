package web_test

import (
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/TheMaru/ma_training_organizer/internal/store"
	"github.com/TheMaru/ma_training_organizer/internal/store/storetest"
	"github.com/TheMaru/ma_training_organizer/internal/trainer/trainertest"
)

// rosterSortQuery is one roster query as a trainer's URL carries it.
type rosterSortQuery struct{ sort, dir string }

// sortedRosterQueries are all the sort queries a trainer's URL can carry: every
// sortable column in both directions. The default query is deliberately among them
// — it must round-trip as a bare /athletes.
func sortedRosterQueries() []rosterSortQuery {
	var queries []rosterSortQuery
	for _, sort := range []string{
		store.RosterSortLastName, store.RosterSortFirstName,
		store.RosterSortBirthDate, store.RosterSortJoinedOn, store.RosterSortRank,
	} {
		queries = append(queries, rosterSortQuery{sort, "asc"}, rosterSortQuery{sort, "desc"})
	}
	return queries
}

// params is the query as it arrives on a request.
func (q rosterSortQuery) params() string {
	return fmt.Sprintf("sort=%s&dir=%s", q.sort, q.dir)
}

// rosterURL is the roster URL this query must produce. The default query carries
// no parameters at all — a bare /athletes is what renders it.
func (q rosterSortQuery) rosterURL() string {
	if q.sort == store.RosterSortDefault && q.dir == "asc" {
		return "/athletes"
	}
	return "/athletes?" + q.params()
}

// rosterQueryParams is one non-default query with every key set, so a redirect
// that drops any part of it fails.
const rosterQueryParams = "sort=rank&dir=desc&system=bjj-adult"

func TestOnlyRosterRedirectsKeepTheQuery(t *testing.T) {
	routes := []struct {
		name string
		path func(id int64) string
		form func(rankID int64) url.Values
		want func(id int64) string
	}{
		{
			name: "create",
			path: func(int64) string { return "/athletes" },
			form: func(int64) url.Values { return athleteForm("Grace", "Hopper", "", "", "") },
			want: func(int64) string { return "/athletes?" + rosterQueryParams },
		},
		{
			name: "update",
			path: func(id int64) string { return fmt.Sprintf("/athletes/%d", id) },
			form: func(int64) url.Values { return athleteForm("Augusta", "King", "", "", "") },
			want: func(int64) string { return "/athletes?" + rosterQueryParams },
		},
		{
			name: "delete",
			path: func(id int64) string { return fmt.Sprintf("/athletes/%d/delete", id) },
			form: func(int64) url.Values { return nil },
			want: func(int64) string { return "/athletes?" + rosterQueryParams },
		},
		{
			name: "promote",
			path: func(id int64) string { return fmt.Sprintf("/athletes/%d/promotions", id) },
			form: func(rankID int64) url.Values {
				return url.Values{"rankId": {fmt.Sprint(rankID)}, "promotedOn": {"2026-01-15"}}
			},
			want: func(id int64) string { return fmt.Sprintf("/athletes/%d?%s", id, rosterQueryParams) },
		},
		{
			name: "logout",
			path: func(int64) string { return "/logout" },
			form: func(int64) url.Values { return nil },
			want: func(int64) string { return "/login" },
		},
	}
	for _, route := range routes {
		for _, htmx := range []bool{false, true} {
			name := route.name
			if htmx {
				name += "-htmx"
			}
			t.Run(name, func(t *testing.T) {
				ts, client, db := newAuthTestServer(t)
				login(t, ts, client, testUsername, trainertest.Password).Body.Close()
				id, err := store.CreateAthlete(db, store.Athlete{FirstName: "Ada", LastName: "Lovelace"})
				if err != nil {
					t.Fatalf("CreateAthlete: %v", err)
				}
				rankID := storetest.RankID(t, db, "BJJ Adult", "Blue")

				path := route.path(id) + "?" + rosterQueryParams
				if htmx {
					hxRedirectTo(t, postHTMX(t, ts, client, path, route.form(rankID)), route.want(id))
				} else {
					seeOtherTo(t, post(t, ts, client, path, route.form(rankID)), route.want(id))
				}
			})
		}
	}
}

func TestEverySortRoundTripsThroughARedirect(t *testing.T) {
	for _, query := range sortedRosterQueries() {
		t.Run(query.sort+"-"+query.dir, func(t *testing.T) {
			ts, client, db := newAuthTestServer(t)
			login(t, ts, client, testUsername, trainertest.Password).Body.Close()
			id, err := store.CreateAthlete(db, store.Athlete{FirstName: "Ada", LastName: "Lovelace"})
			if err != nil {
				t.Fatalf("CreateAthlete: %v", err)
			}

			path := fmt.Sprintf("/athletes/%d/delete?%s", id, query.params())
			seeOtherTo(t, post(t, ts, client, path, nil), query.rosterURL())
		})
	}
}

func TestMutationWithJunkQueryRedirectsToWhitelistedRoster(t *testing.T) {
	ts, client, db := newAuthTestServer(t)
	login(t, ts, client, testUsername, trainertest.Password).Body.Close()
	id, err := store.CreateAthlete(db, store.Athlete{FirstName: "Ada", LastName: "Lovelace"})
	if err != nil {
		t.Fatalf("CreateAthlete: %v", err)
	}

	// Junk normalises to the default query, which carries no parameters — so nothing
	// user-controlled can reach the Location header.
	path := fmt.Sprintf("/athletes/%d/delete?sort=bogus&dir=sideways", id)
	seeOtherTo(t, post(t, ts, client, path, nil), "/athletes")
}

func TestMutationRedirectCarriesNormalisedQuery(t *testing.T) {
	ts, client, db := newAuthTestServer(t)
	login(t, ts, client, testUsername, trainertest.Password).Body.Close()
	id, err := store.CreateAthlete(db, store.Athlete{FirstName: "Ada", LastName: "Lovelace"})
	if err != nil {
		t.Fatalf("CreateAthlete: %v", err)
	}

	// An unknown column with a valid direction keeps the direction and falls back
	// to the default column — the redirect shows the normalised values, not the
	// raw ones.
	path := fmt.Sprintf("/athletes/%d/delete?sort=%s&dir=desc", id, url.QueryEscape("a.first_name; DROP TABLE athletes--"))
	seeOtherTo(t, post(t, ts, client, path, nil), "/athletes?sort=firstName&dir=desc")
}

func TestEditFormReturnsToSortedRoster(t *testing.T) {
	ts, client, db := newAuthTestServer(t)
	login(t, ts, client, testUsername, trainertest.Password).Body.Close()
	id, err := store.CreateAthlete(db, store.Athlete{FirstName: "Ada", LastName: "Lovelace"})
	if err != nil {
		t.Fatalf("CreateAthlete: %v", err)
	}

	body := readBody(t, get(t, ts, client, fmt.Sprintf("/athletes/%d/edit?sort=rank&dir=asc", id)))
	// Saving posts the query back, so the update's own redirect can use it …
	if want := fmt.Sprintf(`action="/athletes/%d?sort=rank&amp;dir=asc"`, id); !strings.Contains(body, want) {
		t.Errorf("edit form is missing %s", want)
	}
	// … and cancelling returns to the same roster without saving.
	if want := `href="/athletes?sort=rank&amp;dir=asc"`; !strings.Contains(body, want) {
		t.Errorf("edit form is missing a cancel link %s", want)
	}
}

func TestAthleteDetailOffersAWayBackToTheSortedRoster(t *testing.T) {
	ts, client, db := newAuthTestServer(t)
	login(t, ts, client, testUsername, trainertest.Password).Body.Close()
	id, err := store.CreateAthlete(db, store.Athlete{FirstName: "Ada", LastName: "Lovelace"})
	if err != nil {
		t.Fatalf("CreateAthlete: %v", err)
	}

	body := readBody(t, get(t, ts, client, fmt.Sprintf("/athletes/%d?sort=birthDate&dir=desc", id)))
	if want := `href="/athletes?sort=birthDate&amp;dir=desc"`; !strings.Contains(body, want) {
		t.Errorf("detail page is missing a way back: %s", want)
	}
	// Editing from here keeps the state, so the round trip closes.
	if want := fmt.Sprintf(`href="/athletes/%d/edit?sort=birthDate&amp;dir=desc"`, id); !strings.Contains(body, want) {
		t.Errorf("detail page is missing %s", want)
	}
}

func TestFilterOnlyQueryCarriesNoSortKeys(t *testing.T) {
	ts, client, db := newAuthTestServer(t)
	login(t, ts, client, testUsername, trainertest.Password).Body.Close()
	id, err := store.CreateAthlete(db, store.Athlete{FirstName: "Ada", LastName: "Lovelace"})
	if err != nil {
		t.Fatalf("CreateAthlete: %v", err)
	}

	// Filtering without sorting is a non-default query, so the URL spells out the
	// whole query — but the filter's own default (Alle) still adds nothing.
	path := fmt.Sprintf("/athletes/%d/delete?system=none", id)
	seeOtherTo(t, post(t, ts, client, path, nil), "/athletes?sort=firstName&dir=asc&system=none")
}

func TestMalformedFilterIsDropped(t *testing.T) {
	// Anything that is not slug-shaped reads as no filter at all, so no
	// user-controlled string can reach a Location header or a rendered URL —
	// the guarantee that lets params() render without escaping.
	for _, junk := range []string{
		"BJJ Kids", `bjj"onload=alert(1)`, "bjj_kids", "Bjj-Kids", "bjj-kids\r\nX: y",
		strings.Repeat("a", 65),
	} {
		t.Run(junk, func(t *testing.T) {
			ts, client, db := newAuthTestServer(t)
			login(t, ts, client, testUsername, trainertest.Password).Body.Close()
			id, err := store.CreateAthlete(db, store.Athlete{FirstName: "Ada", LastName: "Lovelace"})
			if err != nil {
				t.Fatalf("CreateAthlete: %v", err)
			}

			path := fmt.Sprintf("/athletes/%d/delete?system=%s", id, url.QueryEscape(junk))
			seeOtherTo(t, post(t, ts, client, path, nil), "/athletes")
		})
	}
}

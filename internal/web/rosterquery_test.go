package web_test

import (
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/TheMaru/ma_training_organizer/internal/store"
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

func TestDeleteReturnsToSortedRoster(t *testing.T) {
	for _, query := range sortedRosterQueries() {
		t.Run(query.sort+"-"+query.dir, func(t *testing.T) {
			ts, client, db := newAuthTestServer(t)
			login(t, ts, client, testUsername, testPassword).Body.Close()
			id, err := store.CreateAthlete(db, store.Athlete{FirstName: "Ada", LastName: "Lovelace"})
			if err != nil {
				t.Fatalf("CreateAthlete: %v", err)
			}

			path := fmt.Sprintf("/athletes/%d/delete?%s", id, query.params())
			seeOtherTo(t, post(t, ts, client, path, nil), query.rosterURL())
		})
	}
}

func TestDeleteViaHTMXReturnsToSortedRoster(t *testing.T) {
	ts, client, db := newAuthTestServer(t)
	login(t, ts, client, testUsername, testPassword).Body.Close()
	id, err := store.CreateAthlete(db, store.Athlete{FirstName: "Ada", LastName: "Lovelace"})
	if err != nil {
		t.Fatalf("CreateAthlete: %v", err)
	}

	// The roster's delete button posts via HTMX; the sort must survive on that path
	// too, where the target travels in HX-Redirect instead of Location.
	path := fmt.Sprintf("/athletes/%d/delete?sort=rank&dir=desc", id)
	hxRedirectTo(t, postHTMX(t, ts, client, path, nil), "/athletes?sort=rank&dir=desc")
}

func TestCreateReturnsToSortedRoster(t *testing.T) {
	ts, client, _ := newAuthTestServer(t)
	login(t, ts, client, testUsername, testPassword).Body.Close()

	resp := post(t, ts, client, "/athletes?sort=joinedOn&dir=desc",
		athleteForm("Ada", "Lovelace", "", "", ""))
	seeOtherTo(t, resp, "/athletes?sort=joinedOn&dir=desc")
}

func TestUpdateReturnsToSortedRoster(t *testing.T) {
	ts, client, db := newAuthTestServer(t)
	login(t, ts, client, testUsername, testPassword).Body.Close()
	id, err := store.CreateAthlete(db, store.Athlete{FirstName: "Ada", LastName: "Lovelace"})
	if err != nil {
		t.Fatalf("CreateAthlete: %v", err)
	}

	resp := post(t, ts, client, fmt.Sprintf("/athletes/%d?sort=lastName&dir=desc", id),
		athleteForm("Augusta", "King", "", "", ""))
	seeOtherTo(t, resp, "/athletes?sort=lastName&dir=desc")
}

func TestMutationWithJunkQueryRedirectsToWhitelistedRoster(t *testing.T) {
	ts, client, db := newAuthTestServer(t)
	login(t, ts, client, testUsername, testPassword).Body.Close()
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
	login(t, ts, client, testUsername, testPassword).Body.Close()
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

func TestRosterLinksCarryTheQuery(t *testing.T) {
	ts, client, db := newAuthTestServer(t)
	login(t, ts, client, testUsername, testPassword).Body.Close()
	id, err := store.CreateAthlete(db, store.Athlete{FirstName: "Ada", LastName: "Lovelace"})
	if err != nil {
		t.Fatalf("CreateAthlete: %v", err)
	}

	body := readBody(t, get(t, ts, client, "/athletes?sort=lastName&dir=desc"))
	for _, want := range []string{
		fmt.Sprintf("/athletes/%d?sort=lastName&amp;dir=desc", id),        // detail
		fmt.Sprintf("/athletes/%d/delete?sort=lastName&amp;dir=desc", id), // delete form
		"/athletes/new?sort=lastName&amp;dir=desc",                        // Neuer Athlet
	} {
		if !strings.Contains(body, want) {
			t.Errorf("roster body is missing a link to %q", want)
		}
	}
}

func TestDefaultRosterLinksCarryNoQuery(t *testing.T) {
	ts, client, db := newAuthTestServer(t)
	login(t, ts, client, testUsername, testPassword).Body.Close()
	if _, err := store.CreateAthlete(db, store.Athlete{FirstName: "Ada", LastName: "Lovelace"}); err != nil {
		t.Fatalf("CreateAthlete: %v", err)
	}

	// A trainer who never sorted sees the URLs they saw before this ticket.
	body := readBody(t, get(t, ts, client, "/athletes"))
	if strings.Contains(body, "/athletes/new?") {
		t.Error(`want a bare "Neuer Athlet" link in the default view`)
	}
}

func TestEditFormReturnsToSortedRoster(t *testing.T) {
	ts, client, db := newAuthTestServer(t)
	login(t, ts, client, testUsername, testPassword).Body.Close()
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
	login(t, ts, client, testUsername, testPassword).Body.Close()
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

func TestRecordingAPromotionKeepsTheQuery(t *testing.T) {
	ts, client, db := newAuthTestServer(t)
	login(t, ts, client, testUsername, testPassword).Body.Close()
	id, err := store.CreateAthlete(db, store.Athlete{FirstName: "Ada", LastName: "Lovelace"})
	if err != nil {
		t.Fatalf("CreateAthlete: %v", err)
	}
	rankID := seededRankID(t, db, "BJJ Adult", "Blue")

	// The detail page is reached from the roster, so its own form must not drop the
	// state either — otherwise the way back is lost after recording a promotion.
	resp := post(t, ts, client, fmt.Sprintf("/athletes/%d/promotions?sort=rank&dir=desc", id), url.Values{
		"rankId":     {fmt.Sprint(rankID)},
		"promotedOn": {"2026-01-15"},
	})
	seeOtherTo(t, resp, fmt.Sprintf("/athletes/%d?sort=rank&dir=desc", id))
}

func TestMutationRedirectKeepsTheFilter(t *testing.T) {
	ts, client, db := newAuthTestServer(t)
	login(t, ts, client, testUsername, testPassword).Body.Close()
	id, err := store.CreateAthlete(db, store.Athlete{FirstName: "Ada", LastName: "Lovelace"})
	if err != nil {
		t.Fatalf("CreateAthlete: %v", err)
	}

	// Filter and sort compose in one URL, and a mutation returns to both. The
	// value is carried through without the database being asked whether anyone is
	// in that system — only the roster handler checks that.
	path := fmt.Sprintf("/athletes/%d/delete?sort=rank&dir=desc&system=bjj-adult", id)
	seeOtherTo(t, post(t, ts, client, path, nil), "/athletes?sort=rank&dir=desc&system=bjj-adult")
}

func TestFilterOnlyQueryCarriesNoSortKeys(t *testing.T) {
	ts, client, db := newAuthTestServer(t)
	login(t, ts, client, testUsername, testPassword).Body.Close()
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
			login(t, ts, client, testUsername, testPassword).Body.Close()
			id, err := store.CreateAthlete(db, store.Athlete{FirstName: "Ada", LastName: "Lovelace"})
			if err != nil {
				t.Fatalf("CreateAthlete: %v", err)
			}

			path := fmt.Sprintf("/athletes/%d/delete?system=%s", id, url.QueryEscape(junk))
			seeOtherTo(t, post(t, ts, client, path, nil), "/athletes")
		})
	}
}

func TestRosterLinksCarryTheFilter(t *testing.T) {
	ts, client, db := newAuthTestServer(t)
	login(t, ts, client, testUsername, testPassword).Body.Close()
	id := promoteTo(t, db, "Kai", "Kind", "BJJ Kids", "White", "2026-01-01")
	promoteTo(t, db, "Adam", "Adult", "BJJ Adult", "Blue", "2026-01-01")

	// Every way out of a filtered roster comes back to it, without those call
	// sites knowing the filter exists.
	body := readBody(t, get(t, ts, client, "/athletes?system=bjj-kids"))
	for _, want := range []string{
		fmt.Sprintf("/athletes/%d?sort=firstName&amp;dir=asc&amp;system=bjj-kids", id),
		fmt.Sprintf("/athletes/%d/delete?sort=firstName&amp;dir=asc&amp;system=bjj-kids", id),
		"/athletes/new?sort=firstName&amp;dir=asc&amp;system=bjj-kids",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("filtered roster is missing a link to %q", want)
		}
	}
}

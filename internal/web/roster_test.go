package web_test

import (
	"database/sql"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/TheMaru/ma_training_organizer/internal/store"
)

// readBody returns the response body as a string, closing it.
func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(b)
}

// headerCell returns the <th> markup of the roster header carrying the given
// label, so a test can assert on its link and direction indicator without
// pinning down the rest of the table.
func headerCell(t *testing.T, body, label string) string {
	t.Helper()
	at := strings.Index(body, ">"+label)
	if at < 0 {
		t.Fatalf("header %q not found in body", label)
	}
	start := strings.LastIndex(body[:at], "<th")
	end := strings.Index(body[at:], "</th>")
	if start < 0 || end < 0 {
		t.Fatalf("header %q is not inside a <th>", label)
	}
	return body[start : at+end]
}

// rosterRow returns the markup of the roster row mentioning the given name.
func rosterRow(t *testing.T, body, name string) string {
	t.Helper()
	for _, row := range strings.Split(body, "<tr ")[1:] {
		end := strings.Index(row, "</tr>")
		if end >= 0 && strings.Contains(row[:end], name) {
			return row[:end]
		}
	}
	t.Fatalf("no roster row for %q", name)
	return ""
}

// rosterOrder reports the order the given names appear in the body, so a test can
// assert the rendered sort order.
func rosterOrder(t *testing.T, body string, names ...string) []string {
	t.Helper()
	type pos struct {
		name string
		at   int
	}
	found := make([]pos, 0, len(names))
	for _, name := range names {
		at := strings.Index(body, name)
		if at < 0 {
			t.Fatalf("athlete %q missing from roster", name)
		}
		found = append(found, pos{name, at})
	}
	slices.SortFunc(found, func(a, b pos) int { return a.at - b.at })
	order := make([]string, len(found))
	for i, f := range found {
		order[i] = f.name
	}
	return order
}

// threeRosterAthletes seeds a roster whose first-name and last-name orders differ.
func threeRosterAthletes(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, a := range []store.Athlete{
		{FirstName: "Carol", LastName: "Adler"},
		{FirstName: "Alice", LastName: "Zeder"},
		{FirstName: "Bob", LastName: "Meier"},
	} {
		if _, err := store.CreateAthlete(db, a); err != nil {
			t.Fatalf("CreateAthlete %s: %v", a.FirstName, err)
		}
	}
}

func TestRosterDefaultsToFirstNameAscending(t *testing.T) {
	ts, client, db := newAuthTestServer(t)
	login(t, ts, client, testUsername, testPassword).Body.Close()
	threeRosterAthletes(t, db)

	body := readBody(t, get(t, ts, client, "/athletes"))
	want := []string{"Alice", "Bob", "Carol"}
	if got := rosterOrder(t, body, "Carol", "Alice", "Bob"); !slices.Equal(got, want) {
		t.Errorf("default order = %v, want %v", got, want)
	}
	// The default column is the active one, so it carries the ascending indicator
	// and its own link toggles to descending.
	vorname := headerCell(t, body, "Vorname")
	if !strings.Contains(vorname, "▲") {
		t.Errorf("Vorname header = %q, want an ascending indicator", vorname)
	}
	if !strings.Contains(vorname, "/athletes?sort=firstName&amp;dir=desc") {
		t.Errorf("Vorname header = %q, want a link toggling to desc", vorname)
	}
	// An inactive column links to itself ascending and shows no indicator.
	nachname := headerCell(t, body, "Nachname")
	if !strings.Contains(nachname, "/athletes?sort=lastName&amp;dir=asc") {
		t.Errorf("Nachname header = %q, want a link to lastName asc", nachname)
	}
	if strings.ContainsAny(nachname, "▲▼") {
		t.Errorf("Nachname header = %q, want no indicator on an inactive column", nachname)
	}
}

func TestRosterSortsByRequestedColumn(t *testing.T) {
	ts, client, db := newAuthTestServer(t)
	login(t, ts, client, testUsername, testPassword).Body.Close()
	threeRosterAthletes(t, db)

	body := readBody(t, get(t, ts, client, "/athletes?sort=lastName&dir=desc"))
	want := []string{"Zeder", "Meier", "Adler"}
	if got := rosterOrder(t, body, "Adler", "Zeder", "Meier"); !slices.Equal(got, want) {
		t.Errorf("lastName desc order = %v, want %v", got, want)
	}
	// The active column shows the descending indicator and toggles back to asc.
	nachname := headerCell(t, body, "Nachname")
	if !strings.Contains(nachname, "▼") {
		t.Errorf("Nachname header = %q, want a descending indicator", nachname)
	}
	if !strings.Contains(nachname, "/athletes?sort=lastName&amp;dir=asc") {
		t.Errorf("Nachname header = %q, want a link toggling to asc", nachname)
	}
}

func TestRosterUnknownSortParamsFallBackToDefault(t *testing.T) {
	ts, client, db := newAuthTestServer(t)
	login(t, ts, client, testUsername, testPassword).Body.Close()
	threeRosterAthletes(t, db)

	// Sorting is a view concern: junk params render the default view, not an error.
	resp := get(t, ts, client, "/athletes?sort=bogus&dir=sideways")
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	body := readBody(t, resp)
	want := []string{"Alice", "Bob", "Carol"}
	if got := rosterOrder(t, body, "Carol", "Alice", "Bob"); !slices.Equal(got, want) {
		t.Errorf("order = %v, want default %v", got, want)
	}
	if !strings.Contains(headerCell(t, body, "Vorname"), "▲") {
		t.Error("want Vorname active ascending after junk params")
	}
}

func TestRosterShowsCurrentRank(t *testing.T) {
	ts, client, db := newAuthTestServer(t)
	login(t, ts, client, testUsername, testPassword).Body.Close()

	rankID := seededRankID(t, db, "BJJ Adult", "Blue")
	graded, err := store.CreateAthlete(db, store.Athlete{FirstName: "Ada", LastName: "Lovelace"})
	if err != nil {
		t.Fatalf("CreateAthlete: %v", err)
	}
	if _, err := store.CreatePromotion(db, store.Promotion{AthleteID: graded, RankID: rankID, PromotedOn: "2026-01-15"}); err != nil {
		t.Fatalf("CreatePromotion: %v", err)
	}
	if _, err := store.CreateAthlete(db, store.Athlete{FirstName: "Uwe", LastName: "Unbelted"}); err != nil {
		t.Fatalf("CreateAthlete: %v", err)
	}

	body := readBody(t, get(t, ts, client, "/athletes"))
	// Rank name plus the system that disambiguates same-named ranks across cohorts.
	row := rosterRow(t, body, "Lovelace")
	if !strings.Contains(row, "Blue") || !strings.Contains(row, "BJJ Adult") {
		t.Errorf("graded row = %q, want rank and system", row)
	}
	// Ungraded athletes render blank — no promotion at all is not a lowest rank.
	if ungraded := rosterRow(t, body, "Unbelted"); strings.Contains(ungraded, "BJJ Adult") {
		t.Errorf("ungraded row = %q, want a blank rank cell", ungraded)
	}
}

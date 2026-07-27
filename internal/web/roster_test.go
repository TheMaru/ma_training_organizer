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
// pinning down the rest of the table. Scoped to <th> elements because the same
// labels also appear in the phone sort chips (see sortChip).
func headerCell(t *testing.T, body, label string) string {
	t.Helper()
	for _, cell := range strings.Split(body, "<th")[1:] {
		end := strings.Index(cell, "</th>")
		if end >= 0 && strings.Contains(cell[:end], ">"+label) {
			return cell[:end]
		}
	}
	t.Fatalf("header %q not found in a <th>", label)
	return ""
}

// sortChipRow returns the markup of the phone sort chip row, the second sort
// surface (ADR-0005). Anchored on the class because the page header holds a
// <nav> of its own.
func sortChipRow(t *testing.T, body string) string {
	t.Helper()
	start := strings.Index(body, `<nav class="sort-chips"`)
	if start < 0 {
		t.Fatal("no sort chip row in body")
	}
	end := strings.Index(body[start:], "</nav>")
	if end < 0 {
		t.Fatal("sort chip row is not closed")
	}
	return body[start : start+end]
}

// sortChip returns the markup of the sort chip carrying the given label.
func sortChip(t *testing.T, body, label string) string {
	t.Helper()
	for _, chip := range strings.Split(sortChipRow(t, body), "<a")[1:] {
		end := strings.Index(chip, "</a>")
		if end >= 0 && strings.Contains(chip[:end], ">"+label) {
			return chip[:end]
		}
	}
	t.Fatalf("no sort chip for %q", label)
	return ""
}

// sortChipLabels returns the chips' labels in the order they are rendered.
func sortChipLabels(t *testing.T, body string) []string {
	t.Helper()
	chips := strings.Split(sortChipRow(t, body), "<a")[1:]
	labels := make([]string, 0, len(chips))
	for _, chip := range chips {
		open := strings.Index(chip, ">")
		end := strings.Index(chip, "</a>")
		if open < 0 || end < 0 {
			t.Fatalf("malformed sort chip %q", chip)
		}
		// The label runs from the tag's end to the first nested span (indicator
		// or direction text) or to the end of the chip.
		text := chip[open+1 : end]
		if nested := strings.Index(text, "<"); nested >= 0 {
			text = text[:nested]
		}
		labels = append(labels, strings.TrimSpace(text))
	}
	return labels
}

// attrValue returns the value of the named attribute in a fragment of markup.
// The leading space keeps a prefixed attribute from matching (aria-sort would
// otherwise answer a request for sort).
func attrValue(t *testing.T, markup, name string) string {
	t.Helper()
	at := strings.Index(markup, " "+name+`="`)
	if at < 0 {
		t.Fatalf("no %s attribute in %q", name, markup)
	}
	rest := markup[at+len(name)+3:]
	end := strings.Index(rest, `"`)
	if end < 0 {
		t.Fatalf("unterminated %s attribute in %q", name, markup)
	}
	return rest[:end]
}

// textOfElementWithID returns the text of the element carrying the given id, so
// a test can check what an aria-labelledby actually points at.
func textOfElementWithID(t *testing.T, body, id string) string {
	t.Helper()
	at := strings.Index(body, `id="`+id+`"`)
	if at < 0 {
		t.Fatalf("nothing carries id %q", id)
	}
	open := strings.Index(body[at:], ">")
	if open < 0 {
		t.Fatalf("unterminated tag carrying id %q", id)
	}
	rest := body[at+open+1:]
	end := strings.Index(rest, "<")
	if end < 0 {
		t.Fatalf("unterminated element carrying id %q", id)
	}
	return rest[:end]
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

// rosterColumnLabels are the sortable columns as a trainer sees them, in display
// order — the phone chips and the desktop headers must both offer exactly these.
var rosterColumnLabels = []string{"Nachname", "Vorname", "Geburtsdatum", "Eintritt", "Aktueller Rang"}

func TestRosterSortChipsLinkWhereTheHeadersDo(t *testing.T) {
	ts, client, db := newAuthTestServer(t)
	login(t, ts, client, testUsername, testPassword).Body.Close()
	threeRosterAthletes(t, db)

	// Two surfaces, one truth: whatever view the roster is in, a chip and its
	// column header sort to the same place.
	for _, url := range []string{"/athletes", "/athletes?sort=rank&dir=desc"} {
		body := readBody(t, get(t, ts, client, url))
		for _, label := range rosterColumnLabels {
			chip := attrValue(t, sortChip(t, body, label), "href")
			header := attrValue(t, headerCell(t, body, label), "href")
			if chip != header {
				t.Errorf("%s at %s: chip href = %q, header href = %q", label, url, chip, header)
			}
		}
	}
}

func TestRosterSortChipMarksTheActiveColumn(t *testing.T) {
	ts, client, db := newAuthTestServer(t)
	login(t, ts, client, testUsername, testPassword).Body.Close()
	threeRosterAthletes(t, db)

	// The default view: Vorname is active and ascending, and says so where the
	// ▲ cannot be read out.
	body := readBody(t, get(t, ts, client, "/athletes"))
	vorname := sortChip(t, body, "Vorname")
	if !strings.Contains(vorname, `aria-current="true"`) {
		t.Errorf("Vorname chip = %q, want aria-current on the active column", vorname)
	}
	if !strings.Contains(vorname, "aufsteigend") {
		t.Errorf("Vorname chip = %q, want visually hidden ascending direction text", vorname)
	}
	// An inactive chip carries neither.
	nachname := sortChip(t, body, "Nachname")
	if strings.Contains(nachname, "aria-current") {
		t.Errorf("Nachname chip = %q, want no aria-current on an inactive column", nachname)
	}
	if strings.Contains(nachname, "aufsteigend") || strings.Contains(nachname, "absteigend") {
		t.Errorf("Nachname chip = %q, want no direction text on an inactive column", nachname)
	}

	// Sorted descending: the active chip moves and reports the new direction.
	body = readBody(t, get(t, ts, client, "/athletes?sort=lastName&dir=desc"))
	nachname = sortChip(t, body, "Nachname")
	if !strings.Contains(nachname, `aria-current="true"`) {
		t.Errorf("Nachname chip = %q, want aria-current when it is the active column", nachname)
	}
	if !strings.Contains(nachname, "absteigend") {
		t.Errorf("Nachname chip = %q, want visually hidden descending direction text", nachname)
	}
	if strings.Contains(sortChip(t, body, "Vorname"), "aria-current") {
		t.Error("want no aria-current on Vorname once Nachname is active")
	}
}

func TestRosterSortChipRowIsALabelledNav(t *testing.T) {
	ts, client, db := newAuthTestServer(t)
	login(t, ts, client, testUsername, testPassword).Body.Close()
	threeRosterAthletes(t, db)

	body := readBody(t, get(t, ts, client, "/athletes"))
	// The row's name comes from the visible "Sortieren" line above it, so the
	// label is announced once rather than twice.
	labelID := attrValue(t, sortChipRow(t, body), "aria-labelledby")
	if label := textOfElementWithID(t, body, labelID); !strings.Contains(label, "Sortieren") {
		t.Errorf("element id=%q reads %q, want \"Sortieren\"", labelID, label)
	}
	// Every sortable column is reachable, in display order.
	if got := sortChipLabels(t, body); !slices.Equal(got, rosterColumnLabels) {
		t.Errorf("chips = %v, want %v", got, rosterColumnLabels)
	}
}

func TestRosterSortTargetIsTheWholeHeaderCell(t *testing.T) {
	ts, client, db := newAuthTestServer(t)
	login(t, ts, client, testUsername, testPassword).Body.Close()
	threeRosterAthletes(t, db)

	// app.css hangs the cell-filling click target off this class: the cell gives
	// up its padding and the link takes it over. Without the class a trainer is
	// back to aiming at the few characters of the column name.
	body := readBody(t, get(t, ts, client, "/athletes"))
	for _, label := range rosterColumnLabels {
		if cell := headerCell(t, body, label); !strings.Contains(cell, `class="sortable"`) {
			t.Errorf("%s header = %q, want class=\"sortable\"", label, cell)
		}
	}
	// The actions column sorts nothing, so it is not a target.
	if got := strings.Count(body, `<th class="sortable"`); got != len(rosterColumnLabels) {
		t.Errorf("sortable header cells = %d, want %d", got, len(rosterColumnLabels))
	}
}

func TestRosterSortIndicatorIsHiddenFromAssistiveTech(t *testing.T) {
	ts, client, db := newAuthTestServer(t)
	login(t, ts, client, testUsername, testPassword).Body.Close()
	threeRosterAthletes(t, db)

	// The bare ▲/▼ is announced as a character name; the meaning is carried by
	// aria-sort on the header and by the chip's direction text.
	body := readBody(t, get(t, ts, client, "/athletes"))
	glyph := `<span class="sort-dir" aria-hidden="true">▲</span>`
	if !strings.Contains(headerCell(t, body, "Vorname"), glyph) {
		t.Errorf("Vorname header = %q, want an aria-hidden indicator", headerCell(t, body, "Vorname"))
	}
	if !strings.Contains(sortChip(t, body, "Vorname"), glyph) {
		t.Errorf("Vorname chip = %q, want an aria-hidden indicator", sortChip(t, body, "Vorname"))
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

package web_test

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/TheMaru/ma_training_organizer/internal/store"
	"github.com/TheMaru/ma_training_organizer/internal/store/storetest"
)

// promoteTo creates an athlete promoted to one seeded rank, returning their id.
func promoteTo(t *testing.T, db *sql.DB, first, last, system, rank, date string) int64 {
	t.Helper()
	rankID := storetest.RankID(t, db, system, rank)
	id, err := store.CreateAthlete(db, store.Athlete{FirstName: first, LastName: last})
	if err != nil {
		t.Fatalf("CreateAthlete: %v", err)
	}
	if _, err := store.CreatePromotion(db, store.Promotion{AthleteID: id, RankID: rankID, PromotedOn: date}); err != nil {
		t.Fatalf("CreatePromotion: %v", err)
	}
	return id
}

// definitionValue returns the <dd> value following the named <dt> term, so a test
// can read one row of the athlete detail's meta list.
func definitionValue(t *testing.T, body, term string) string {
	t.Helper()
	at := strings.Index(body, "<dt>"+term+"</dt>")
	if at < 0 {
		t.Fatalf("no %q term in body", term)
	}
	rest := body[at:]
	open := strings.Index(rest, "<dd>")
	end := strings.Index(rest, "</dd>")
	if open < 0 || end < 0 {
		t.Fatalf("no value for %q in %q", term, rest)
	}
	return rest[open+len("<dd>") : end]
}

// historyRow returns the Verlauf row for the given promotion date. The history
// rows are plain <tr>, unlike the roster's, so rosterRow cannot find them.
func historyRow(t *testing.T, body, date string) string {
	t.Helper()
	for _, row := range strings.Split(body, "<tr>")[1:] {
		end := strings.Index(row, "</tr>")
		if end >= 0 && strings.Contains(row[:end], date) {
			return row[:end]
		}
	}
	t.Fatalf("no promotion history row for %q", date)
	return ""
}

func promoteForm(rankID int64, date string) url.Values {
	return url.Values{
		"rankId":     {fmt.Sprintf("%d", rankID)},
		"promotedOn": {date},
	}
}

func TestAthleteDetailLoads(t *testing.T) {
	ts, client, db := newAuthTestServer(t)
	login(t, ts, client, testUsername, testPassword).Body.Close()

	id, err := store.CreateAthlete(db, store.Athlete{FirstName: "Ada", LastName: "Lovelace"})
	if err != nil {
		t.Fatalf("CreateAthlete: %v", err)
	}

	resp := get(t, ts, client, fmt.Sprintf("/athletes/%d", id))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestAthleteDetailUnknownReturns404(t *testing.T) {
	ts, client, _ := newAuthTestServer(t)
	login(t, ts, client, testUsername, testPassword).Body.Close()

	resp := get(t, ts, client, "/athletes/999")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestAthleteDetailRequiresAuth(t *testing.T) {
	ts, client, db := newAuthTestServer(t)

	id, err := store.CreateAthlete(db, store.Athlete{FirstName: "Ada", LastName: "Lovelace"})
	if err != nil {
		t.Fatalf("CreateAthlete: %v", err)
	}

	resp := get(t, ts, client, fmt.Sprintf("/athletes/%d", id))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("status = %d, want %d (redirect)", resp.StatusCode, http.StatusSeeOther)
	}
	if loc := resp.Header.Get("Location"); loc != "/login" {
		t.Errorf("Location = %q, want %q", loc, "/login")
	}
}

func TestRecordPromotionEndToEnd(t *testing.T) {
	ts, client, db := newAuthTestServer(t)
	login(t, ts, client, testUsername, testPassword).Body.Close()

	rankID := storetest.RankID(t, db, "BJJ Adult", "White")
	id, err := store.CreateAthlete(db, store.Athlete{FirstName: "Ada", LastName: "Lovelace"})
	if err != nil {
		t.Fatalf("CreateAthlete: %v", err)
	}

	resp, err := client.PostForm(fmt.Sprintf("%s/athletes/%d/promotions", ts.URL, id), promoteForm(rankID, "2026-01-15"))
	if err != nil {
		t.Fatalf("POST promotion: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusSeeOther)
	}
	if loc := resp.Header.Get("Location"); loc != fmt.Sprintf("/athletes/%d", id) {
		t.Errorf("Location = %q, want /athletes/%d", loc, id)
	}

	promotions, err := store.ListPromotions(db, id)
	if err != nil {
		t.Fatalf("ListPromotions: %v", err)
	}
	if len(promotions) != 1 {
		t.Fatalf("promotions = %d, want 1", len(promotions))
	}
	if promotions[0].RankName != "White" || promotions[0].PromotedOn != "2026-01-15" {
		t.Errorf("promotion = %+v, want White/2026-01-15", promotions[0])
	}
}

// TestRecordPromotionUpdatesCurrentRank is the acceptance seam: recording a later
// promotion changes the derived current rank, including across systems.
func TestRecordPromotionUpdatesCurrentRank(t *testing.T) {
	ts, client, db := newAuthTestServer(t)
	login(t, ts, client, testUsername, testPassword).Body.Close()

	kidsGreen := storetest.RankID(t, db, "BJJ Kids", "Green")
	adultWhite := storetest.RankID(t, db, "BJJ Adult", "White")
	id, err := store.CreateAthlete(db, store.Athlete{FirstName: "Ada", LastName: "Lovelace"})
	if err != nil {
		t.Fatalf("CreateAthlete: %v", err)
	}

	post := func(rankID int64, date string) {
		resp, err := client.PostForm(fmt.Sprintf("%s/athletes/%d/promotions", ts.URL, id), promoteForm(rankID, date))
		if err != nil {
			t.Fatalf("POST promotion: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusSeeOther {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusSeeOther)
		}
	}

	post(kidsGreen, "2024-06-01")
	promotions, _ := store.ListPromotions(db, id)
	if cur, _ := store.CurrentRank(promotions); cur.RankName != "Green" || cur.SystemName != "BJJ Kids" {
		t.Fatalf("current after kids = %+v, want Green/BJJ Kids", cur)
	}

	// Crossing to the adult system with a later date makes the adult rank current.
	post(adultWhite, "2026-02-01")
	promotions, _ = store.ListPromotions(db, id)
	if cur, _ := store.CurrentRank(promotions); cur.RankName != "White" || cur.SystemName != "BJJ Adult" {
		t.Errorf("current after adult = %+v, want White/BJJ Adult", cur)
	}
}

func TestRecordPromotionRequiresRankAndDate(t *testing.T) {
	ts, client, db := newAuthTestServer(t)
	login(t, ts, client, testUsername, testPassword).Body.Close()

	id, err := store.CreateAthlete(db, store.Athlete{FirstName: "Ada", LastName: "Lovelace"})
	if err != nil {
		t.Fatalf("CreateAthlete: %v", err)
	}

	resp, err := client.PostForm(fmt.Sprintf("%s/athletes/%d/promotions", ts.URL, id), promoteForm(0, ""))
	if err != nil {
		t.Fatalf("POST promotion: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	promotions, _ := store.ListPromotions(db, id)
	if len(promotions) != 0 {
		t.Errorf("promotions = %d, want 0 (invalid record must not persist)", len(promotions))
	}
}

// TestRecordPromotionRejectsMalformedDate guards the DATE column: a hand-crafted
// POST with a non-ISO date must be rejected, not written (a bad string would
// break every later read of the athlete's history).
func TestRecordPromotionRejectsMalformedDate(t *testing.T) {
	ts, client, db := newAuthTestServer(t)
	login(t, ts, client, testUsername, testPassword).Body.Close()

	rankID := storetest.RankID(t, db, "BJJ Adult", "White")
	id, err := store.CreateAthlete(db, store.Athlete{FirstName: "Ada", LastName: "Lovelace"})
	if err != nil {
		t.Fatalf("CreateAthlete: %v", err)
	}

	resp, err := client.PostForm(fmt.Sprintf("%s/athletes/%d/promotions", ts.URL, id), promoteForm(rankID, "banana"))
	if err != nil {
		t.Fatalf("POST promotion: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	promotions, _ := store.ListPromotions(db, id)
	if len(promotions) != 0 {
		t.Errorf("promotions = %d, want 0 (malformed date must not persist)", len(promotions))
	}
}

func TestRecordPromotionRequiresAuth(t *testing.T) {
	ts, client, db := newAuthTestServer(t)

	rankID := storetest.RankID(t, db, "BJJ Adult", "White")
	id, err := store.CreateAthlete(db, store.Athlete{FirstName: "Ada", LastName: "Lovelace"})
	if err != nil {
		t.Fatalf("CreateAthlete: %v", err)
	}

	resp, err := client.PostForm(fmt.Sprintf("%s/athletes/%d/promotions", ts.URL, id), promoteForm(rankID, "2026-01-15"))
	if err != nil {
		t.Fatalf("POST promotion: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("status = %d, want %d (redirect to login)", resp.StatusCode, http.StatusSeeOther)
	}

	promotions, _ := store.ListPromotions(db, id)
	if len(promotions) != 0 {
		t.Errorf("promotions = %d, want 0 (anonymous record must not persist)", len(promotions))
	}
}

func TestRecordPromotionUnknownAthleteReturns404(t *testing.T) {
	ts, client, db := newAuthTestServer(t)
	login(t, ts, client, testUsername, testPassword).Body.Close()

	rankID := storetest.RankID(t, db, "BJJ Adult", "White")

	resp, err := client.PostForm(ts.URL+"/athletes/999/promotions", promoteForm(rankID, "2026-01-15"))
	if err != nil {
		t.Fatalf("POST promotion: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestAthleteDetailShowsTheBeltBesideTheRankName covers the two detail surfaces:
// unlike the roster, both print the rank name next to the graphic, which is why
// the graphic is decorative there (ADR-0004).
func TestAthleteDetailShowsTheBeltBesideTheRankName(t *testing.T) {
	ts, client, db := newAuthTestServer(t)
	login(t, ts, client, testUsername, testPassword).Body.Close()

	id := promoteTo(t, db, "Mia", "Kind", "BJJ Kids", "Grey-White, 2 stripes", "2026-02-02")

	body := readBody(t, get(t, ts, client, fmt.Sprintf("/athletes/%d", id)))

	current := definitionValue(t, body, "Aktueller Rang")
	if !strings.Contains(current, "Grau-Weiß, 2 Streifen") {
		t.Errorf("current rank = %q, want the rank name spelled out", current)
	}
	belt := beltIn(t, current)
	if !strings.Contains(belt, `aria-hidden="true"`) {
		t.Errorf("current-rank belt = %q, want it marked decorative", belt)
	}
	if strings.Contains(belt, "aria-label") {
		t.Errorf("current-rank belt = %q, want no accessible name beside the visible one", belt)
	}

	// Every history row too, so a belt can be checked against its own label.
	rank := cellWithLabel(t, historyRow(t, body, "2026-02-02"), "Rang")
	if !strings.Contains(rank, "Grau-Weiß, 2 Streifen") {
		t.Errorf("history rank cell = %q, want the rank name", rank)
	}
	if !strings.Contains(beltIn(t, rank), `aria-hidden="true"`) {
		t.Errorf("history belt = %q, want it marked decorative", beltIn(t, rank))
	}
}

func TestAthleteDetailFallsBackToTextWithoutAGraduation(t *testing.T) {
	ts, client, db := newAuthTestServer(t)
	login(t, ts, client, testUsername, testPassword).Body.Close()

	id, err := store.CreateAthlete(db, store.Athlete{FirstName: "Sophie", LastName: "Neumann"})
	if err != nil {
		t.Fatalf("CreateAthlete: %v", err)
	}

	body := readBody(t, get(t, ts, client, fmt.Sprintf("/athletes/%d", id)))
	current := definitionValue(t, body, "Aktueller Rang")
	if hasBelt(current) {
		t.Errorf("current rank = %q, want no graphic without a graduation", current)
	}
	if !strings.Contains(current, "noch keine Graduierung") {
		t.Errorf("current rank = %q, want the ungraded note", current)
	}
}

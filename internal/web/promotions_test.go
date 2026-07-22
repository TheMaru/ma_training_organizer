package web_test

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/TheMaru/ma_training_organizer/internal/store"
)

// seededRankID seeds the built-in grading systems and returns one rank's id, so
// promotion handler tests can target a real rank.
func seededRankID(t *testing.T, db *sql.DB, system, name string) int64 {
	t.Helper()
	if err := store.Seed(db); err != nil {
		t.Fatalf("Seed: %v", err)
	}
	var id int64
	err := db.QueryRow(`
		SELECT r.id FROM ranks r
		JOIN grading_systems g ON g.id = r.grading_system_id
		WHERE g.name = ? AND r.name = ?`, system, name).Scan(&id)
	if err != nil {
		t.Fatalf("lookup rank %q/%q: %v", system, name, err)
	}
	return id
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

	rankID := seededRankID(t, db, "BJJ Adult", "White")
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

	kidsGreen := seededRankID(t, db, "BJJ Kids", "Green")
	adultWhite := seededRankID(t, db, "BJJ Adult", "White")
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

	rankID := seededRankID(t, db, "BJJ Adult", "White")
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

	rankID := seededRankID(t, db, "BJJ Adult", "White")
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

	rankID := seededRankID(t, db, "BJJ Adult", "White")

	resp, err := client.PostForm(ts.URL+"/athletes/999/promotions", promoteForm(rankID, "2026-01-15"))
	if err != nil {
		t.Fatalf("POST promotion: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

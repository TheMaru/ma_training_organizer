package web_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/TheMaru/ma_training_organizer/internal/store"
)

// athleteForm is the set of form fields the create/edit handlers accept.
func athleteForm(first, last, birth, joined, notes string) url.Values {
	return url.Values{
		"firstName": {first},
		"lastName":  {last},
		"birthDate": {birth},
		"joinedOn":  {joined},
		"notes":     {notes},
	}
}

func TestAthletesListRequiresAuth(t *testing.T) {
	ts, client, _ := newAuthTestServer(t)

	resp := get(t, ts, client, "/athletes")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("status = %d, want %d (redirect)", resp.StatusCode, http.StatusSeeOther)
	}
	if loc := resp.Header.Get("Location"); loc != "/login" {
		t.Errorf("Location = %q, want %q", loc, "/login")
	}
}

func TestAthletesListLoadsWhenAuthenticated(t *testing.T) {
	ts, client, _ := newAuthTestServer(t)
	login(t, ts, client, testUsername, testPassword).Body.Close()

	resp := get(t, ts, client, "/athletes")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestCreateAthleteEndToEnd(t *testing.T) {
	ts, client, db := newAuthTestServer(t)
	login(t, ts, client, testUsername, testPassword).Body.Close()

	resp, err := client.PostForm(ts.URL+"/athletes",
		athleteForm("Ada", "Lovelace", "1990-12-10", "2026-01-15", "linkshänder"))
	if err != nil {
		t.Fatalf("POST /athletes: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusSeeOther)
	}
	if loc := resp.Header.Get("Location"); loc != "/athletes" {
		t.Errorf("Location = %q, want %q", loc, "/athletes")
	}

	athletes, err := store.ListAthletes(db, false)
	if err != nil {
		t.Fatalf("ListAthletes: %v", err)
	}
	if len(athletes) != 1 {
		t.Fatalf("athlete count = %d, want 1", len(athletes))
	}
	got := athletes[0]
	if got.FirstName != "Ada" || got.LastName != "Lovelace" ||
		got.BirthDate != "1990-12-10" || got.JoinedOn != "2026-01-15" || got.Notes != "linkshänder" {
		t.Errorf("stored athlete = %+v", got)
	}
}

func TestCreateAthleteRequiresName(t *testing.T) {
	ts, client, db := newAuthTestServer(t)
	login(t, ts, client, testUsername, testPassword).Body.Close()

	resp, err := client.PostForm(ts.URL+"/athletes", athleteForm("Ada", "", "", "", ""))
	if err != nil {
		t.Fatalf("POST /athletes: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	athletes, err := store.ListAthletes(db, false)
	if err != nil {
		t.Fatalf("ListAthletes: %v", err)
	}
	if len(athletes) != 0 {
		t.Errorf("athlete count = %d, want 0 (invalid create must not persist)", len(athletes))
	}
}

// What the trainer is promised for a date the store refuses
// (store.ErrMalformedDate): a client error, a message, their own input still in
// the fields — and the shared roster still standing.
func TestCreateAthleteRefusesAMalformedDate(t *testing.T) {
	tests := []struct {
		name          string
		birth, joined string
		wantPreserved string
	}{
		{"birth date", "morgen", "", "morgen"},
		{"joined on", "", "irgendwann", "irgendwann"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts, client, db := newAuthTestServer(t)
			login(t, ts, client, testUsername, testPassword).Body.Close()

			resp, err := client.PostForm(ts.URL+"/athletes",
				athleteForm("Ada", "Lovelace", tt.birth, tt.joined, "linkshänder"))
			if err != nil {
				t.Fatalf("POST /athletes: %v", err)
			}
			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
			}
			body := readBody(t, resp)
			if !strings.Contains(body, `class="error"`) {
				t.Error("no error message in the re-rendered form")
			}
			for _, want := range []string{"Ada", "Lovelace", "linkshänder", tt.wantPreserved} {
				if !strings.Contains(body, want) {
					t.Errorf("re-rendered form lost %q", want)
				}
			}

			athletes, err := store.ListAthletes(db, false)
			if err != nil {
				t.Fatalf("ListAthletes: %v", err)
			}
			if len(athletes) != 0 {
				t.Errorf("athlete count = %d, want 0 (a malformed date must not persist)", len(athletes))
			}

			// The reproduction: the shared roster still loads afterwards.
			roster := get(t, ts, client, "/athletes")
			defer roster.Body.Close()
			if roster.StatusCode != http.StatusOK {
				t.Errorf("roster status after the refused write = %d, want %d", roster.StatusCode, http.StatusOK)
			}
		})
	}
}

func TestUpdateAthleteRefusesAMalformedDate(t *testing.T) {
	ts, client, db := newAuthTestServer(t)
	login(t, ts, client, testUsername, testPassword).Body.Close()

	id, err := store.CreateAthlete(db, store.Athlete{
		FirstName: "Ada", LastName: "Lovelace", BirthDate: "1990-12-10",
	})
	if err != nil {
		t.Fatalf("CreateAthlete: %v", err)
	}

	resp, err := client.PostForm(fmt.Sprintf("%s/athletes/%d", ts.URL, id),
		athleteForm("Augusta", "King", "morgen", "irgendwann", "gräfin"))
	if err != nil {
		t.Fatalf("POST update: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	body := readBody(t, resp)
	if !strings.Contains(body, `class="error"`) {
		t.Error("no error message in the re-rendered form")
	}
	for _, want := range []string{"Augusta", "King", "gräfin", "morgen", "irgendwann"} {
		if !strings.Contains(body, want) {
			t.Errorf("re-rendered form lost %q", want)
		}
	}

	got, err := store.AthleteByID(db, id)
	if err != nil {
		t.Fatalf("AthleteByID: %v", err)
	}
	if got.BirthDate != "1990-12-10" {
		t.Errorf("birth date = %q, want unchanged %q", got.BirthDate, "1990-12-10")
	}
}

func TestCreateAthleteRequiresAuth(t *testing.T) {
	ts, client, db := newAuthTestServer(t)

	resp, err := client.PostForm(ts.URL+"/athletes", athleteForm("Ada", "Lovelace", "", "", ""))
	if err != nil {
		t.Fatalf("POST /athletes: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("status = %d, want %d (redirect to login)", resp.StatusCode, http.StatusSeeOther)
	}

	athletes, _ := store.ListAthletes(db, false)
	if len(athletes) != 0 {
		t.Errorf("athlete count = %d, want 0 (anonymous create must not persist)", len(athletes))
	}
}

func TestEditFormLoads(t *testing.T) {
	ts, client, db := newAuthTestServer(t)
	login(t, ts, client, testUsername, testPassword).Body.Close()

	id, err := store.CreateAthlete(db, store.Athlete{FirstName: "Ada", LastName: "Lovelace"})
	if err != nil {
		t.Fatalf("CreateAthlete: %v", err)
	}

	resp := get(t, ts, client, fmt.Sprintf("/athletes/%d/edit", id))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestEditFormUnknownAthleteReturns404(t *testing.T) {
	ts, client, _ := newAuthTestServer(t)
	login(t, ts, client, testUsername, testPassword).Body.Close()

	resp := get(t, ts, client, "/athletes/999/edit")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestUpdateAthleteEndToEnd(t *testing.T) {
	ts, client, db := newAuthTestServer(t)
	login(t, ts, client, testUsername, testPassword).Body.Close()

	id, err := store.CreateAthlete(db, store.Athlete{FirstName: "Ada", LastName: "Lovelace"})
	if err != nil {
		t.Fatalf("CreateAthlete: %v", err)
	}

	resp, err := client.PostForm(fmt.Sprintf("%s/athletes/%d", ts.URL, id),
		athleteForm("Augusta", "King", "1815-12-10", "", "gräfin"))
	if err != nil {
		t.Fatalf("POST update: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusSeeOther)
	}

	got, err := store.AthleteByID(db, id)
	if err != nil {
		t.Fatalf("AthleteByID: %v", err)
	}
	if got.FirstName != "Augusta" || got.LastName != "King" || got.BirthDate != "1815-12-10" || got.Notes != "gräfin" {
		t.Errorf("updated athlete = %+v", got)
	}
}

func TestUpdateAthleteRequiresName(t *testing.T) {
	ts, client, db := newAuthTestServer(t)
	login(t, ts, client, testUsername, testPassword).Body.Close()

	id, err := store.CreateAthlete(db, store.Athlete{FirstName: "Ada", LastName: "Lovelace"})
	if err != nil {
		t.Fatalf("CreateAthlete: %v", err)
	}

	resp, err := client.PostForm(fmt.Sprintf("%s/athletes/%d", ts.URL, id),
		athleteForm("Ada", "", "", "", ""))
	if err != nil {
		t.Fatalf("POST update: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	// The original last name must survive a rejected update.
	got, _ := store.AthleteByID(db, id)
	if got.LastName != "Lovelace" {
		t.Errorf("last name = %q, want unchanged %q", got.LastName, "Lovelace")
	}
}

func TestUpdateUnknownAthleteReturns404(t *testing.T) {
	ts, client, _ := newAuthTestServer(t)
	login(t, ts, client, testUsername, testPassword).Body.Close()

	resp, err := client.PostForm(ts.URL+"/athletes/999", athleteForm("Ghost", "Rider", "", "", ""))
	if err != nil {
		t.Fatalf("POST update: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestDeleteAthleteEndToEnd(t *testing.T) {
	ts, client, db := newAuthTestServer(t)
	login(t, ts, client, testUsername, testPassword).Body.Close()

	id, err := store.CreateAthlete(db, store.Athlete{FirstName: "Ada", LastName: "Lovelace"})
	if err != nil {
		t.Fatalf("CreateAthlete: %v", err)
	}

	resp, err := client.PostForm(fmt.Sprintf("%s/athletes/%d/delete", ts.URL, id), nil)
	if err != nil {
		t.Fatalf("POST delete: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusSeeOther)
	}

	if _, err := store.AthleteByID(db, id); !errors.Is(err, store.ErrAthleteNotFound) {
		t.Errorf("after delete, AthleteByID error = %v, want ErrAthleteNotFound", err)
	}
}

func TestDeleteUnknownAthleteReturns404(t *testing.T) {
	ts, client, _ := newAuthTestServer(t)
	login(t, ts, client, testUsername, testPassword).Body.Close()

	resp, err := client.PostForm(ts.URL+"/athletes/999/delete", nil)
	if err != nil {
		t.Fatalf("POST delete: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

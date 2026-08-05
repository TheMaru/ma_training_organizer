package store_test

import (
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/TheMaru/ma_training_organizer/internal/store"
	"github.com/TheMaru/ma_training_organizer/internal/store/storetest"
)

func TestCreateAndGetAthlete(t *testing.T) {
	db := storetest.NewDB(t)

	want := store.Athlete{
		FirstName: "Ada",
		LastName:  "Lovelace",
		BirthDate: "1990-12-10",
		JoinedOn:  "2026-01-15",
		Notes:     "linkshänder",
	}
	id, err := store.CreateAthlete(db, want)
	if err != nil {
		t.Fatalf("CreateAthlete: %v", err)
	}
	if id == 0 {
		t.Fatal("CreateAthlete returned zero id")
	}

	got, err := store.AthleteByID(db, id)
	if err != nil {
		t.Fatalf("AthleteByID: %v", err)
	}
	want.ID = id
	if got != want {
		t.Errorf("athlete = %+v, want %+v", got, want)
	}
}

func TestCreateAthleteStoresBlankDatesAsNull(t *testing.T) {
	db := storetest.NewDB(t)

	id, err := store.CreateAthlete(db, store.Athlete{FirstName: "No", LastName: "Dates"})
	if err != nil {
		t.Fatalf("CreateAthlete: %v", err)
	}

	// Blank optional dates must land as SQL NULL (portable SQL, ADR-0002), not an
	// empty string that a stricter engine would reject for a DATE column.
	var birth, joined sql.NullString
	err = db.QueryRow(`SELECT birth_date, joined_on FROM athletes WHERE id = ?`, id).
		Scan(&birth, &joined)
	if err != nil {
		t.Fatalf("scan raw dates: %v", err)
	}
	if birth.Valid {
		t.Errorf("birth_date = %q, want NULL", birth.String)
	}
	if joined.Valid {
		t.Errorf("joined_on = %q, want NULL", joined.String)
	}

	// And they read back as empty strings through the store.
	got, err := store.AthleteByID(db, id)
	if err != nil {
		t.Fatalf("AthleteByID: %v", err)
	}
	if got.BirthDate != "" || got.JoinedOn != "" {
		t.Errorf("dates = (%q, %q), want empty", got.BirthDate, got.JoinedOn)
	}
}

func TestCreateAthletesInsertsTheWholeBatch(t *testing.T) {
	db := storetest.NewDB(t)

	err := store.CreateAthletes(db, []store.Athlete{
		{FirstName: "Ada", LastName: "Lovelace", BirthDate: "1990-12-10"},
		{FirstName: "Grace", LastName: "Hopper"},
	})
	if err != nil {
		t.Fatalf("CreateAthletes: %v", err)
	}

	got, err := store.ListAthletes(db, false)
	if err != nil {
		t.Fatalf("ListAthletes: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("athletes = %+v, want both inserted", got)
	}
	if got[0].LastName != "Hopper" || got[1].BirthDate != "1990-12-10" {
		t.Errorf("athletes = %+v, want Hopper first and Ada's date kept", got)
	}
}

// The batch is one transaction, so a failure part-way through must leave the
// roster exactly as it was — the CSV import relies on that for its
// all-or-nothing guarantee. A temporary unique index is the lever: it makes the
// third insert fail after the first two have already been written.
func TestCreateAthletesRollsBackTheWholeBatch(t *testing.T) {
	db := storetest.NewDB(t)
	if _, err := db.Exec(`CREATE UNIQUE INDEX tmp_name ON athletes (first_name, last_name)`); err != nil {
		t.Fatalf("create unique index: %v", err)
	}

	err := store.CreateAthletes(db, []store.Athlete{
		{FirstName: "Ada", LastName: "Lovelace"},
		{FirstName: "Grace", LastName: "Hopper"},
		{FirstName: "Ada", LastName: "Lovelace"},
	})
	if err == nil {
		t.Fatal("CreateAthletes succeeded, want the duplicate insert to fail")
	}
	if !strings.Contains(err.Error(), "Ada Lovelace") {
		t.Errorf("error = %v, want it to name the offending athlete", err)
	}

	got, err := store.ListAthletes(db, false)
	if err != nil {
		t.Fatalf("ListAthletes: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("athletes = %+v, want none — the failed batch must roll back", got)
	}
}

func TestCreateAthletesOfNothingIsANoOp(t *testing.T) {
	db := storetest.NewDB(t)

	// The importer hands over an empty batch whenever every row was already on
	// the roster; that must commit cleanly rather than error.
	if err := store.CreateAthletes(db, nil); err != nil {
		t.Fatalf("CreateAthletes(nil): %v", err)
	}
}

func TestAthleteByIDNotFound(t *testing.T) {
	db := storetest.NewDB(t)

	_, err := store.AthleteByID(db, 404)
	if !errors.Is(err, store.ErrAthleteNotFound) {
		t.Errorf("error = %v, want ErrAthleteNotFound", err)
	}
}

func TestListAthletesSortsByLastName(t *testing.T) {
	db := storetest.NewDB(t)

	// Insert deliberately out of order.
	for _, a := range []store.Athlete{
		{FirstName: "Grace", LastName: "Hopper"},
		{FirstName: "Ada", LastName: "Lovelace"},
		{FirstName: "Alan", LastName: "Turing"},
		{FirstName: "Charles", LastName: "Babbage"},
	} {
		if _, err := store.CreateAthlete(db, a); err != nil {
			t.Fatalf("CreateAthlete %s: %v", a.LastName, err)
		}
	}

	asc, err := store.ListAthletes(db, false)
	if err != nil {
		t.Fatalf("ListAthletes asc: %v", err)
	}
	wantAsc := []string{"Babbage", "Hopper", "Lovelace", "Turing"}
	if got := lastNames(asc); !equal(got, wantAsc) {
		t.Errorf("ascending = %v, want %v", got, wantAsc)
	}

	desc, err := store.ListAthletes(db, true)
	if err != nil {
		t.Fatalf("ListAthletes desc: %v", err)
	}
	wantDesc := []string{"Turing", "Lovelace", "Hopper", "Babbage"}
	if got := lastNames(desc); !equal(got, wantDesc) {
		t.Errorf("descending = %v, want %v", got, wantDesc)
	}
}

func TestListAthletesTieBreakerFlipsWithDirection(t *testing.T) {
	db := storetest.NewDB(t)

	// Same last name: the first-name tie-breaker decides order, and must flip
	// with the direction so descending is a true reverse of ascending.
	for _, a := range []store.Athlete{
		{FirstName: "Bob", LastName: "Gracie"},
		{FirstName: "Alice", LastName: "Gracie"},
		{FirstName: "Carol", LastName: "Gracie"},
	} {
		if _, err := store.CreateAthlete(db, a); err != nil {
			t.Fatalf("CreateAthlete %s: %v", a.FirstName, err)
		}
	}

	asc, err := store.ListAthletes(db, false)
	if err != nil {
		t.Fatalf("ListAthletes asc: %v", err)
	}
	if got := firstNames(asc); !equal(got, []string{"Alice", "Bob", "Carol"}) {
		t.Errorf("ascending first names = %v, want [Alice Bob Carol]", got)
	}

	desc, err := store.ListAthletes(db, true)
	if err != nil {
		t.Fatalf("ListAthletes desc: %v", err)
	}
	if got := firstNames(desc); !equal(got, []string{"Carol", "Bob", "Alice"}) {
		t.Errorf("descending first names = %v, want [Carol Bob Alice]", got)
	}
}

func TestUpdateAthlete(t *testing.T) {
	db := storetest.NewDB(t)

	id, err := store.CreateAthlete(db, store.Athlete{FirstName: "Ada", LastName: "Lovelace"})
	if err != nil {
		t.Fatalf("CreateAthlete: %v", err)
	}

	updated := store.Athlete{
		ID:        id,
		FirstName: "Augusta",
		LastName:  "King",
		BirthDate: "1815-12-10",
		Notes:     "gräfin",
	}
	if err := store.UpdateAthlete(db, updated); err != nil {
		t.Fatalf("UpdateAthlete: %v", err)
	}

	got, err := store.AthleteByID(db, id)
	if err != nil {
		t.Fatalf("AthleteByID: %v", err)
	}
	if got != updated {
		t.Errorf("athlete = %+v, want %+v", got, updated)
	}
}

func TestUpdateAthleteUnknownID(t *testing.T) {
	db := storetest.NewDB(t)

	err := store.UpdateAthlete(db, store.Athlete{ID: 999, FirstName: "Ghost", LastName: "Rider"})
	if !errors.Is(err, store.ErrAthleteNotFound) {
		t.Errorf("error = %v, want ErrAthleteNotFound", err)
	}
}

func TestDeleteAthlete(t *testing.T) {
	db := storetest.NewDB(t)

	id, err := store.CreateAthlete(db, store.Athlete{FirstName: "Ada", LastName: "Lovelace"})
	if err != nil {
		t.Fatalf("CreateAthlete: %v", err)
	}

	if err := store.DeleteAthlete(db, id); err != nil {
		t.Fatalf("DeleteAthlete: %v", err)
	}

	if _, err := store.AthleteByID(db, id); !errors.Is(err, store.ErrAthleteNotFound) {
		t.Errorf("after delete, AthleteByID error = %v, want ErrAthleteNotFound", err)
	}
}

func TestDeleteAthleteUnknownID(t *testing.T) {
	db := storetest.NewDB(t)

	err := store.DeleteAthlete(db, 999)
	if !errors.Is(err, store.ErrAthleteNotFound) {
		t.Errorf("error = %v, want ErrAthleteNotFound", err)
	}
}

func TestDeleteAthleteViaStoreCascadesToPromotions(t *testing.T) {
	db := storetest.NewDB(t)

	rankID := storetest.RankID(t, db, "BJJ Adult", "White")
	athleteID, err := store.CreateAthlete(db, store.Athlete{FirstName: "Ada", LastName: "Lovelace"})
	if err != nil {
		t.Fatalf("CreateAthlete: %v", err)
	}
	storetest.MustInsert(t, db, `INSERT INTO promotions (athlete_id, rank_id, promoted_on) VALUES (?, ?, ?)`, athleteID, rankID, "2026-01-01")

	if err := store.DeleteAthlete(db, athleteID); err != nil {
		t.Fatalf("DeleteAthlete: %v", err)
	}

	var remaining int
	if err := db.QueryRow(`SELECT COUNT(*) FROM promotions WHERE athlete_id = ?`, athleteID).Scan(&remaining); err != nil {
		t.Fatalf("count promotions: %v", err)
	}
	if remaining != 0 {
		t.Errorf("promotions after delete = %d, want 0", remaining)
	}
}

func lastNames(as []store.Athlete) []string {
	names := make([]string, len(as))
	for i, a := range as {
		names[i] = a.LastName
	}
	return names
}

func firstNames(as []store.Athlete) []string {
	names := make([]string, len(as))
	for i, a := range as {
		names[i] = a.FirstName
	}
	return names
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

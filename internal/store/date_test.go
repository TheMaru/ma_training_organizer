package store_test

import (
	"errors"
	"testing"

	"github.com/TheMaru/ma_training_organizer/internal/store"
	"github.com/TheMaru/ma_training_organizer/internal/store/storetest"
)

// The invariant under test, and why it is a write-path concern, is documented on
// store.ErrMalformedDate.

func TestCreateAthleteRefusesAMalformedDate(t *testing.T) {
	tests := []struct {
		name    string
		athlete store.Athlete
	}{
		{"birth date is prose", store.Athlete{FirstName: "Ada", LastName: "Lovelace", BirthDate: "morgen"}},
		{"birth date is german", store.Athlete{FirstName: "Ada", LastName: "Lovelace", BirthDate: "10.12.1990"}},
		{"birth date is unpadded", store.Athlete{FirstName: "Ada", LastName: "Lovelace", BirthDate: "1990-12-1"}},
		{"birth date is out of range", store.Athlete{FirstName: "Ada", LastName: "Lovelace", BirthDate: "1990-13-45"}},
		{"joined on is prose", store.Athlete{FirstName: "Ada", LastName: "Lovelace", JoinedOn: "morgen"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := storetest.NewDB(t)

			_, err := store.CreateAthlete(db, tt.athlete)
			if !errors.Is(err, store.ErrMalformedDate) {
				t.Errorf("error = %v, want ErrMalformedDate", err)
			}

			athletes, err := store.ListAthletes(db, false)
			if err != nil {
				t.Fatalf("ListAthletes: %v", err)
			}
			if len(athletes) != 0 {
				t.Errorf("athletes = %+v, want none — a refused write must store nothing", athletes)
			}
		})
	}
}

func TestCreateAthletesRefusesAMalformedDate(t *testing.T) {
	db := storetest.NewDB(t)

	err := store.CreateAthletes(db, []store.Athlete{
		{FirstName: "Grace", LastName: "Hopper"},
		{FirstName: "Ada", LastName: "Lovelace", BirthDate: "morgen"},
	})
	if !errors.Is(err, store.ErrMalformedDate) {
		t.Errorf("error = %v, want ErrMalformedDate", err)
	}

	athletes, err := store.ListAthletes(db, false)
	if err != nil {
		t.Fatalf("ListAthletes: %v", err)
	}
	if len(athletes) != 0 {
		t.Errorf("athletes = %+v, want none — the batch is all-or-nothing", athletes)
	}
}

func TestUpdateAthleteRefusesAMalformedDate(t *testing.T) {
	db := storetest.NewDB(t)

	id, err := store.CreateAthlete(db, store.Athlete{
		FirstName: "Ada", LastName: "Lovelace", BirthDate: "1990-12-10",
	})
	if err != nil {
		t.Fatalf("CreateAthlete: %v", err)
	}

	err = store.UpdateAthlete(db, store.Athlete{
		ID: id, FirstName: "Ada", LastName: "Lovelace", BirthDate: "morgen",
	})
	if !errors.Is(err, store.ErrMalformedDate) {
		t.Errorf("error = %v, want ErrMalformedDate", err)
	}

	got, err := store.AthleteByID(db, id)
	if err != nil {
		t.Fatalf("AthleteByID: %v", err)
	}
	if got.BirthDate != "1990-12-10" {
		t.Errorf("birth date = %q, want the stored one unchanged", got.BirthDate)
	}
}

func TestBlankAthleteDatesStayValid(t *testing.T) {
	db := storetest.NewDB(t)

	// Both dates are optional (migration 00001), so blank is not malformed — it is
	// absent, and clearing a date the trainer filled in by mistake must keep working.
	id, err := store.CreateAthlete(db, store.Athlete{FirstName: "No", LastName: "Dates"})
	if err != nil {
		t.Fatalf("CreateAthlete with blank dates: %v", err)
	}
	err = store.UpdateAthlete(db, store.Athlete{ID: id, FirstName: "No", LastName: "Dates"})
	if err != nil {
		t.Fatalf("UpdateAthlete with blank dates: %v", err)
	}

	got, err := store.AthleteByID(db, id)
	if err != nil {
		t.Fatalf("AthleteByID: %v", err)
	}
	if got.BirthDate != "" || got.JoinedOn != "" {
		t.Errorf("dates = (%q, %q), want both absent", got.BirthDate, got.JoinedOn)
	}
}

// Blank is in the table because a promotion's date is required, not optional.
func TestCreatePromotionRefusesAMalformedDate(t *testing.T) {
	for _, date := range []string{"morgen", "01.01.2026", "2026-1-1", "", " "} {
		t.Run(date, func(t *testing.T) {
			db := storetest.NewDB(t)
			rankID := storetest.RankID(t, db, "BJJ Adult", "White")
			athleteID, err := store.CreateAthlete(db, store.Athlete{FirstName: "Ada", LastName: "Lovelace"})
			if err != nil {
				t.Fatalf("CreateAthlete: %v", err)
			}

			_, err = store.CreatePromotion(db, store.Promotion{
				AthleteID: athleteID, RankID: rankID, PromotedOn: date,
			})
			if !errors.Is(err, store.ErrMalformedDate) {
				t.Errorf("error = %v, want ErrMalformedDate", err)
			}

			promotions, err := store.ListPromotions(db, athleteID)
			if err != nil {
				t.Fatalf("ListPromotions: %v", err)
			}
			if len(promotions) != 0 {
				t.Errorf("promotions = %+v, want none", promotions)
			}
		})
	}
}

// TestARefusedDateLeavesEveryListingReadable pins the reported defect: one
// athlete written with the birth date "morgen" used to make every later read of
// the whole roster fail.
func TestARefusedDateLeavesEveryListingReadable(t *testing.T) {
	db := storetest.NewDB(t)
	rankID := storetest.RankID(t, db, "BJJ Adult", "White")

	graded, err := store.CreateAthlete(db, store.Athlete{FirstName: "Grace", LastName: "Hopper"})
	if err != nil {
		t.Fatalf("CreateAthlete: %v", err)
	}
	if _, err := store.CreatePromotion(db, store.Promotion{
		AthleteID: graded, RankID: rankID, PromotedOn: "2026-01-15",
	}); err != nil {
		t.Fatalf("CreatePromotion: %v", err)
	}

	if _, err := store.CreateAthlete(db, store.Athlete{
		FirstName: "Ada", LastName: "Lovelace", BirthDate: "morgen",
	}); err == nil {
		t.Fatal("CreateAthlete accepted a malformed birth date")
	}
	if _, err := store.CreatePromotion(db, store.Promotion{
		AthleteID: graded, RankID: rankID, PromotedOn: "morgen",
	}); err == nil {
		t.Fatal("CreatePromotion accepted a malformed date")
	}

	view, err := store.LoadRoster(db, store.RosterQuery{})
	if err != nil {
		t.Fatalf("LoadRoster after a refused write: %v", err)
	}
	if len(view.Rows) != 1 {
		t.Errorf("roster rows = %d, want 1 (only the valid athlete)", len(view.Rows))
	}
	athletes, err := store.ListAthletes(db, false)
	if err != nil {
		t.Fatalf("ListAthletes after a refused write: %v", err)
	}
	if len(athletes) != 1 {
		t.Errorf("athletes = %d, want 1", len(athletes))
	}
	if _, err := store.ListPromotions(db, graded); err != nil {
		t.Fatalf("ListPromotions after a refused write: %v", err)
	}
}

// The cases are the ones the callers actually meet: a German cell from a
// spreadsheet, an unpadded month from a hand-written form, an untrimmed cell, an
// empty optional column.
func TestIsDateAcceptsOnlyISOAndBlankOnlyAsOptional(t *testing.T) {
	tests := []struct {
		date        string
		want        bool
		wantOrBlank bool
	}{
		{"1990-12-10", true, true},
		{"", false, true},
		{"10.12.1990", false, false},
		{"1990-12-1", false, false},
		{"1990-13-45", false, false},
		{"morgen", false, false},
		{" 1990-12-10", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.date, func(t *testing.T) {
			if got := store.IsDate(tt.date); got != tt.want {
				t.Errorf("IsDate(%q) = %v, want %v", tt.date, got, tt.want)
			}
			if got := store.IsDateOrBlank(tt.date); got != tt.wantOrBlank {
				t.Errorf("IsDateOrBlank(%q) = %v, want %v", tt.date, got, tt.wantOrBlank)
			}
		})
	}
}

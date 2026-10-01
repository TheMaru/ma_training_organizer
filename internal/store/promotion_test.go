package store_test

import (
	"testing"

	"github.com/TheMaru/ma_training_organizer/internal/store"
	"github.com/TheMaru/ma_training_organizer/internal/store/storetest"
)

func TestCreateAndListPromotions(t *testing.T) {
	db := storetest.NewDB(t)

	athleteID, err := store.CreateAthlete(db, store.Athlete{FirstName: "Ada", LastName: "Lovelace"})
	if err != nil {
		t.Fatalf("CreateAthlete: %v", err)
	}

	white := storetest.RankID(t, db, "BJJ Adult", "White")
	blue := storetest.RankID(t, db, "BJJ Adult", "Blue")

	if _, err := store.CreatePromotion(db, store.Promotion{AthleteID: athleteID, RankID: white, PromotedOn: "2025-01-10"}); err != nil {
		t.Fatalf("CreatePromotion white: %v", err)
	}
	if _, err := store.CreatePromotion(db, store.Promotion{AthleteID: athleteID, RankID: blue, PromotedOn: "2026-03-01"}); err != nil {
		t.Fatalf("CreatePromotion blue: %v", err)
	}

	got, err := store.ListPromotions(db, athleteID)
	if err != nil {
		t.Fatalf("ListPromotions: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("promotions = %d, want 2", len(got))
	}
	// Most recent first, joined with rank + system names for display.
	if got[0].Rank.Name != "Blue" || got[0].Rank.System.Name != "BJJ Adult" || got[0].PromotedOn != "2026-03-01" {
		t.Errorf("promotions[0] = %+v, want Blue/BJJ Adult/2026-03-01", got[0])
	}
	if got[1].Rank.Name != "White" || got[1].PromotedOn != "2025-01-10" {
		t.Errorf("promotions[1] = %+v, want White/2025-01-10", got[1])
	}
}

func TestListPromotionsCarriesRankGroupAndDegree(t *testing.T) {
	db := storetest.NewDB(t)

	athleteID, err := store.CreateAthlete(db, store.Athlete{FirstName: "Mia", LastName: "Kind"})
	if err != nil {
		t.Fatalf("CreateAthlete: %v", err)
	}
	striped := storetest.RankID(t, db, "BJJ Kids", "Grey-White, 2 stripes")
	if _, err := store.CreatePromotion(db, store.Promotion{AthleteID: athleteID, RankID: striped, PromotedOn: "2026-02-02"}); err != nil {
		t.Fatalf("CreatePromotion: %v", err)
	}

	got, err := store.ListPromotions(db, athleteID)
	if err != nil {
		t.Fatalf("ListPromotions: %v", err)
	}
	// The descriptive breakdown travels with the row so a view can render the rank
	// without going back to the ranks table (ADR-0004).
	if got[0].Rank.Group != "Grey-White" || got[0].Rank.Degree != 2 {
		t.Errorf("promotions[0] group/degree = %q/%d, want Grey-White/2", got[0].Rank.Group, got[0].Rank.Degree)
	}
}

func TestCurrentRankPicksMostRecentByDate(t *testing.T) {
	// Out-of-order input, ranks skipped: the latest date wins regardless of slice
	// order or rank ordering.
	ps := []store.PromotionRow{
		{Promotion: store.Promotion{ID: 1, PromotedOn: "2025-01-10"}, Rank: store.Rank{Name: "White"}},
		{Promotion: store.Promotion{ID: 2, PromotedOn: "2025-09-01"}, Rank: store.Rank{Name: "White, 2 stripes"}},
		{Promotion: store.Promotion{ID: 3, PromotedOn: "2025-06-01"}, Rank: store.Rank{Name: "Blue"}},
	}
	got, ok := store.CurrentRank(ps)
	if !ok {
		t.Fatal("CurrentRank ok = false, want true")
	}
	if got.Rank.Name != "White, 2 stripes" {
		t.Errorf("current = %q, want White, 2 stripes", got.Rank.Name)
	}
}

func TestCurrentRankCrossesSystems(t *testing.T) {
	// Kids → adult: the adult rank is current when its date is latest, even though
	// it belongs to a different system.
	ps := []store.PromotionRow{
		{Promotion: store.Promotion{ID: 1, PromotedOn: "2024-05-01"}, Rank: store.Rank{Name: "Green", System: store.System{Name: "BJJ Kids"}}},
		{Promotion: store.Promotion{ID: 2, PromotedOn: "2026-02-01"}, Rank: store.Rank{Name: "White", System: store.System{Name: "BJJ Adult"}}},
	}
	got, ok := store.CurrentRank(ps)
	if !ok {
		t.Fatal("CurrentRank ok = false, want true")
	}
	if got.Rank.System.Name != "BJJ Adult" || got.Rank.Name != "White" {
		t.Errorf("current = %+v, want White/BJJ Adult", got)
	}
}

func TestCurrentRankSameDateTakesLatestRecorded(t *testing.T) {
	// Two promotions on one date: the more recently recorded (higher id) wins, so
	// a correction on the same day supersedes the earlier entry.
	ps := []store.PromotionRow{
		{Promotion: store.Promotion{ID: 5, PromotedOn: "2026-01-01"}, Rank: store.Rank{Name: "Blue"}},
		{Promotion: store.Promotion{ID: 9, PromotedOn: "2026-01-01"}, Rank: store.Rank{Name: "Purple"}},
	}
	got, _ := store.CurrentRank(ps)
	if got.Rank.Name != "Purple" {
		t.Errorf("current = %q, want Purple", got.Rank.Name)
	}
}

func TestCurrentRankEmpty(t *testing.T) {
	if _, ok := store.CurrentRank(nil); ok {
		t.Error("CurrentRank(nil) ok = true, want false")
	}
}

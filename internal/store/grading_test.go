package store_test

import (
	"testing"

	"github.com/TheMaru/ma_training_organizer/internal/store"
)

func TestListGradingSystemsGroupsRanksInOrder(t *testing.T) {
	db := seededDB(t)

	systems, err := store.ListGradingSystems(db)
	if err != nil {
		t.Fatalf("ListGradingSystems: %v", err)
	}

	// Both seeded systems come back, alphabetically by name.
	if len(systems) != 2 {
		t.Fatalf("systems = %d, want 2", len(systems))
	}
	if systems[0].Name != "BJJ Adult" || systems[1].Name != "BJJ Kids" {
		t.Errorf("system names = [%q %q], want [BJJ Adult, BJJ Kids]", systems[0].Name, systems[1].Name)
	}

	// Adult: 25 ranks, ordered by sort_order, carrying descriptive metadata.
	adult := systems[0]
	if len(adult.Ranks) != 25 {
		t.Fatalf("BJJ Adult ranks = %d, want 25", len(adult.Ranks))
	}
	first := adult.Ranks[0]
	if first.Name != "White" || first.Group != "White" || first.Degree != 0 {
		t.Errorf("first rank = %+v, want White/White/0", first)
	}
	last := adult.Ranks[len(adult.Ranks)-1]
	if last.Name != "Black, 4 stripes" || last.Group != "Black" || last.Degree != 4 {
		t.Errorf("last rank = %+v, want Black, 4 stripes/Black/4", last)
	}
	if first.ID == 0 {
		t.Error("rank ID = 0, want a real id for the promotion form")
	}
}

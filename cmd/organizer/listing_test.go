package main

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/TheMaru/ma_training_organizer/internal/store"
	"github.com/TheMaru/ma_training_organizer/internal/store/storetest"
	"github.com/TheMaru/ma_training_organizer/internal/trainer"
)

// theListing is what list-trainers renders: the store's answer to "who has
// access?", which this package asks directly because there is no rule in it (see
// cmdListTrainers).
func theListing(t *testing.T, db *sql.DB) trainerListing {
	t.Helper()
	trainers, err := store.ListTrainers(db)
	if err != nil {
		t.Fatalf("store.ListTrainers: %v", err)
	}
	return trainerListing(trainers)
}

// The listing is the only place either state is visible at all: the app never
// shows one trainer to another, so without this the operator is working blind.
func TestListTrainersShowsBothStates(t *testing.T) {
	db := storetest.NewDB(t)
	addTrainers(t, db, "ada", "grace")
	if err := trainer.Deactivate(db, "grace"); err != nil {
		t.Fatalf("trainer.Deactivate: %v", err)
	}
	if _, err := db.Exec(
		`UPDATE trainers SET deactivated_at = ? WHERE username = ?`, "2020-01-02 03:04:05", "grace",
	); err != nil {
		t.Fatalf("backdate deactivation: %v", err)
	}

	listing := theListing(t, db)

	if len(listing) != 2 {
		t.Fatalf("listed %d trainers, want 2", len(listing))
	}
	if listing[0].Username != "ada" || listing[0].Deactivated() {
		t.Errorf("first entry = %+v, want ada, active", listing[0])
	}
	if listing[1].Username != "grace" || !listing[1].Deactivated() {
		t.Errorf("second entry = %+v, want grace, deactivated", listing[1])
	}
	// "Since when did they lose access?" is the question the timestamp exists to
	// answer, so the rendered line has to carry the date and not just the state. The
	// date is backdated by hand rather than read back off the entry: an expectation
	// computed the way the code computes it would agree with any rendering at all.
	if got := listing.String(); !strings.Contains(got, "2020-01-02") {
		t.Errorf("listing does not carry the deactivation date:\n%s", got)
	}
}

// list-trainers takes no username, so an argument is a typo — most likely a
// username somebody expected it to filter by, which it does not do.
func TestListTrainersRejectsArguments(t *testing.T) {
	if err := cmdListTrainers(filepath.Join(t.TempDir(), "cli.db"), []string{"ada"}); err == nil {
		t.Error("cmdListTrainers accepted an argument")
	}
}

// A run against a database with no trainers in it is not an error, and printing
// nothing at all would read like one. The subcommand wrapper is driven here so the
// empty case is exercised end to end, opening included.
func TestListTrainersOnAnEmptyDatabase(t *testing.T) {
	if err := cmdListTrainers(filepath.Join(t.TempDir(), "cli.db"), nil); err != nil {
		t.Fatalf("cmdListTrainers: %v", err)
	}
	if got := trainerListing(nil).String(); !strings.Contains(got, "create-trainer") {
		t.Errorf("empty listing = %q, want it to name create-trainer", got)
	}
}

// Two runs of the listing have to be comparable, which insertion order is not:
// these are provisioned in the order nobody would read them in. "Zoe" is
// capitalised on purpose — under the database's default collation every capital
// sorts ahead of every lowercase letter, which is an order but not a legible one.
func TestListTrainersOrdersByUsername(t *testing.T) {
	db := storetest.NewDB(t)
	addTrainers(t, db, "Zoe", "ada", "mira", "bea")

	listing := theListing(t, db)

	var got []string
	for _, tr := range listing {
		got = append(got, tr.Username)
	}
	want := []string{"ada", "bea", "mira", "Zoe"}
	if !slices.Equal(got, want) {
		t.Errorf("order = %v, want %v", got, want)
	}
}

// What the listing may not carry, asserted directly rather than left implied by
// the tests above (see store.TrainerSummary for why it may not). Both the data and
// its rendering are searched, and the search is over every field the entries carry,
// so a hash arriving in some future column fails this too.
func TestListTrainersCarriesNoPasswordMaterial(t *testing.T) {
	db := storetest.NewDB(t)
	addTrainers(t, db, "ada", "grace")
	tr, err := store.TrainerByUsername(db, "ada")
	if err != nil {
		t.Fatalf("TrainerByUsername: %v", err)
	}

	listing := theListing(t, db)

	for _, subject := range []string{fmt.Sprintf("%+v", listing), listing.String()} {
		if strings.Contains(subject, tr.PasswordHash) {
			t.Errorf("listing carries the password hash: %s", subject)
		}
		// Not just the one hash: any argon2id string at all, whoever it belongs to.
		if strings.Contains(subject, "$argon2") {
			t.Errorf("listing carries argon2 material: %s", subject)
		}
		if strings.Contains(subject, trainerPassword) {
			t.Errorf("listing carries a plaintext password: %s", subject)
		}
	}
}

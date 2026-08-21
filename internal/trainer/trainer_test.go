package trainer_test

import (
	"database/sql"
	"testing"

	"github.com/TheMaru/ma_training_organizer/internal/store"
	"github.com/TheMaru/ma_training_organizer/internal/store/storetest"
	"github.com/TheMaru/ma_training_organizer/internal/trainer"
)

// trainerPassword is what every trainer these tests provision is given, so a
// password that turns out not to verify did so for the reason under test.
const trainerPassword = "correct-horse"

// club is a database holding these trainers, each provisioned through the act
// rather than written into the table — so what a test starts from is a state the
// Operator can actually produce. Most tests want at least two: the
// last-active-trainer rule refuses an act on the only one left.
func club(t *testing.T, usernames ...string) *sql.DB {
	t.Helper()
	db := storetest.NewDB(t)
	for _, u := range usernames {
		if err := trainer.Provision(db, u, trainerPassword); err != nil {
			t.Fatalf("trainer.Provision %q: %v", u, err)
		}
	}
	return db
}

// account is the trainer as they now stand, for the assertions about what an act
// left behind — or did not, when it was refused.
func account(t *testing.T, db *sql.DB, username string) store.Trainer {
	t.Helper()
	tr, err := store.TrainerByUsername(db, username)
	if err != nil {
		t.Fatalf("TrainerByUsername %q: %v", username, err)
	}
	return tr
}

// trainerID is what a question about Sessions is asked with, so it is read while
// the account is still there — see session.Manager.RevokeAll for why an act that
// removes the row does not take the Sessions with it.
func trainerID(t *testing.T, db *sql.DB, username string) int64 {
	t.Helper()
	return account(t, db, username).ID
}

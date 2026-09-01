package trainer_test

import (
	"database/sql"
	"testing"

	"github.com/TheMaru/ma_training_organizer/internal/store/storetest"
	"github.com/TheMaru/ma_training_organizer/internal/trainer/trainertest"
)

// club is a database holding these trainers; how many a test wants, and why they
// go through the act rather than into the table, is trainertest.Provision's.
func club(t *testing.T, usernames ...string) *sql.DB {
	t.Helper()
	db := storetest.NewDB(t)
	trainertest.Provision(t, db, usernames...)
	return db
}

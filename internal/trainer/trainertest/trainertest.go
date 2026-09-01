// Package trainertest provisions Trainer accounts for tests. Three suites need
// the same trainers, and one of them is package main — storetest's package doc
// gives the reason a fixture has to be its own package for that to work.
//
// It provisions through internal/trainer's acts, never store.CreateTrainer, so a
// test starts from a state the Operator can actually produce. That is also why it
// needs no place on TestTrainerIsTheOnlyPackageThatActsOnAnAccount's allowed list.
//
// What it does not do is make the database. Its callers want different ones:
// internal/trainer a bare one, internal/web a whole server round it, cmd/organizer
// a path the subcommands open themselves. It provisions into a database it is
// handed, which is the part all three share.
package trainertest

import (
	"database/sql"
	"testing"

	"github.com/TheMaru/ma_training_organizer/internal/store"
	"github.com/TheMaru/ma_training_organizer/internal/trainer"
)

// Password is what every trainer provisioned here logs in with, so a password that
// turns out not to verify did so for the reason under test.
const Password = "correct-horse"

// Provision puts each username on db as an active Trainer, all on Password. Most
// tests want at least two, because an Offboarding act refuses to take the club's
// last login away (trainer.ErrLastActiveTrainer): aimed at the only trainer on the
// database, deactivation and deletion are refused before they start.
func Provision(t *testing.T, db *sql.DB, usernames ...string) {
	t.Helper()
	for _, u := range usernames {
		if err := trainer.Provision(db, u, Password); err != nil {
			t.Fatalf("trainer.Provision %q: %v", u, err)
		}
	}
}

// Account is the trainer as they now stand, for the assertions about what an act
// left behind — or did not, when it was refused.
func Account(t *testing.T, db *sql.DB, username string) store.Trainer {
	t.Helper()
	tr, err := store.TrainerByUsername(db, username)
	if err != nil {
		t.Fatalf("TrainerByUsername %q: %v", username, err)
	}
	return tr
}

// ID is the id a Session records for username, read while the account is still
// there — see session.Manager.RevokeAll for why an act that removes the row does
// not take the Sessions with it.
func ID(t *testing.T, db *sql.DB, username string) int64 {
	t.Helper()
	return Account(t, db, username).ID
}

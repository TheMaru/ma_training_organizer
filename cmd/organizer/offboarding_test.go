package main

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/TheMaru/ma_training_organizer/internal/store"
	"github.com/TheMaru/ma_training_organizer/internal/trainer"
)

// trainerPassword is what every trainer these tests provision logs in with.
const trainerPassword = "correct-horse"

// addTrainers provisions trainers through the act the operator invokes, so a
// wiring test starts from the state a real database is in. Most tests here need
// at least two: the last-active-trainer rule refuses an act on the only one left.
func addTrainers(t *testing.T, db *sql.DB, usernames ...string) {
	t.Helper()
	for _, u := range usernames {
		if err := trainer.Provision(db, u, trainerPassword); err != nil {
			t.Fatalf("trainer.Provision %q: %v", u, err)
		}
	}
}

// The one act nothing gives back, so the operator is asked first — and a
// mistyped username survives an answer that is not a clear yes.
func TestDeleteTrainerLeavesTheAccountAloneWithoutAYes(t *testing.T) {
	for _, answer := range []string{"n\n", "\n", "", "sure\n"} {
		t.Run(fmt.Sprintf("%q", answer), func(t *testing.T) {
			path := cliDatabase(t, "ada", "grace")
			answering(t, answer)

			if err := cmdDeleteTrainer(path, []string{"grace"}); err != nil {
				t.Fatalf("cmdDeleteTrainer: %v", err)
			}

			withCLIDatabase(t, path, func(db *sql.DB) {
				if _, err := store.TrainerByUsername(db, "grace"); err != nil {
					t.Errorf("account gone after %q: %v", answer, err)
				}
			})
		})
	}
}

// The other half: a yes deletes, so the prompt is a question and not a wall.
func TestDeleteTrainerProceedsOnAYes(t *testing.T) {
	for _, answer := range []string{"y\n", "yes\n", "  Y  \n"} {
		t.Run(fmt.Sprintf("%q", answer), func(t *testing.T) {
			path := cliDatabase(t, "ada", "grace")
			answering(t, answer)

			if err := cmdDeleteTrainer(path, []string{"grace"}); err != nil {
				t.Fatalf("cmdDeleteTrainer: %v", err)
			}

			withCLIDatabase(t, path, func(db *sql.DB) {
				if _, err := store.TrainerByUsername(db, "grace"); !errors.Is(err, store.ErrTrainerNotFound) {
					t.Errorf("lookup after %q = %v, want ErrTrainerNotFound", answer, err)
				}
			})
		})
	}
}

// cliDatabase builds a database on a path, which is what a subcommand wrapper
// takes: it opens the database itself rather than being handed one.
func cliDatabase(t *testing.T, usernames ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "cli.db")
	withCLIDatabase(t, path, func(db *sql.DB) { addTrainers(t, db, usernames...) })
	return path
}

func withCLIDatabase(t *testing.T, path string, fn func(*sql.DB)) {
	t.Helper()
	if err := withDB(path, func(db *sql.DB) error {
		fn(db)
		return nil
	}); err != nil {
		t.Fatalf("withDB %s: %v", path, err)
	}
}

// answering stands in for the operator at the terminal: it points os.Stdin at a
// canned answer for the length of the test. confirm reads that global the way
// readHidden does, which is what keeps the prompt out of the core — the price is
// that a test using this one may not call t.Parallel.
func answering(t *testing.T, answer string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "answer")
	if err := os.WriteFile(path, []byte(answer), 0o600); err != nil {
		t.Fatalf("write answer: %v", err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open answer: %v", err)
	}
	saved := os.Stdin
	os.Stdin = f
	t.Cleanup(func() {
		os.Stdin = saved
		f.Close()
	})
}

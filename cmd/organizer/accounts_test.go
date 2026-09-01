package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TheMaru/ma_training_organizer/internal/store"
	"github.com/TheMaru/ma_training_organizer/internal/trainer/trainertest"
)

// TestCLIDatabaseCarriesTheGradingSystems is the defect the one-call open closes:
// every subcommand goes through withDB, which used to migrate without seeding, so
// a Trainer provisioned on a fresh database logged in to a promotion form with no
// GradingSystem to pick.
func TestCLIDatabaseCarriesTheGradingSystems(t *testing.T) {
	err := withDB(filepath.Join(t.TempDir(), "cli.db"), func(db *sql.DB) error {
		trainertest.Provision(t, db, "ada")
		systems, err := store.ListGradingSystems(db)
		if err != nil {
			return err
		}
		if len(systems) == 0 {
			t.Error("no grading systems on the database create-trainer opened")
		}
		// A system the form could offer but with no Rank in it is no Promotion a
		// Trainer could record.
		for _, s := range systems {
			if len(s.Ranks) == 0 {
				t.Errorf("grading system %q has no ranks", s.Name)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("withDB: %v", err)
	}
}

// What the subcommand prints is a rendering of the count, so the wording is
// assertable without capturing stdout — the shape trainerListing uses. None is
// the case worth pinning: it is an answer, not a failure.
func TestCountSessionsReadsAsAnAnswer(t *testing.T) {
	for n, want := range map[int]string{0: "no sessions", 1: "1 session", 3: "3 sessions"} {
		if got := countSessions(n); got != want {
			t.Errorf("countSessions(%d) = %q, want %q", n, got, want)
		}
	}
}

// Every subcommand that names a trainer takes exactly one, and answers with its
// usage line when it does not get one — a name made of spaces included, because
// that is a typo rather than a username. The check runs before anything else, so a
// mistyped command line leaves no database behind and never reaches a prompt.
func TestSubcommandsNeedExactlyOneUsername(t *testing.T) {
	// Read off the command table by its argument sketch rather than listed again
	// here, so a subcommand that comes to take a username is checked without
	// anybody remembering to add it.
	subcommands := 0
	path := filepath.Join(t.TempDir(), "cli.db")

	for _, c := range commands {
		if c.Args != "<username>" {
			continue
		}
		subcommands++
		for _, args := range [][]string{nil, {"   "}, {"ada", "grace"}} {
			t.Run(fmt.Sprintf("%s %v", c.Name, args), func(t *testing.T) {
				err := c.Run(path, args)
				if err == nil {
					t.Fatalf("%s accepted %v", c.Name, args)
				}
				if want := "usage: organizer " + c.Name; !strings.Contains(err.Error(), want) {
					t.Errorf("error = %q, want it to carry %q", err, want)
				}
			})
		}
	}
	// A sketch nobody spells "<username>" any more would leave this test passing
	// over nothing.
	if subcommands == 0 {
		t.Fatal("no subcommand in the table takes a username")
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("a database was opened for a command line that was never valid: %v", err)
	}
}

package main

import (
	"bufio"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"golang.org/x/term"

	"github.com/TheMaru/ma_training_organizer/internal/store"
	"github.com/TheMaru/ma_training_organizer/internal/trainer"
)

// trainerListing is every trainer as list-trainers reports them. It renders
// itself, which is what lets the subcommand's core return data and print nothing —
// the shape importReport already uses, and what makes the output assertable
// without capturing stdout. What it may not carry is store.TrainerSummary's to
// say.
type trainerListing []store.TrainerSummary

// String renders the listing, or names the way out of the one case where there is
// nothing to show. Dates are printed as stored, which is UTC: the deactivation
// date answers "since when?" to the day, and a local-time conversion would put the
// listing and the database a day apart for an operator reading it near midnight.
func (l trainerListing) String() string {
	if len(l) == 0 {
		return "no trainers yet — create one with create-trainer"
	}
	width := 0
	for _, tr := range l {
		if n := utf8.RuneCountInString(tr.Username); n > width {
			width = n
		}
	}
	var b strings.Builder
	for i, tr := range l {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(tr.Username)
		// Runes, not bytes: a username with an umlaut in it would otherwise pull the
		// whole column out of line.
		b.WriteString(strings.Repeat(" ", width-utf8.RuneCountInString(tr.Username)+2))
		if tr.Deactivated() {
			b.WriteString("deactivated since " + tr.DeactivatedAt.Format(store.ISODate))
		} else {
			b.WriteString("active")
		}
	}
	return b.String()
}

// cmdCreateTrainer wires the create-trainer subcommand: it opens the database,
// reads the username from args and the password from an interactive prompt, then
// provisions the account.
func cmdCreateTrainer(dbPath string, args []string) error {
	username, err := singleUsernameArg("create-trainer", args)
	if err != nil {
		return err
	}
	return withDB(dbPath, func(db *sql.DB) error {
		password, err := promptNewPassword()
		if err != nil {
			return err
		}
		if err := trainer.Provision(db, username, password); err != nil {
			return err
		}
		fmt.Printf("created trainer %q\n", username)
		return nil
	})
}

// cmdResetPassword wires the reset-password subcommand, mirroring
// cmdCreateTrainer but updating an existing trainer's password.
func cmdResetPassword(dbPath string, args []string) error {
	username, err := singleUsernameArg("reset-password", args)
	if err != nil {
		return err
	}
	return withDB(dbPath, func(db *sql.DB) error {
		password, err := promptNewPassword()
		if err != nil {
			return err
		}
		if err := trainer.ResetPassword(db, username, password); err != nil {
			return err
		}
		fmt.Printf("reset password for trainer %q\n", username)
		return nil
	})
}

// cmdRevokeSessions wires the revoke-sessions subcommand, the operator
// counterpart to the trainer's own control in the account area.
func cmdRevokeSessions(dbPath string, args []string) error {
	username, err := singleUsernameArg("revoke-sessions", args)
	if err != nil {
		return err
	}
	return withDB(dbPath, func(db *sql.DB) error {
		revoked, err := trainer.RevokeSessions(db, username)
		if err != nil {
			return err
		}
		fmt.Printf("revoked %s for trainer %q\n", countSessions(revoked), username)
		return nil
	})
}

// countSessions renders how many sessions an act ended. None is a real answer
// rather than a failure — the trainer was signed in nowhere — and saying so beats
// "0 sessions" for an operator working an incident.
func countSessions(n int) string {
	switch n {
	case 0:
		return "no sessions"
	case 1:
		return "1 session"
	}
	return fmt.Sprintf("%d sessions", n)
}

// cmdDeactivateTrainer wires the deactivate-trainer subcommand, the ordinary
// offboarding act: it needs no prompt, because it takes nothing away that
// reactivate-trainer cannot give back.
func cmdDeactivateTrainer(dbPath string, args []string) error {
	username, err := singleUsernameArg("deactivate-trainer", args)
	if err != nil {
		return err
	}
	return withDB(dbPath, func(db *sql.DB) error {
		if err := trainer.Deactivate(db, username); err != nil {
			return err
		}
		fmt.Printf("deactivated trainer %q: login refused from now on, sessions ended\n", username)
		return nil
	})
}

// cmdReactivateTrainer wires the reactivate-trainer subcommand, the way back.
func cmdReactivateTrainer(dbPath string, args []string) error {
	username, err := singleUsernameArg("reactivate-trainer", args)
	if err != nil {
		return err
	}
	return withDB(dbPath, func(db *sql.DB) error {
		if err := trainer.Reactivate(db, username); err != nil {
			return err
		}
		fmt.Printf("reactivated trainer %q: the password they had works again\n", username)
		return nil
	})
}

// cmdDeleteTrainer wires the delete-trainer subcommand. It asks before it acts,
// because this is the one act nothing gives back — and it asks here rather than in
// trainer.Delete, so the act stays non-interactive and testable.
//
// The question comes before the account is looked up: an operator who answers no
// has said no to whatever they typed, and a name that turns out not to exist is
// reported the same way afterwards either way.
func cmdDeleteTrainer(dbPath string, args []string) error {
	username, err := singleUsernameArg("delete-trainer", args)
	if err != nil {
		return err
	}
	return withDB(dbPath, func(db *sql.DB) error {
		confirmed, err := confirm(fmt.Sprintf(
			"Delete trainer %q? The account is gone for good; the roster is untouched.", username))
		if err != nil {
			return err
		}
		if !confirmed {
			fmt.Printf("left trainer %q alone\n", username)
			return nil
		}
		if err := trainer.Delete(db, username); err != nil {
			return err
		}
		fmt.Printf("deleted trainer %q: the account is gone, the username is free again\n", username)
		return nil
	})
}

// cmdListTrainers wires the list-trainers subcommand. It takes no username, so it
// rejects extra arguments the way the demo subcommands do.
//
// It reads the roster of accounts straight from the store: answering "who has
// access?" is a query with no rule in it, so there is no act for it to call
// (internal/trainer holds the acts). Only the rendering is this package's.
func cmdListTrainers(dbPath string, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: organizer list-trainers")
	}
	return withDB(dbPath, func(db *sql.DB) error {
		trainers, err := store.ListTrainers(db)
		if err != nil {
			return err
		}
		fmt.Println(trainerListing(trainers))
		return nil
	})
}

// withDB opens the database at dbPath — ready even on a first run, since Open
// migrates and seeds — invokes fn, and always closes the connection.
func withDB(dbPath string, fn func(*sql.DB) error) error {
	db, err := store.Open(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	return fn(db)
}

// singleUsernameArg extracts exactly one username from a subcommand's args.
func singleUsernameArg(cmd string, args []string) (string, error) {
	if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
		return "", fmt.Errorf("usage: organizer %s <username>", cmd)
	}
	return args[0], nil
}

// promptNewPassword reads a password from the terminal twice (without echoing)
// and returns it only if both entries match. Reading from the TTY keeps the
// password out of shell history and the process argument list.
func promptNewPassword() (string, error) {
	first, err := readHidden("New password: ")
	if err != nil {
		return "", err
	}
	second, err := readHidden("Confirm password: ")
	if err != nil {
		return "", err
	}
	if first != second {
		return "", errors.New("passwords do not match")
	}
	return first, nil
}

// confirm asks a yes-or-no question on the terminal and reports the answer. Only
// "y" or "yes" is a yes: anything else — a typo, a bare newline, a closed stdin —
// leaves the act undone, which is the safe way round for the one act that cannot
// be undone.
//
// The prompt goes to stderr like the password prompts, so a run whose output is
// being captured is not left waiting behind an invisible question.
func confirm(question string) (bool, error) {
	fmt.Fprintf(os.Stderr, "%s [y/N]: ", question)
	answer, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, fmt.Errorf("read confirmation: %w", err)
	}
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "y", "yes":
		return true, nil
	}
	return false, nil
}

// readHidden prints prompt and reads a line from the terminal without echoing.
func readHidden(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}
	return string(b), nil
}

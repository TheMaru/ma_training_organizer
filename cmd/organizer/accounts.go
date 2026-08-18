package main

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"golang.org/x/term"

	"github.com/TheMaru/ma_training_organizer/internal/auth"
	"github.com/TheMaru/ma_training_organizer/internal/session"
	"github.com/TheMaru/ma_training_organizer/internal/store"
)

// createTrainer provisions a new trainer with a hashed password. It is the
// testable core of the create-trainer subcommand: the CLI wrapper only supplies
// the username and prompts for the password.
func createTrainer(db *sql.DB, username, password string) error {
	if err := auth.ValidatePassword(password); err != nil {
		return err
	}
	hash, err := auth.Hash(password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	if _, err := store.CreateTrainer(db, username, hash); err != nil {
		return explainTakenUsername(db, username, err)
	}
	return nil
}

// explainTakenUsername says which kind of account holds the name, when the name is
// held at all. A deactivated one is invisible everywhere else in the app — no
// trainer sees another, and there is no web listing — so "already taken" on its own
// sends the operator hunting (ADR-0010). The error stays an ErrUsernameTaken, so a
// caller matching on it is unaffected.
//
// A lookup that fails here is swallowed on purpose: the caller's real answer is
// already in hand and only its wording was at stake.
func explainTakenUsername(db *sql.DB, username string, err error) error {
	if !errors.Is(err, store.ErrUsernameTaken) {
		return err
	}
	if tr, lookupErr := store.TrainerByUsername(db, username); lookupErr == nil && tr.Deactivated() {
		return fmt.Errorf("%w: %q belongs to a deactivated trainer — reactivate-trainer gives that account back",
			err, username)
	}
	return err
}

// errTrainerDeactivated is returned when an act is refused because the account
// cannot log in anyway. It names reactivation, because an operator working on a
// deactivated account often meant the homecoming rather than the act they typed.
var errTrainerDeactivated = errors.New("the account is deactivated — reactivate-trainer gives it back first")

// resetPassword sets a new password for an existing trainer. Like createTrainer,
// it is the testable core behind the reset-password subcommand.
//
// A deactivated trainer is refused: the new password would not let them in, so
// setting one is either a surprise waiting for the operator or the wrong command
// for what they meant (ADR-0010).
func resetPassword(db *sql.DB, username, password string) error {
	if err := auth.ValidatePassword(password); err != nil {
		return err
	}
	tr, err := store.TrainerByUsername(db, username)
	if err != nil {
		return err
	}
	if tr.Deactivated() {
		return fmt.Errorf("cannot reset the password of %q: %w", username, errTrainerDeactivated)
	}
	hash, err := auth.Hash(password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	return store.UpdateTrainerPassword(db, tr.ID, hash)
}

// revokeSessions ends every session a named trainer holds, the operator's path
// for a lost phone or a suspected takeover, and reports how many it ended. It is
// the testable core behind the revoke-sessions subcommand.
func revokeSessions(db *sql.DB, username string) (int, error) {
	tr, err := store.TrainerByUsername(db, username)
	if err != nil {
		return 0, err
	}
	return revokeAllSessions(db, tr.ID)
}

// revokeAllSessions ends every session a trainer holds, sparing none. Offboarding
// is a caller of session.Manager.RevokeAll, not a second copy of it.
func revokeAllSessions(db *sql.DB, trainerID int64) (int, error) {
	return session.ForCommand(db).RevokeAll(context.Background(), trainerID)
}

// errLastActiveTrainer is returned when an offboarding act would leave the club
// with nobody who can log in. There is no override flag: the alternative — every
// trainer locked out until somebody reaches a console — is not a trade worth
// offering (ADR-0010).
var errLastActiveTrainer = errors.New("that would leave the club with no trainer who can log in")

// refuseIfLastActiveTrainer refuses an act that would take the last login away.
// act names it for the message, which has to name the way through as well: an
// operator who is told only "no" has to guess.
//
// A trainer who is already deactivated cannot be the last one, so the act is
// permitted there whatever the count — that is what makes a second
// deactivate-trainer harmless, and what keeps an erasure request for somebody
// long departed from being refused.
func refuseIfLastActiveTrainer(db *sql.DB, act string, tr store.Trainer) error {
	if tr.Deactivated() {
		return nil
	}
	active, err := store.CountActiveTrainers(db)
	if err != nil {
		return err
	}
	if active <= 1 {
		return fmt.Errorf("cannot %s %q: %w — create the replacement with create-trainer first",
			act, tr.Username, errLastActiveTrainer)
	}
	return nil
}

// deactivateTrainer takes a departed trainer's access away without touching their
// account: login is refused from now on and the devices they are already signed
// in on lose their sessions. It is the testable core behind the
// deactivate-trainer subcommand, and it is reversible with reactivateTrainer.
func deactivateTrainer(db *sql.DB, username string) error {
	tr, err := store.TrainerByUsername(db, username)
	if err != nil {
		return err
	}
	if err := refuseIfLastActiveTrainer(db, "deactivate", tr); err != nil {
		return err
	}
	if err := store.DeactivateTrainer(db, tr.ID); err != nil {
		return err
	}
	// After the state change, so a failed deactivation does not sign anybody out.
	_, err = revokeAllSessions(db, tr.ID)
	return err
}

// reactivateTrainer gives a returning trainer their access back, with the
// password they always had. It is the testable core behind the
// reactivate-trainer subcommand and it does nothing else: no password reset, and
// the sessions deactivation ended stay ended (ADR-0010).
func reactivateTrainer(db *sql.DB, username string) error {
	tr, err := store.TrainerByUsername(db, username)
	if err != nil {
		return err
	}
	return store.ReactivateTrainer(db, tr.ID)
}

// deleteTrainer removes a trainer's account outright: the erasure act, and the
// exception rather than the ordinary offboarding one (ADR-0010). It is the
// testable core behind the delete-trainer subcommand, which is where the operator
// is asked to confirm.
//
// It works on any trainer, deactivated or not — see ADR-0010 for why nothing is
// gained by demanding the two acts in order.
func deleteTrainer(db *sql.DB, username string) error {
	tr, err := store.TrainerByUsername(db, username)
	if err != nil {
		return err
	}
	if err := refuseIfLastActiveTrainer(db, "delete", tr); err != nil {
		return err
	}
	if err := store.DeleteTrainer(db, tr.ID); err != nil {
		return err
	}
	// After the state change, as in deactivateTrainer. The sessions outlive the row
	// they belong to (see session.Manager.RevokeAll), so revoking them is a real act
	// here and not a formality.
	_, err = revokeAllSessions(db, tr.ID)
	return err
}

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

// listTrainers answers "who has access?", which nothing in the app itself does.
// It is the testable core behind the list-trainers subcommand: it returns the
// listing and prints none of it.
func listTrainers(db *sql.DB) (trainerListing, error) {
	trainers, err := store.ListTrainers(db)
	if err != nil {
		return nil, err
	}
	return trainerListing(trainers), nil
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
		if err := createTrainer(db, username, password); err != nil {
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
		if err := resetPassword(db, username, password); err != nil {
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
		revoked, err := revokeSessions(db, username)
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
		if err := deactivateTrainer(db, username); err != nil {
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
		if err := reactivateTrainer(db, username); err != nil {
			return err
		}
		fmt.Printf("reactivated trainer %q: the password they had works again\n", username)
		return nil
	})
}

// cmdDeleteTrainer wires the delete-trainer subcommand. It asks before it acts,
// because this is the one act nothing gives back — and it asks here rather than in
// deleteTrainer, so the core stays non-interactive and testable.
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
		if err := deleteTrainer(db, username); err != nil {
			return err
		}
		fmt.Printf("deleted trainer %q: the account is gone, the username is free again\n", username)
		return nil
	})
}

// cmdListTrainers wires the list-trainers subcommand. It takes no username, so it
// rejects extra arguments the way the demo subcommands do.
func cmdListTrainers(dbPath string, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: organizer list-trainers")
	}
	return withDB(dbPath, func(db *sql.DB) error {
		listing, err := listTrainers(db)
		if err != nil {
			return err
		}
		fmt.Println(listing)
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

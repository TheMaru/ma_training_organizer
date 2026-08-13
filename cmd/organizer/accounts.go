package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/TheMaru/ma_training_organizer/internal/auth"
	"github.com/TheMaru/ma_training_organizer/internal/store"
	"github.com/TheMaru/ma_training_organizer/internal/web"
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
		return err
	}
	return nil
}

// resetPassword sets a new password for an existing trainer. Like createTrainer,
// it is the testable core behind the reset-password subcommand.
func resetPassword(db *sql.DB, username, password string) error {
	if err := auth.ValidatePassword(password); err != nil {
		return err
	}
	tr, err := store.TrainerByUsername(db, username)
	if err != nil {
		return err
	}
	hash, err := auth.Hash(password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	return store.UpdateTrainerPassword(db, tr.ID, hash)
}

// revokeSessions ends every session a named trainer holds, the operator's path
// for a lost phone or a suspected takeover. It is the testable core behind the
// revoke-sessions subcommand.
func revokeSessions(db *sql.DB, username string) error {
	tr, err := store.TrainerByUsername(db, username)
	if err != nil {
		return err
	}
	return revokeAllSessions(db, tr.ID)
}

// revokeAllSessions ends every session a trainer holds, sparing none — see
// web.RevokeSessions for what the empty token means. Offboarding is a caller of
// that mechanism, not a second copy of it.
func revokeAllSessions(db *sql.DB, trainerID int64) error {
	return web.RevokeSessions(context.Background(), web.NewStoredSessions(db), trainerID, "")
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
// deactivate-trainer harmless.
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
	return revokeAllSessions(db, tr.ID)
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
		if err := revokeSessions(db, username); err != nil {
			return err
		}
		fmt.Printf("revoked all sessions for trainer %q\n", username)
		return nil
	})
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

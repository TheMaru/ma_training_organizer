package main

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/TheMaru/ma_training_organizer/internal/auth"
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

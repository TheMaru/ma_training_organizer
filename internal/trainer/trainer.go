// Package trainer owns the Operator's acts on a Trainer account: provisioning
// one, resetting a password, revoking its Sessions, and the Offboarding acts —
// deactivating, reactivating, deleting (CONTEXT.md, ADR-0010). The rules that go
// with those acts live here with them, so what "Deactivated" costs an account has
// one definition wherever it is asked from, and the side that enforces it can set
// its state up through the act that produces it.
//
// The refusals name CLI commands — "create-trainer first", "reactivate-trainer
// gives it back" — because the Operator is the reason these acts sit on the
// command line at all (ADR-0010), so the CLI is the only caller there will be.
// Splitting each message across two packages would buy a layering purity with no
// second consumer and turn one assertion into two halves. A second kind of caller
// is the moment to reconsider that, not before.
//
// The acts are free functions over a *sql.DB: there is nothing to hide behind a
// type here and no policy to state, so a constructor would be ceremony at every
// call site. Where a Session has to be ended, the act builds its own manager
// (see RevokeSessions) rather than asking its caller for one.
package trainer

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/TheMaru/ma_training_organizer/internal/auth"
	"github.com/TheMaru/ma_training_organizer/internal/store"
)

// ErrTrainerDeactivated is returned when an act is refused because the account
// cannot log in anyway. It names reactivation, because an operator working on a
// deactivated account often meant the homecoming rather than the act they typed.
var ErrTrainerDeactivated = errors.New("the account is deactivated — reactivate-trainer gives it back first")

// Provision creates a new trainer with a hashed password. It is the Operator's
// only way in: accounts are provisioned out-of-band, never by self-registration
// (CONTEXT.md).
func Provision(db *sql.DB, username, password string) error {
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

// ResetPassword sets a new password for an existing trainer, the Operator's
// answer to one that was forgotten.
//
// A deactivated trainer is refused: the new password would not let them in, so
// setting one is either a surprise waiting for the operator or the wrong command
// for what they meant (ADR-0010).
func ResetPassword(db *sql.DB, username, password string) error {
	if err := auth.ValidatePassword(password); err != nil {
		return err
	}
	tr, err := store.TrainerByUsername(db, username)
	if err != nil {
		return err
	}
	if tr.Deactivated() {
		return fmt.Errorf("cannot reset the password of %q: %w", username, ErrTrainerDeactivated)
	}
	hash, err := auth.Hash(password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	return store.UpdateTrainerPassword(db, tr.ID, hash)
}

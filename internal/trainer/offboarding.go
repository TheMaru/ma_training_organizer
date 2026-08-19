package trainer

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/TheMaru/ma_training_organizer/internal/store"
)

// ErrLastActiveTrainer is returned when an Offboarding act would leave the club
// with nobody who can log in. There is no override flag: the alternative — every
// trainer locked out until somebody reaches a console — is not a trade worth
// offering (ADR-0010).
var ErrLastActiveTrainer = errors.New("that would leave the club with no trainer who can log in")

// refuseIfLastActiveTrainer refuses an act that would take the last login away.
// act names it for the message, which has to name the way through as well: an
// operator who is told only "no" has to guess.
//
// A trainer who is already deactivated cannot be the last one, so the act is
// permitted there whatever the count — that is what makes a second Deactivate
// harmless, and what keeps an erasure request for somebody long departed from
// being refused.
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
			act, tr.Username, ErrLastActiveTrainer)
	}
	return nil
}

// Deactivate takes a departed trainer's access away without touching their
// account: login is refused from now on and the devices they are already signed
// in on lose their Sessions. It is the ordinary Offboarding act, and it is
// reversible with Reactivate (ADR-0010).
func Deactivate(db *sql.DB, username string) error {
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
	_, err = revokeAll(db, tr.ID)
	return err
}

// Reactivate gives a returning trainer their access back, with the password they
// always had. It does nothing else: no password reset, and the Sessions
// deactivation ended stay ended (ADR-0010).
func Reactivate(db *sql.DB, username string) error {
	tr, err := store.TrainerByUsername(db, username)
	if err != nil {
		return err
	}
	return store.ReactivateTrainer(db, tr.ID)
}

// Delete removes a trainer's account outright: the erasure act, and the exception
// rather than the ordinary Offboarding one (ADR-0010).
//
// It works on any trainer, deactivated or not — see ADR-0010 for why nothing is
// gained by demanding the two acts in order. The confirmation an operator is
// asked for lives with the subcommand, so this stays non-interactive.
func Delete(db *sql.DB, username string) error {
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
	// After the state change, as in Deactivate. The Sessions outlive the row they
	// belong to (see session.Manager.RevokeAll), so revoking them is a real act
	// here and not a formality.
	_, err = revokeAll(db, tr.ID)
	return err
}

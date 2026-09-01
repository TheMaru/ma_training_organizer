package trainer_test

import (
	"errors"
	"testing"

	"github.com/TheMaru/ma_training_organizer/internal/session/sessiontest"
	"github.com/TheMaru/ma_training_organizer/internal/store"
	"github.com/TheMaru/ma_training_organizer/internal/trainer"
	"github.com/TheMaru/ma_training_organizer/internal/trainer/trainertest"
)

// The operator's path spares nothing — including the device they might be holding
// themselves, which is what separates it from the trainer's own control in the
// account area. The colleague at the same club keeps theirs.
//
// The Sessions are started through a server's manager while the act builds its
// own over the same store: that is the real arrangement, a command reaching what a
// running server wrote.
func TestRevokeSessionsEndsEveryDevice(t *testing.T) {
	db := club(t, "ada", "grace")
	sessions := sessiontest.NewManager(t, db)
	ada, grace := trainertest.ID(t, db, "ada"), trainertest.ID(t, db, "grace")
	sessiontest.SignIn(t, sessions, ada)
	sessiontest.SignIn(t, sessions, ada)
	sessiontest.SignIn(t, sessions, grace)

	revoked, err := trainer.RevokeSessions(db, "ada")
	if err != nil {
		t.Fatalf("trainer.RevokeSessions: %v", err)
	}

	// The count is what the operator is told, so it is asserted rather than inferred
	// from the sessions being gone.
	if revoked != 2 {
		t.Errorf("revoked = %d, want 2", revoked)
	}
	if got := sessiontest.Count(t, sessions, ada); got != 0 {
		t.Errorf("the trainer still holds %d sessions", got)
	}
	if got := sessiontest.Count(t, sessions, grace); got != 1 {
		t.Errorf("the colleague holds %d sessions, want 1", got)
	}
}

// Revocation stays permitted on a deactivated trainer, and stays revocation: an
// operator working an incident should not have to reason about command order, and a
// subcommand that quietly did nothing would be worse than a refusal.
//
// The account is deactivated through the store rather than through the act, which
// revokes as it goes — that would leave nothing for the act under test to end.
func TestRevokeSessionsWorksOnADeactivatedTrainer(t *testing.T) {
	db := club(t, "ada", "grace")
	sessions := sessiontest.NewManager(t, db)
	grace := trainertest.ID(t, db, "grace")
	sessiontest.SignIn(t, sessions, grace)
	if err := store.DeactivateTrainer(db, grace); err != nil {
		t.Fatalf("DeactivateTrainer: %v", err)
	}

	revoked, err := trainer.RevokeSessions(db, "grace")
	if err != nil {
		t.Fatalf("trainer.RevokeSessions on a deactivated trainer: %v", err)
	}

	if revoked != 1 {
		t.Errorf("revoked = %d, want 1", revoked)
	}
	if got := sessiontest.Count(t, sessions, grace); got != 0 {
		t.Errorf("the trainer still holds %d sessions", got)
	}
}

func TestRevokeSessionsUnknownTrainer(t *testing.T) {
	db := club(t, "ada")

	_, err := trainer.RevokeSessions(db, "ghost")

	if !errors.Is(err, store.ErrTrainerNotFound) {
		t.Errorf("error = %v, want ErrTrainerNotFound", err)
	}
}

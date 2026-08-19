package trainer_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/TheMaru/ma_training_organizer/internal/auth"
	"github.com/TheMaru/ma_training_organizer/internal/store"
	"github.com/TheMaru/ma_training_organizer/internal/store/storetest"
	"github.com/TheMaru/ma_training_organizer/internal/trainer"
)

func TestProvisionStoresAPasswordThatVerifies(t *testing.T) {
	db := storetest.NewDB(t)

	if err := trainer.Provision(db, "ada", trainerPassword); err != nil {
		t.Fatalf("trainer.Provision: %v", err)
	}

	tr := account(t, db, "ada")
	if ok, _ := auth.Verify(trainerPassword, tr.PasswordHash); !ok {
		t.Error("provisioned password does not verify")
	}
	// The stored value is a hash, not the plaintext.
	if tr.PasswordHash == trainerPassword {
		t.Error("password stored in plaintext")
	}
}

func TestProvisionRejectsShortPassword(t *testing.T) {
	db := storetest.NewDB(t)

	err := trainer.Provision(db, "ada", "short")

	if !errors.Is(err, auth.ErrPasswordTooShort) {
		t.Errorf("error = %v, want auth.ErrPasswordTooShort", err)
	}
	if _, err := store.TrainerByUsername(db, "ada"); !errors.Is(err, store.ErrTrainerNotFound) {
		t.Error("trainer was created despite a too-short password")
	}
}

func TestProvisionRejectsATakenUsername(t *testing.T) {
	db := club(t, "dup")

	err := trainer.Provision(db, "dup", "another-horse")

	if !errors.Is(err, store.ErrUsernameTaken) {
		t.Errorf("error = %v, want ErrUsernameTaken", err)
	}
}

// create-trainer is where an operator meets a name they cannot see anywhere else:
// the account exists, deactivated, and "already taken" alone would send them
// hunting for it (ADR-0010).
func TestProvisionReportsADeactivatedAccount(t *testing.T) {
	db := club(t, "ada", "grace")
	if err := trainer.Deactivate(db, "grace"); err != nil {
		t.Fatalf("trainer.Deactivate: %v", err)
	}

	err := trainer.Provision(db, "grace", "another-horse")

	// Still the taken-username case, so a caller matching on it keeps working.
	if !errors.Is(err, store.ErrUsernameTaken) {
		t.Fatalf("error = %v, want ErrUsernameTaken", err)
	}
	if !strings.Contains(err.Error(), "deactivated") {
		t.Errorf("message %q does not say the account is deactivated", err)
	}
	if !strings.Contains(err.Error(), "reactivate-trainer") {
		t.Errorf("message %q does not name the way to that account", err)
	}
}

// The taken-username message only names deactivation when that is true, or the
// operator is sent to reactivate an account that is already active.
func TestProvisionAgainstAnActiveNameStaysPlain(t *testing.T) {
	db := club(t, "ada")

	err := trainer.Provision(db, "ada", "another-horse")

	if !errors.Is(err, store.ErrUsernameTaken) {
		t.Fatalf("error = %v, want ErrUsernameTaken", err)
	}
	if strings.Contains(err.Error(), "deactivated") {
		t.Errorf("message %q calls an active account deactivated", err)
	}
}

func TestResetPasswordReplacesTheHash(t *testing.T) {
	db := club(t, "grace")

	if err := trainer.ResetPassword(db, "grace", "new-secret-2"); err != nil {
		t.Fatalf("trainer.ResetPassword: %v", err)
	}

	tr := account(t, db, "grace")
	if ok, _ := auth.Verify("new-secret-2", tr.PasswordHash); !ok {
		t.Error("new password does not verify after the reset")
	}
	if ok, _ := auth.Verify(trainerPassword, tr.PasswordHash); ok {
		t.Error("old password still verifies after the reset")
	}
}

func TestResetPasswordRejectsShortPassword(t *testing.T) {
	db := club(t, "grace")

	if err := trainer.ResetPassword(db, "grace", "short"); !errors.Is(err, auth.ErrPasswordTooShort) {
		t.Errorf("error = %v, want auth.ErrPasswordTooShort", err)
	}
}

// An operator resetting a deactivated trainer's password is working on an account
// that cannot log in either way. Refused, and told the verb for what they may
// actually have meant.
func TestResetPasswordRefusesADeactivatedTrainer(t *testing.T) {
	db := club(t, "ada", "grace")
	if err := trainer.Deactivate(db, "grace"); err != nil {
		t.Fatalf("trainer.Deactivate: %v", err)
	}
	before := account(t, db, "grace")

	err := trainer.ResetPassword(db, "grace", "brand-new-secret")

	if !errors.Is(err, trainer.ErrTrainerDeactivated) {
		t.Fatalf("error = %v, want trainer.ErrTrainerDeactivated", err)
	}
	if !strings.Contains(err.Error(), "reactivate-trainer") {
		t.Errorf("refusal %q does not name reactivation", err)
	}
	// Refused, not half-applied: the old hash stands, so the password the operator
	// typed is no way in either. That it is no way in at a login is web's to assert.
	if got := account(t, db, "grace").PasswordHash; got != before.PasswordHash {
		t.Error("password hash changed despite the refusal")
	}
}

func TestResetPasswordUnknownTrainer(t *testing.T) {
	db := club(t, "ada")

	err := trainer.ResetPassword(db, "ghost", "some-secret-9")

	if !errors.Is(err, store.ErrTrainerNotFound) {
		t.Errorf("error = %v, want ErrTrainerNotFound", err)
	}
}

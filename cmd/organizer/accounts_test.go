package main

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/TheMaru/ma_training_organizer/internal/auth"
	"github.com/TheMaru/ma_training_organizer/internal/store"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "cli.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return db
}

func TestCreateTrainerProducesWorkingLogin(t *testing.T) {
	db := newTestDB(t)

	if err := createTrainer(db, "ada", "correct-horse"); err != nil {
		t.Fatalf("createTrainer: %v", err)
	}

	tr, err := store.TrainerByUsername(db, "ada")
	if err != nil {
		t.Fatalf("TrainerByUsername: %v", err)
	}
	if ok, _ := auth.Verify("correct-horse", tr.PasswordHash); !ok {
		t.Error("provisioned password does not verify")
	}
	// The stored value is a hash, not the plaintext.
	if tr.PasswordHash == "correct-horse" {
		t.Error("password stored in plaintext")
	}
}

func TestCreateTrainerRejectsShortPassword(t *testing.T) {
	db := newTestDB(t)

	err := createTrainer(db, "ada", "short")
	if !errors.Is(err, auth.ErrPasswordTooShort) {
		t.Errorf("error = %v, want auth.ErrPasswordTooShort", err)
	}
	if _, err := store.TrainerByUsername(db, "ada"); !errors.Is(err, store.ErrTrainerNotFound) {
		t.Error("trainer was created despite a too-short password")
	}
}

func TestCreateTrainerRejectsDuplicate(t *testing.T) {
	db := newTestDB(t)

	if err := createTrainer(db, "dup", "correct-horse"); err != nil {
		t.Fatalf("first createTrainer: %v", err)
	}
	err := createTrainer(db, "dup", "another-horse")
	if !errors.Is(err, store.ErrUsernameTaken) {
		t.Errorf("error = %v, want ErrUsernameTaken", err)
	}
}

func TestResetPasswordChangesLogin(t *testing.T) {
	db := newTestDB(t)

	if err := createTrainer(db, "grace", "old-secret-1"); err != nil {
		t.Fatalf("createTrainer: %v", err)
	}
	if err := resetPassword(db, "grace", "new-secret-2"); err != nil {
		t.Fatalf("resetPassword: %v", err)
	}

	tr, err := store.TrainerByUsername(db, "grace")
	if err != nil {
		t.Fatalf("TrainerByUsername: %v", err)
	}
	if ok, _ := auth.Verify("new-secret-2", tr.PasswordHash); !ok {
		t.Error("new password does not verify after reset")
	}
	if ok, _ := auth.Verify("old-secret-1", tr.PasswordHash); ok {
		t.Error("old password still verifies after reset")
	}
}

func TestResetPasswordUnknownTrainer(t *testing.T) {
	db := newTestDB(t)

	err := resetPassword(db, "ghost", "some-secret-9")
	if !errors.Is(err, store.ErrTrainerNotFound) {
		t.Errorf("error = %v, want ErrTrainerNotFound", err)
	}
}

func TestResetPasswordRejectsShortPassword(t *testing.T) {
	db := newTestDB(t)

	if err := createTrainer(db, "grace", "old-secret-1"); err != nil {
		t.Fatalf("createTrainer: %v", err)
	}
	if err := resetPassword(db, "grace", "short"); !errors.Is(err, auth.ErrPasswordTooShort) {
		t.Errorf("error = %v, want auth.ErrPasswordTooShort", err)
	}
}

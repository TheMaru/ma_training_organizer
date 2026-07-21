package store_test

import (
	"errors"
	"testing"

	"github.com/TheMaru/ma_training_organizer/internal/store"
)

func TestCreateAndGetTrainerByUsername(t *testing.T) {
	db := newTestDB(t)

	id, err := store.CreateTrainer(db, "ada", "hash-1")
	if err != nil {
		t.Fatalf("CreateTrainer: %v", err)
	}
	if id == 0 {
		t.Fatal("CreateTrainer returned zero id")
	}

	got, err := store.TrainerByUsername(db, "ada")
	if err != nil {
		t.Fatalf("TrainerByUsername: %v", err)
	}
	if got.ID != id {
		t.Errorf("ID = %d, want %d", got.ID, id)
	}
	if got.Username != "ada" {
		t.Errorf("Username = %q, want %q", got.Username, "ada")
	}
	if got.PasswordHash != "hash-1" {
		t.Errorf("PasswordHash = %q, want %q", got.PasswordHash, "hash-1")
	}
}

func TestTrainerByIDRoundTrips(t *testing.T) {
	db := newTestDB(t)

	id, err := store.CreateTrainer(db, "grace", "hash-g")
	if err != nil {
		t.Fatalf("CreateTrainer: %v", err)
	}

	got, err := store.TrainerByID(db, id)
	if err != nil {
		t.Fatalf("TrainerByID: %v", err)
	}
	if got.Username != "grace" {
		t.Errorf("Username = %q, want %q", got.Username, "grace")
	}
}

func TestTrainerByUsernameNotFound(t *testing.T) {
	db := newTestDB(t)

	_, err := store.TrainerByUsername(db, "ghost")
	if !errors.Is(err, store.ErrTrainerNotFound) {
		t.Errorf("error = %v, want ErrTrainerNotFound", err)
	}
}

func TestTrainerByIDNotFound(t *testing.T) {
	db := newTestDB(t)

	_, err := store.TrainerByID(db, 404)
	if !errors.Is(err, store.ErrTrainerNotFound) {
		t.Errorf("error = %v, want ErrTrainerNotFound", err)
	}
}

func TestCreateTrainerRejectsDuplicateUsername(t *testing.T) {
	db := newTestDB(t)

	if _, err := store.CreateTrainer(db, "dup", "h1"); err != nil {
		t.Fatalf("first CreateTrainer: %v", err)
	}

	_, err := store.CreateTrainer(db, "dup", "h2")
	if !errors.Is(err, store.ErrUsernameTaken) {
		t.Errorf("error = %v, want ErrUsernameTaken", err)
	}
}

func TestUpdateTrainerPassword(t *testing.T) {
	db := newTestDB(t)

	id, err := store.CreateTrainer(db, "changeme", "old-hash")
	if err != nil {
		t.Fatalf("CreateTrainer: %v", err)
	}

	if err := store.UpdateTrainerPassword(db, id, "new-hash"); err != nil {
		t.Fatalf("UpdateTrainerPassword: %v", err)
	}

	got, err := store.TrainerByID(db, id)
	if err != nil {
		t.Fatalf("TrainerByID: %v", err)
	}
	if got.PasswordHash != "new-hash" {
		t.Errorf("PasswordHash = %q, want %q", got.PasswordHash, "new-hash")
	}
}

func TestUpdateTrainerPasswordUnknownID(t *testing.T) {
	db := newTestDB(t)

	err := store.UpdateTrainerPassword(db, 999, "whatever")
	if !errors.Is(err, store.ErrTrainerNotFound) {
		t.Errorf("error = %v, want ErrTrainerNotFound", err)
	}
}

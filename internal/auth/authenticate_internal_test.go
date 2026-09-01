package auth

import (
	"errors"
	"testing"
	"time"

	"github.com/TheMaru/ma_training_organizer/internal/store"
)

// internal/web's tests hold that the three refusals give one status and one body.
// What they cannot see is the argon2 work behind that answer, which is the one
// axis the cases could still differ on, so this counts the password checks.
//
// Both ways to break it show up as a count of zero: reading Deactivated before
// the check, and dropping the decoy. Neither changes anything else observable.
//
// The success row is not decoration — without it a function that always refused
// would pass every other row.
func TestEveryLoginChecksExactlyOnePassword(t *testing.T) {
	const password = "correct horse battery staple"
	hash, err := Hash(password)
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	active := store.Trainer{ID: 1, Username: "ada", PasswordHash: hash}
	departed := store.Trainer{ID: 2, Username: "grace", PasswordHash: hash, DeactivatedAt: time.Now()}

	tests := []struct {
		name     string
		tr       store.Trainer
		found    bool
		password string
		wantErr  error
		wantID   int64
	}{
		{"unknown username", store.Trainer{}, false, password, ErrBadCredentials, 0},
		{"wrong password", active, true, "not-the-password", ErrBadCredentials, 0},
		{"deactivated account", departed, true, password, ErrBadCredentials, 0},
		{"correct password", active, true, password, nil, active.ID},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checks := 0
			check := func(plain, stored string) (bool, error) {
				checks++
				return verify(plain, stored)
			}

			got, err := authenticate(tt.tr, tt.found, tt.password, check)

			if checks != 1 {
				t.Errorf("password checks = %d, want 1", checks)
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
			if got.ID != tt.wantID {
				t.Errorf("trainer id = %d, want %d", got.ID, tt.wantID)
			}
		})
	}
}

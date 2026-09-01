// Package auth owns authentication: turning a username and a password into a
// Trainer who may sign in, turning a Session's trainer id into a Trainer who may
// use the app, and letting a Trainer change their own password. The password
// primitives the rest of the app shares live here too — the length policy and the
// hash, which the provisioning CLI uses as well. It wraps alexedwards/argon2id
// (the algorithm chosen in ADR-0002) so nothing else depends on it, and so the
// parameters live in one place.
//
// This is the only place a password is checked: verify is unexported, so that
// claim is held by the compiler rather than by a comment. A handler that wanted
// to check one would have to move the decision in here first.
//
// A refused login has to be indistinguishable from a wrong password (ADR-0010,
// CONTEXT.md on Deactivated), which costs Authenticate a decoy hash and an
// ordering. Each is stated where it stands, at decoyHash and in authenticate.
package auth

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/TheMaru/ma_training_organizer/internal/store"
)

// ErrBadCredentials is the one answer a failed login gets, whether the username
// is unknown, the password is wrong or the account is deactivated.
var ErrBadCredentials = errors.New("auth: bad credentials")

// ErrCurrentPasswordWrong is returned by ChangePassword when the current password
// does not verify. Callers match it with errors.Is.
var ErrCurrentPasswordWrong = errors.New("auth: current password is wrong")

// decoyHash is a valid argon2id hash of a throwaway value. authenticate verifies
// against it when the username is unknown, so a login attempt performs the same
// argon2 work whether or not the account exists — closing the timing side-channel
// that would otherwise reveal which usernames are registered.
var decoyHash = mustDecoyHash()

func mustDecoyHash() string {
	h, err := Hash("decoy: no trainer will ever have this password")
	if err != nil {
		panic("auth: precompute decoy password hash: " + err.Error())
	}
	return h
}

// Authenticate is the sign-in decision: the Trainer to sign in, or
// ErrBadCredentials. It starts no Session — that is internal/web's act, through
// session.Manager — so the name promises only what happens here.
//
// A database that could not answer is an error of its own, distinct from a
// refusal, so the caller can send a 500 rather than a login page that lies.
func Authenticate(db *sql.DB, username, password string) (store.Trainer, error) {
	tr, err := store.TrainerByUsername(db, username)
	switch {
	case errors.Is(err, store.ErrTrainerNotFound):
		return authenticate(store.Trainer{}, false, password, verify)
	case err != nil:
		return store.Trainer{}, fmt.Errorf("look up trainer: %w", err)
	}
	return authenticate(tr, true, password, verify)
}

// authenticate decides a login from a Trainer already looked up, with the check
// passed in. Pure, so the ordering below is testable by counting the checks: see
// TestEveryLoginChecksExactlyOnePassword.
func authenticate(tr store.Trainer, found bool, password string, check func(plain, hash string) (bool, error)) (store.Trainer, error) {
	hash := tr.PasswordHash
	if !found {
		hash = decoyHash
	}
	ok, err := check(password, hash)
	if err != nil {
		return store.Trainer{}, fmt.Errorf("verify password: %w", err)
	}
	// The Deactivated read comes after the check, never before it, so the refusal
	// costs the same argon2 work as a wrong password. !found is here for the same
	// reason, and is load-bearing besides: whoever submits the decoy's own
	// plaintext gets a match against it.
	if !ok || !found || tr.Deactivated() {
		return store.Trainer{}, ErrBadCredentials
	}
	return tr, nil
}

// TrainerMayUseTheApp says what the id in a Session is worth: the Trainer it
// names, and whether that Trainer may use the app at all. An account that is gone
// and one that is deactivated are both a plain "no" rather than an error; only a
// database that could not answer is one (ADR-0010).
//
// It returns the Trainer as well as the verdict so one read serves every question
// a request has about them.
func TrainerMayUseTheApp(db *sql.DB, id int64) (store.Trainer, bool, error) {
	tr, err := store.TrainerByID(db, id)
	switch {
	case errors.Is(err, store.ErrTrainerNotFound):
		return store.Trainer{}, false, nil
	case err != nil:
		return store.Trainer{}, false, fmt.Errorf("look up trainer: %w", err)
	}
	if tr.Deactivated() {
		return store.Trainer{}, false, nil
	}
	return tr, true, nil
}

// ChangePassword is the Trainer's own act on their own credentials: the current
// password has to verify, the new one has to satisfy the shared length policy.
// It takes the Trainer rather than an id because its caller already holds one,
// loaded once for this request.
//
// Whether the new password was typed the same way twice is not a rule about
// credentials — it compares two form fields, and stays with the form.
func ChangePassword(db *sql.DB, tr store.Trainer, current, next string) error {
	ok, err := verify(current, tr.PasswordHash)
	if err != nil {
		return fmt.Errorf("verify current password: %w", err)
	}
	if !ok {
		return ErrCurrentPasswordWrong
	}
	if err := ValidatePassword(next); err != nil {
		return err
	}
	hash, err := Hash(next)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	return store.UpdateTrainerPassword(db, tr.ID, hash)
}

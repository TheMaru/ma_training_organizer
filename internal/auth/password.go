package auth

import (
	"fmt"

	"github.com/alexedwards/argon2id"
)

// MinPasswordLength is the shortest password the app accepts, applied uniformly
// by the self-service change form and the provisioning CLI so no path can set a
// weaker one.
const MinPasswordLength = 8

// ErrPasswordTooShort is returned by ValidatePassword when a password is below
// MinPasswordLength. Callers match it with errors.Is.
var ErrPasswordTooShort = fmt.Errorf("auth: password must be at least %d characters", MinPasswordLength)

// ValidatePassword enforces the shared password policy. It is the single place
// the length rule lives, so the change form and the provisioning CLI can never
// drift apart.
func ValidatePassword(plain string) error {
	if len(plain) < MinPasswordLength {
		return ErrPasswordTooShort
	}
	return nil
}

// Hash derives a storable, salted argon2id hash of a plaintext password. The
// returned string is self-describing (it encodes the algorithm, parameters and
// salt), so verify needs nothing but the password and this string.
func Hash(plain string) (string, error) {
	return argon2id.CreateHash(plain, argon2id.DefaultParams)
}

// verify reports whether plain matches the previously stored hash. It returns a
// non-nil error only when hash is malformed (not for a simple mismatch), so a
// corrupt stored hash is never mistaken for a wrong password.
//
// Unexported: the acts in this package are the only callers, which is what makes
// "a password is checked here and nowhere else" a compile error to break rather
// than a rule to remember.
func verify(plain, hash string) (bool, error) {
	return argon2id.ComparePasswordAndHash(plain, hash)
}

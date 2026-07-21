package auth_test

import (
	"testing"

	"github.com/TheMaru/ma_training_organizer/internal/auth"
)

func TestHashThenVerifyAcceptsCorrectPassword(t *testing.T) {
	hash, err := auth.Hash("correct horse battery staple")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if hash == "" {
		t.Fatal("Hash returned empty string")
	}

	ok, err := auth.Verify("correct horse battery staple", hash)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !ok {
		t.Error("Verify rejected the correct password")
	}
}

func TestVerifyRejectsWrongPassword(t *testing.T) {
	hash, err := auth.Hash("s3cret")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}

	ok, err := auth.Verify("not the password", hash)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if ok {
		t.Error("Verify accepted a wrong password")
	}
}

// A hash embeds a random salt, so hashing the same password twice must yield
// different encoded strings — otherwise identical passwords would be linkable.
func TestHashIsSalted(t *testing.T) {
	a, err := auth.Hash("same")
	if err != nil {
		t.Fatalf("Hash a: %v", err)
	}
	b, err := auth.Hash("same")
	if err != nil {
		t.Fatalf("Hash b: %v", err)
	}
	if a == b {
		t.Error("two hashes of the same password are identical; salt missing")
	}
}

// A garbage hash must surface an error rather than silently reporting no match,
// so callers can distinguish "wrong password" from "corrupt stored hash".
func TestVerifyErrorsOnMalformedHash(t *testing.T) {
	if _, err := auth.Verify("whatever", "not-a-real-hash"); err == nil {
		t.Error("expected an error for a malformed hash, got nil")
	}
}

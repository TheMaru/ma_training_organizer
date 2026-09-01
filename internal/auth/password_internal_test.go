package auth

import (
	"testing"
)

func TestHashThenVerifyAcceptsCorrectPassword(t *testing.T) {
	hash, err := Hash("correct horse battery staple")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if hash == "" {
		t.Fatal("Hash returned empty string")
	}

	ok, err := verify("correct horse battery staple", hash)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !ok {
		t.Error("Verify rejected the correct password")
	}
}

func TestVerifyRejectsWrongPassword(t *testing.T) {
	hash, err := Hash("s3cret")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}

	ok, err := verify("not the password", hash)
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
	a, err := Hash("same")
	if err != nil {
		t.Fatalf("Hash a: %v", err)
	}
	b, err := Hash("same")
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
	if _, err := verify("whatever", "not-a-real-hash"); err == nil {
		t.Error("expected an error for a malformed hash, got nil")
	}
}

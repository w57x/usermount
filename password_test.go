package main

import (
	"testing"
)

func TestPasswordHashingAndVerification(t *testing.T) {
	password := "my-secure-password-123"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	if len(hash) == 0 {
		t.Fatalf("expected non-empty hash")
	}

	// Verify valid password
	ok, err := VerifyPassword(password, hash)
	if err != nil {
		t.Fatalf("error verifying password: %v", err)
	}
	if !ok {
		t.Fatalf("expected password verification to succeed")
	}

	// Verify invalid password
	ok, err = VerifyPassword("wrong-password", hash)
	if err != nil {
		t.Fatalf("error verifying wrong password: %v", err)
	}
	if ok {
		t.Fatalf("expected wrong password verification to fail")
	}

	// Verify malformed hash
	ok, err = VerifyPassword(password, "invalid$hash$string")
	if err == nil || ok {
		t.Fatalf("expected error when verifying malformed hash")
	}
}

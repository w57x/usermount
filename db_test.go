package main

import (
	"os"
	"testing"
)

func TestDbRollback(t *testing.T) {
	// Setup temporary database for testing
	tmpDb := "./test_usermount.db"
	defer os.Remove(tmpDb)

	AppConfig.DBPath = tmpDb
	if err := initDB(); err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer db.Close()

	// Test deleteUser
	username := "testuser"
	err := createUserDb(username, "hash", "user")
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	user, err := getUser(username)
	if err != nil || user == nil {
		t.Fatalf("failed to get user: %v", err)
	}

	err = deleteUser(username)
	if err != nil {
		t.Fatalf("failed to delete user: %v", err)
	}

	user, err = getUser(username)
	if err != nil {
		t.Fatalf("error getting user: %v", err)
	}
	if user != nil {
		t.Fatalf("user was not deleted")
	}

	// Test markInviteAsUnused
	code, err := createInvite("test@example.com")
	if err != nil {
		t.Fatalf("failed to create invite: %v", err)
	}

	invite, err := getInviteByCode(code)
	if err != nil || invite == nil {
		t.Fatalf("failed to get invite: %v", err)
	}
	if invite.Used {
		t.Fatalf("expected new invite to be unused")
	}

	consumed, err := markInviteAsUsed(code)
	if err != nil || !consumed {
		t.Fatalf("failed to mark invite as used: %v", err)
	}

	// Because getInviteByCode checks created_at >= datetime('now', '-10 minutes'),
	// and used invites are returned by getInviteByCode, let's verify
	invite, err = getInviteByCode(code)
	if err != nil || invite == nil {
		t.Fatalf("failed to get invite after markInviteAsUsed: %v", err)
	}
	if !invite.Used {
		t.Fatalf("expected invite to be marked as used")
	}

	err = markInviteAsUnused(code)
	if err != nil {
		t.Fatalf("failed to mark invite as unused: %v", err)
	}

	invite, err = getInviteByCode(code)
	if err != nil || invite == nil {
		t.Fatalf("failed to get invite after markInviteAsUnused: %v", err)
	}
	if invite.Used {
		t.Fatalf("expected invite to be unmarked (used=false)")
	}
}

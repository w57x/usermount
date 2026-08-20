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

func TestInitialAdminAndUsers(t *testing.T) {
	tmpDb := "./test_usermount_admin.db"
	defer os.Remove(tmpDb)

	AppConfig.DBPath = tmpDb
	if err := initDB(); err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer db.Close()

	hasAdm, err := hasAdmin()
	if err != nil || hasAdm {
		t.Fatalf("expected no admin initially")
	}

	created, err := createInitialAdmin("firstadmin", "hash123")
	if err != nil || !created {
		t.Fatalf("expected initial admin to be created")
	}

	// Second attempt should fail atomically
	created2, err := createInitialAdmin("secondadmin", "hash456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created2 {
		t.Fatalf("expected second initial admin creation to be rejected")
	}

	// Password update
	err = updateUserPassword("firstadmin", "newhash789")
	if err != nil {
		t.Fatalf("failed to update password: %v", err)
	}
	user, err := getUser("firstadmin")
	if err != nil || user == nil || user.PasswordHash != "newhash789" {
		t.Fatalf("expected password hash to be updated")
	}
}

func TestInviteManagement(t *testing.T) {
	tmpDb := "./test_usermount_invites.db"
	defer os.Remove(tmpDb)

	AppConfig.DBPath = tmpDb
	if err := initDB(); err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer db.Close()

	code1, err := createInvite("u1@example.com")
	if err != nil {
		t.Fatalf("failed to create invite 1: %v", err)
	}
	code2, err := createInvite("u2@example.com")
	if err != nil {
		t.Fatalf("failed to create invite 2: %v", err)
	}

	invites, err := listInvites()
	if err != nil || len(invites) != 2 {
		t.Fatalf("expected 2 invites, got %d", len(invites))
	}

	err = revokeInvite(code1)
	if err != nil {
		t.Fatalf("failed to revoke invite: %v", err)
	}

	invites, err = listInvites()
	if err != nil || len(invites) != 1 || invites[0].Code != code2 {
		t.Fatalf("expected 1 remaining invite (code2)")
	}
}

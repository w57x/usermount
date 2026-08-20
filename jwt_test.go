package main

import (
	"testing"
)

func TestJWTTokens(t *testing.T) {
	AppConfig.JWTSecret = "test-secret-key-12345"

	username := "testuser"
	role := "admin"

	accessToken, refreshToken, err := GenerateTokens(username, role)
	if err != nil {
		t.Fatalf("failed to generate tokens: %v", err)
	}

	if accessToken == "" || refreshToken == "" {
		t.Fatalf("expected non-empty tokens")
	}

	// Validate access token
	claims, err := ValidateToken(accessToken)
	if err != nil {
		t.Fatalf("failed to validate access token: %v", err)
	}
	if claims.Username != username || claims.Role != role {
		t.Fatalf("claims mismatch: got user=%s role=%s, want user=%s role=%s", claims.Username, claims.Role, username, role)
	}

	// Validate refresh token
	refreshClaims, err := ValidateToken(refreshToken)
	if err != nil {
		t.Fatalf("failed to validate refresh token: %v", err)
	}
	if refreshClaims.Username != username || refreshClaims.Role != role {
		t.Fatalf("refresh claims mismatch")
	}

	// Validate tampered / invalid token
	_, err = ValidateToken(accessToken + "tampered")
	if err == nil {
		t.Fatalf("expected error when validating tampered token")
	}
}

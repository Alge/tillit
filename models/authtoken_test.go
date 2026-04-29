package models

import (
	"testing"
	"time"
)

func mkToken(now time.Time, lifetime time.Duration) AuthToken {
	return AuthToken{
		Type:   AuthTokenType,
		Signer: "alice",
		Server: "https://example.com",
		IAT:    now.UTC().Format(time.RFC3339),
		EXP:    now.UTC().Add(lifetime).Format(time.RFC3339),
	}
}

func TestAuthToken_Validate_Accepts(t *testing.T) {
	now := time.Now().UTC()
	tok := mkToken(now, 4*time.Minute)
	if err := tok.Validate(now, "https://example.com"); err != nil {
		t.Errorf("expected valid token, got: %v", err)
	}
}

func TestAuthToken_Validate_RejectsExpired(t *testing.T) {
	now := time.Now().UTC()
	tok := mkToken(now.Add(-10*time.Minute), 5*time.Minute)
	if err := tok.Validate(now, "https://example.com"); err == nil {
		t.Error("expected expired token to be rejected")
	}
}

func TestAuthToken_Validate_RejectsFutureIAT(t *testing.T) {
	now := time.Now().UTC()
	tok := mkToken(now.Add(5*time.Minute), 5*time.Minute) // iat too far ahead
	if err := tok.Validate(now, "https://example.com"); err == nil {
		t.Error("expected far-future iat to be rejected")
	}
}

func TestAuthToken_Validate_RejectsExcessiveLifetime(t *testing.T) {
	now := time.Now().UTC()
	tok := mkToken(now, 30*time.Minute) // exp - iat > MaxAuthTokenLifetime
	if err := tok.Validate(now, "https://example.com"); err == nil {
		t.Error("expected over-window lifetime to be rejected")
	}
}

func TestAuthToken_Validate_RejectsServerMismatch(t *testing.T) {
	now := time.Now().UTC()
	tok := mkToken(now, 4*time.Minute)
	if err := tok.Validate(now, "https://other.example.com"); err == nil {
		t.Error("expected server mismatch to be rejected")
	}
}

func TestAuthToken_Validate_RejectsWrongType(t *testing.T) {
	now := time.Now().UTC()
	tok := mkToken(now, 4*time.Minute)
	tok.Type = "decision"
	if err := tok.Validate(now, "https://example.com"); err == nil {
		t.Error("expected wrong type to be rejected")
	}
}

func TestAuthToken_Validate_AllowsSmallClockSkew(t *testing.T) {
	now := time.Now().UTC()
	// iat 30s in the future — within tolerance.
	tok := mkToken(now.Add(30*time.Second), 2*time.Minute)
	if err := tok.Validate(now, "https://example.com"); err != nil {
		t.Errorf("30s clock skew should be tolerated, got: %v", err)
	}
}

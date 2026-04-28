package sqliteconnector

import (
	"testing"
	"time"

	"github.com/Alge/tillit/models"
)

func makeSignature(id, signer string) *models.Signature {
	return &models.Signature{
		ID:         id,
		Signer:     signer,
		Payload:    `{"type":"vetted","package":"example"}`,
		Algorithm:  "ed25519",
		Sig:        "base64sig==",
		UploadedAt: time.Now().UTC().Truncate(time.Second),
	}
}

func TestCreateAndGetSignature(t *testing.T) {
	c := newTestConnector(t)

	sig := makeSignature("sig-1", "user-a")

	if err := c.CreateSignature(sig); err != nil {
		t.Fatalf("CreateSignature failed: %v", err)
	}

	got, err := c.GetSignature("sig-1")
	if err != nil {
		t.Fatalf("GetSignature failed: %v", err)
	}

	if got.ID != sig.ID ||
		got.Signer != sig.Signer ||
		got.Payload != sig.Payload ||
		got.Algorithm != sig.Algorithm ||
		got.Sig != sig.Sig ||
		!got.UploadedAt.Equal(sig.UploadedAt) ||
		got.Revoked != false ||
		got.RevokedAt != nil {
		t.Errorf("got %+v, want %+v", got, sig)
	}
}

func TestGetSignatureNotFound(t *testing.T) {
	c := newTestConnector(t)
	_, err := c.GetSignature("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent signature")
	}
}

func TestGetUserSignatures(t *testing.T) {
	c := newTestConnector(t)

	for _, id := range []string{"sig-1", "sig-2"} {
		if err := c.CreateSignature(makeSignature(id, "user-a")); err != nil {
			t.Fatalf("CreateSignature failed: %v", err)
		}
	}
	// Different signer — should not appear
	if err := c.CreateSignature(makeSignature("sig-3", "user-b")); err != nil {
		t.Fatalf("CreateSignature failed: %v", err)
	}

	sigs, err := c.GetUserSignatures("user-a", nil)
	if err != nil {
		t.Fatalf("GetUserSignatures failed: %v", err)
	}
	if len(sigs) != 2 {
		t.Errorf("expected 2 signatures for user-a, got %d", len(sigs))
	}
}

func TestGetUserSignaturesSince(t *testing.T) {
	c := newTestConnector(t)

	old := makeSignature("sig-old", "user-a")
	old.UploadedAt = time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Second)
	if err := c.CreateSignature(old); err != nil {
		t.Fatalf("CreateSignature failed: %v", err)
	}

	recent := makeSignature("sig-new", "user-a")
	recent.UploadedAt = time.Now().UTC().Truncate(time.Second)
	if err := c.CreateSignature(recent); err != nil {
		t.Fatalf("CreateSignature failed: %v", err)
	}

	cutoff := time.Now().UTC().Add(-1 * time.Hour)
	sigs, err := c.GetUserSignatures("user-a", &cutoff)
	if err != nil {
		t.Fatalf("GetUserSignatures failed: %v", err)
	}
	if len(sigs) != 1 || sigs[0].ID != "sig-new" {
		t.Errorf("expected only sig-new, got %+v", sigs)
	}
}

func TestRevokeSignature(t *testing.T) {
	c := newTestConnector(t)

	sig := makeSignature("sig-1", "user-a")
	if err := c.CreateSignature(sig); err != nil {
		t.Fatalf("CreateSignature failed: %v", err)
	}

	revokedAt := time.Now().UTC().Truncate(time.Second)
	if err := c.RevokeSignature("sig-1", revokedAt); err != nil {
		t.Fatalf("RevokeSignature failed: %v", err)
	}

	got, err := c.GetSignature("sig-1")
	if err != nil {
		t.Fatalf("GetSignature failed: %v", err)
	}

	if !got.Revoked {
		t.Error("expected Revoked = true")
	}
	if got.RevokedAt == nil || !got.RevokedAt.Equal(revokedAt) {
		t.Errorf("RevokedAt = %v, want %v", got.RevokedAt, revokedAt)
	}
}

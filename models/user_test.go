package models_test

import (
	"crypto/sha256"
	"encoding/base64"
	"testing"

	"github.com/Alge/tillit/crypto"
	"github.com/Alge/tillit/models"
)

func TestNewUserFromSigner_ID(t *testing.T) {
	signer, _ := crypto.NewEd25519Signer()
	u, err := models.NewUserFromSigner("alice", signer)
	if err != nil {
		t.Fatalf("NewUserFromSigner failed: %v", err)
	}

	// ID must be SHA-256(pubkey) base64url
	hash := sha256.Sum256(signer.PublicKey())
	expected := base64.RawURLEncoding.EncodeToString(hash[:])
	if u.ID != expected {
		t.Errorf("ID = %q, want %q", u.ID, expected)
	}
}

func TestNewUserFromSigner_Algorithm(t *testing.T) {
	signer, _ := crypto.NewEd25519Signer()
	u, _ := models.NewUserFromSigner("alice", signer)
	if u.Algorithm != "ed25519" {
		t.Errorf("Algorithm = %q, want %q", u.Algorithm, "ed25519")
	}
}

func TestNewUserFromSigner_PubKey(t *testing.T) {
	signer, _ := crypto.NewEd25519Signer()
	u, _ := models.NewUserFromSigner("alice", signer)

	// PubKey must be base64url encoding of the raw public key bytes
	decoded, err := base64.RawURLEncoding.DecodeString(u.PubKey)
	if err != nil {
		t.Fatalf("PubKey is not valid base64url: %v", err)
	}
	if string(decoded) != string(signer.PublicKey()) {
		t.Error("PubKey does not match signer's public key")
	}
}

func TestNewUserFromSigner_SLH(t *testing.T) {
	signer, _ := crypto.NewSLHDSASigner()
	u, err := models.NewUserFromSigner("bob", signer)
	if err != nil {
		t.Fatalf("NewUserFromSigner with SLH-DSA failed: %v", err)
	}
	if u.Algorithm != "slh-dsa-shake-128s" {
		t.Errorf("Algorithm = %q, want %q", u.Algorithm, "slh-dsa-shake-128s")
	}
}

func TestUserVerifier(t *testing.T) {
	signer, _ := crypto.NewEd25519Signer()
	u, _ := models.NewUserFromSigner("alice", signer)

	verifier, err := u.Verifier()
	if err != nil {
		t.Fatalf("Verifier() failed: %v", err)
	}

	msg := []byte("test message")
	sig, _ := signer.Sign(msg)

	if !verifier.Verify(msg, sig) {
		t.Error("verifier failed to verify valid signature")
	}
}

func TestUserIDConsistency(t *testing.T) {
	signer, _ := crypto.NewEd25519Signer()
	u1, _ := models.NewUserFromSigner("alice", signer)
	u2, _ := models.NewUserFromSigner("alice-renamed", signer)

	// Same key → same ID regardless of username
	if u1.ID != u2.ID {
		t.Error("same key with different username should produce same ID")
	}
}

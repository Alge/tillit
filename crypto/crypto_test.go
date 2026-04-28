package crypto_test

import (
	"testing"

	"github.com/Alge/tillit/crypto"
)

func testSigner(t *testing.T, s crypto.Signer) {
	t.Helper()

	// Algorithm identifier must be non-empty
	if s.Algorithm() == "" {
		t.Error("Algorithm() must return a non-empty string")
	}

	// Public key must be exportable
	pub := s.PublicKey()
	if len(pub) == 0 {
		t.Error("PublicKey() returned empty bytes")
	}

	// Sign and verify round-trip
	msg := []byte("hello tillit")
	sig, err := s.Sign(msg)
	if err != nil {
		t.Fatalf("Sign failed: %v", err)
	}
	if len(sig) == 0 {
		t.Error("Sign returned empty signature")
	}

	if !s.Verify(msg, sig) {
		t.Error("Verify returned false for valid signature")
	}

	// Tampered message must not verify
	tampered := append([]byte(nil), msg...)
	tampered[0] ^= 0xff
	if s.Verify(tampered, sig) {
		t.Error("Verify returned true for tampered message")
	}

	// Tampered signature must not verify
	tamperedSig := append([]byte(nil), sig...)
	tamperedSig[0] ^= 0xff
	if s.Verify(msg, tamperedSig) {
		t.Error("Verify returned true for tampered signature")
	}
}

func TestEd25519Signer(t *testing.T) {
	s, err := crypto.NewEd25519Signer()
	if err != nil {
		t.Fatalf("NewEd25519Signer failed: %v", err)
	}
	testSigner(t, s)
}

func TestEd25519Algorithm(t *testing.T) {
	s, _ := crypto.NewEd25519Signer()
	if s.Algorithm() != "ed25519" {
		t.Errorf("expected algorithm ed25519, got %s", s.Algorithm())
	}
}

func TestSLHDSASigner(t *testing.T) {
	s, err := crypto.NewSLHDSASigner()
	if err != nil {
		t.Fatalf("NewSLHDSASigner failed: %v", err)
	}
	testSigner(t, s)
}

func TestSLHDSAAlgorithm(t *testing.T) {
	s, _ := crypto.NewSLHDSASigner()
	if s.Algorithm() != "slh-dsa-shake-128s" {
		t.Errorf("expected algorithm slh-dsa-shake-128s, got %s", s.Algorithm())
	}
}

func TestNewSignerFromAlgorithm(t *testing.T) {
	for _, alg := range []string{"ed25519", "slh-dsa-shake-128s"} {
		s, err := crypto.NewSigner(alg)
		if err != nil {
			t.Errorf("NewSigner(%q) failed: %v", alg, err)
			continue
		}
		if s.Algorithm() != alg {
			t.Errorf("expected algorithm %s, got %s", alg, s.Algorithm())
		}
	}
}

func TestNewSignerUnknownAlgorithm(t *testing.T) {
	_, err := crypto.NewSigner("rsa-2048")
	if err == nil {
		t.Error("expected error for unknown algorithm")
	}
}

func TestLoadSigner_Ed25519(t *testing.T) {
	orig, _ := crypto.NewEd25519Signer()
	privBytes := orig.PrivateKey()

	loaded, err := crypto.LoadSigner("ed25519", privBytes)
	if err != nil {
		t.Fatalf("LoadSigner failed: %v", err)
	}

	msg := []byte("round-trip test")
	sig, _ := orig.Sign(msg)
	if !loaded.Verify(msg, sig) {
		t.Error("loaded signer failed to verify signature from original")
	}
	// loaded signer should produce verifiable signatures too
	sig2, _ := loaded.Sign(msg)
	if !orig.Verify(msg, sig2) {
		t.Error("original signer failed to verify signature from loaded signer")
	}
}

func TestLoadSigner_SLHDSA(t *testing.T) {
	orig, _ := crypto.NewSLHDSASigner()
	loaded, err := crypto.LoadSigner("slh-dsa-shake-128s", orig.PrivateKey())
	if err != nil {
		t.Fatalf("LoadSigner failed: %v", err)
	}
	msg := []byte("round-trip test")
	sig, _ := orig.Sign(msg)
	if !loaded.Verify(msg, sig) {
		t.Error("loaded SLH-DSA signer failed to verify")
	}
}

func TestLoadSigner_Unknown(t *testing.T) {
	_, err := crypto.LoadSigner("rsa-2048", []byte("key"))
	if err == nil {
		t.Error("expected error for unknown algorithm")
	}
}

func TestLoadSignerFromPublicKey(t *testing.T) {
	s, _ := crypto.NewEd25519Signer()
	pub := s.PublicKey()

	verifier, err := crypto.NewVerifier("ed25519", pub)
	if err != nil {
		t.Fatalf("NewVerifier failed: %v", err)
	}

	msg := []byte("hello")
	sig, _ := s.Sign(msg)

	if !verifier.Verify(msg, sig) {
		t.Error("verifier from public key failed to verify valid signature")
	}
}

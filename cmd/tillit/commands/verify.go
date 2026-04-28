package commands

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"

	tillit_crypto "github.com/Alge/tillit/crypto"
	"github.com/Alge/tillit/localstore"
)

// verifySigned verifies that the given signed payload was created by the
// signer named in `signerID`, using the public key stored in cached_users.
// Returns nil iff the signature is valid AND the signer ID matches
// sha256(pubkey) (so a server can't substitute a different user's key).
func verifySigned(s *localstore.Store, signerID, payload, algorithm, sig string) error {
	u, err := s.GetCachedUser(signerID)
	if err != nil {
		return fmt.Errorf("no cached pubkey for signer %q — cannot verify locally", signerID)
	}
	if u.Algorithm != algorithm {
		return fmt.Errorf("algorithm mismatch: cached %q, signature %q", u.Algorithm, algorithm)
	}
	pubBytes, err := base64.RawURLEncoding.DecodeString(u.PubKey)
	if err != nil {
		return fmt.Errorf("cached pubkey is not valid base64url: %w", err)
	}

	// Defense-in-depth: the user ID is meant to be sha256(pubkey).
	expected := base64.RawURLEncoding.EncodeToString(sha256Hash(pubBytes))
	if expected != signerID {
		return fmt.Errorf("cached pubkey does not match signer ID (got %s, want %s)", expected, signerID)
	}

	verifier, err := tillit_crypto.NewVerifier(algorithm, pubBytes)
	if err != nil {
		return fmt.Errorf("failed building verifier: %w", err)
	}
	sigBytes, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil {
		return fmt.Errorf("signature is not valid base64url: %w", err)
	}
	if !verifier.Verify([]byte(payload), sigBytes) {
		return fmt.Errorf("signature does not verify against cached pubkey")
	}
	return nil
}

func sha256Hash(b []byte) []byte {
	h := sha256.Sum256(b)
	return h[:]
}

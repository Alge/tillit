package crypto

import "fmt"

// Signer can sign messages and verify signatures, and exposes its public key.
type Signer interface {
	Verifier
	Sign(message []byte) ([]byte, error)
	PublicKey() []byte
}

// Verifier can verify signatures given a public key.
type Verifier interface {
	Algorithm() string
	Verify(message, sig []byte) bool
}

// NewSigner returns a new Signer for the given algorithm with a freshly generated key.
func NewSigner(algorithm string) (Signer, error) {
	switch algorithm {
	case "ed25519":
		return NewEd25519Signer()
	case "slh-dsa-shake-128s":
		return NewSLHDSASigner()
	default:
		return nil, fmt.Errorf("unknown algorithm: %q", algorithm)
	}
}

// NewVerifier returns a Verifier for the given algorithm and public key bytes.
func NewVerifier(algorithm string, publicKey []byte) (Verifier, error) {
	switch algorithm {
	case "ed25519":
		return newEd25519Verifier(publicKey)
	case "slh-dsa-shake-128s":
		return newSLHDSAVerifier(publicKey)
	default:
		return nil, fmt.Errorf("unknown algorithm: %q", algorithm)
	}
}

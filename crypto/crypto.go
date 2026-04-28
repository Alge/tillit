package crypto

import "fmt"

// Signer can sign messages and verify signatures, and exposes its public and private keys.
type Signer interface {
	Verifier
	Sign(message []byte) ([]byte, error)
	PublicKey() []byte
	PrivateKey() []byte
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

// LoadSigner reconstructs a Signer from a stored private key for the given algorithm.
func LoadSigner(algorithm string, privateKey []byte) (Signer, error) {
	switch algorithm {
	case "ed25519":
		return loadEd25519Signer(privateKey)
	case "slh-dsa-shake-128s":
		return loadSLHDSASigner(privateKey)
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

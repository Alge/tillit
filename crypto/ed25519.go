package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
)

func loadEd25519Signer(privateKey []byte) (Signer, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid ed25519 private key length: got %d, want %d", len(privateKey), ed25519.PrivateKeySize)
	}
	priv := ed25519.PrivateKey(privateKey)
	return &ed25519Signer{priv: priv, pub: priv.Public().(ed25519.PublicKey)}, nil
}

type ed25519Signer struct {
	priv ed25519.PrivateKey
	pub  ed25519.PublicKey
}

func NewEd25519Signer() (Signer, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed generating ed25519 key: %w", err)
	}
	return &ed25519Signer{priv: priv, pub: pub}, nil
}

func (s *ed25519Signer) Algorithm() string {
	return "ed25519"
}

func (s *ed25519Signer) PublicKey() []byte  { return []byte(s.pub) }
func (s *ed25519Signer) PrivateKey() []byte { return []byte(s.priv) }

func (s *ed25519Signer) Sign(message []byte) ([]byte, error) {
	return ed25519.Sign(s.priv, message), nil
}

func (s *ed25519Signer) Verify(message, sig []byte) bool {
	return ed25519.Verify(s.pub, message, sig)
}

type ed25519Verifier struct {
	pub ed25519.PublicKey
}

func newEd25519Verifier(publicKey []byte) (Verifier, error) {
	if len(publicKey) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid ed25519 public key length: got %d, want %d", len(publicKey), ed25519.PublicKeySize)
	}
	return &ed25519Verifier{pub: ed25519.PublicKey(publicKey)}, nil
}

func (v *ed25519Verifier) Algorithm() string {
	return "ed25519"
}

func (v *ed25519Verifier) Verify(message, sig []byte) bool {
	return ed25519.Verify(v.pub, message, sig)
}

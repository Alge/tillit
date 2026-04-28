package crypto

import (
	"crypto/rand"
	"fmt"

	"github.com/cloudflare/circl/sign/slhdsa"
)

type slhDSASigner struct {
	priv slhdsa.PrivateKey
	pub  slhdsa.PublicKey
}

func NewSLHDSASigner() (Signer, error) {
	pub, priv, err := slhdsa.GenerateKey(rand.Reader, slhdsa.SHAKE_128s)
	if err != nil {
		return nil, fmt.Errorf("failed generating SLH-DSA key: %w", err)
	}
	return &slhDSASigner{priv: priv, pub: pub}, nil
}

func (s *slhDSASigner) Algorithm() string {
	return "slh-dsa-shake-128s"
}

func (s *slhDSASigner) PublicKey() []byte {
	b, _ := s.pub.MarshalBinary()
	return b
}

func (s *slhDSASigner) PrivateKey() []byte {
	b, _ := s.priv.MarshalBinary()
	return b
}

func loadSLHDSASigner(privateKey []byte) (Signer, error) {
	var priv slhdsa.PrivateKey
	priv.ID = slhdsa.SHAKE_128s
	if err := priv.UnmarshalBinary(privateKey); err != nil {
		return nil, fmt.Errorf("invalid SLH-DSA private key: %w", err)
	}
	pub := priv.Public().(slhdsa.PublicKey)
	return &slhDSASigner{priv: priv, pub: pub}, nil
}

func (s *slhDSASigner) Sign(message []byte) ([]byte, error) {
	return slhdsa.SignRandomized(&s.priv, rand.Reader, slhdsa.NewMessage(message), nil)
}

func (s *slhDSASigner) Verify(message, sig []byte) bool {
	return slhdsa.Verify(&s.pub, slhdsa.NewMessage(message), sig, nil)
}

type slhDSAVerifier struct {
	pub slhdsa.PublicKey
}

func newSLHDSAVerifier(publicKey []byte) (Verifier, error) {
	var pub slhdsa.PublicKey
	pub.ID = slhdsa.SHAKE_128s
	if err := pub.UnmarshalBinary(publicKey); err != nil {
		return nil, fmt.Errorf("invalid SLH-DSA public key: %w", err)
	}
	return &slhDSAVerifier{pub: pub}, nil
}

func (v *slhDSAVerifier) Algorithm() string {
	return "slh-dsa-shake-128s"
}

func (v *slhDSAVerifier) Verify(message, sig []byte) bool {
	return slhdsa.Verify(&v.pub, slhdsa.NewMessage(message), sig, nil)
}

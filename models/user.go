package models

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"

	"github.com/Alge/tillit/crypto"
)

type User struct {
	ID          string       `json:"id"`
	Username    string       `json:"username"`
	PubKey      string       `json:"public_key"`
	Algorithm   string       `json:"algorithm"`
	Connections []Connection `json:"connections,omitempty"`
	IsAdmin     bool         `json:"is_admin,omitempty"`
}

// NewUserFromSigner creates a User whose ID is SHA-256(pubkey) base64url encoded.
func NewUserFromSigner(username string, signer crypto.Signer) (*User, error) {
	pubBytes := signer.PublicKey()
	hash := sha256.Sum256(pubBytes)
	return &User{
		ID:        base64.RawURLEncoding.EncodeToString(hash[:]),
		Username:  username,
		PubKey:    base64.RawURLEncoding.EncodeToString(pubBytes),
		Algorithm: signer.Algorithm(),
	}, nil
}

// Verifier returns a crypto.Verifier that can verify signatures made by this user.
func (u *User) Verifier() (crypto.Verifier, error) {
	pubBytes, err := base64.RawURLEncoding.DecodeString(u.PubKey)
	if err != nil {
		return nil, fmt.Errorf("failed decoding public key: %w", err)
	}
	return crypto.NewVerifier(u.Algorithm, pubBytes)
}

func (u *User) Connect(other *User, public bool, trust bool, delegate bool, trustExtends int) *Connection {
	return &Connection{
		Owner:        u.ID,
		OtherID:      other.ID,
		Public:       public,
		Trust:        trust,
		Delegate:     delegate,
		TrustExtends: trustExtends,
	}
}

func (u User) String() string {
	return fmt.Sprintf("User{ID: %s, Username: %s, Algorithm: %s}", u.ID, u.Username, u.Algorithm)
}

package commands

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"time"

	tillit_crypto "github.com/Alge/tillit/crypto"
	"github.com/Alge/tillit/localstore"
)

const storeName = ".tillit.db"

func openStore() (*localstore.Store, error) {
	dir, err := storeDir()
	if err != nil {
		return nil, err
	}
	return localstore.Init(filepath.Join(dir, storeName))
}

// activeSignerAndID loads the active key, prompting for a password
// if it's stored encrypted, and returns a Signer plus the user ID.
// Use this from commands that actually need to produce signatures.
// Commands that only need the user identity (status, query, check)
// should call activeUserID instead, which doesn't require the
// password.
func activeSignerAndID(s *localstore.Store) (tillit_crypto.Signer, string, error) {
	keyName, err := s.GetActiveKey()
	if err != nil {
		return nil, "", fmt.Errorf("no active key — run 'tillit init' first")
	}
	k, err := s.GetKey(keyName)
	if err != nil {
		return nil, "", err
	}
	privBytes, err := decodePrivateKey(k)
	if err != nil {
		return nil, "", err
	}
	signer, err := tillit_crypto.LoadSigner(k.Algorithm, privBytes)
	if err != nil {
		return nil, "", fmt.Errorf("failed loading signer: %w", err)
	}
	hash := sha256.Sum256(signer.PublicKey())
	userID := base64.RawURLEncoding.EncodeToString(hash[:])
	return signer, userID, nil
}

// activeUserID returns the active user's ID without ever touching
// the private key. The ID is sha256(pubkey) and the pubkey is stored
// in plaintext, so password-protected keys can be queried for
// identity without prompting.
func activeUserID(s *localstore.Store) (string, error) {
	keyName, err := s.GetActiveKey()
	if err != nil {
		return "", fmt.Errorf("no active key — run 'tillit init' first")
	}
	k, err := s.GetKey(keyName)
	if err != nil {
		return "", err
	}
	pubBytes, err := base64.RawURLEncoding.DecodeString(k.PubKey)
	if err != nil {
		return "", fmt.Errorf("invalid stored pubkey: %w", err)
	}
	hash := sha256.Sum256(pubBytes)
	return base64.RawURLEncoding.EncodeToString(hash[:]), nil
}

// decodePrivateKey returns the raw private key bytes from a stored
// Key, prompting for a password if the key is encrypted at rest.
// The leading byte of k.PrivKey distinguishes the JSON envelope
// (encrypted) from a base64url-encoded plaintext blob (legacy).
func decodePrivateKey(k *localstore.Key) ([]byte, error) {
	if tillit_crypto.IsEncryptedKey([]byte(k.PrivKey)) {
		password, err := promptPassword(fmt.Sprintf("Password for key %q: ", k.Name))
		if err != nil {
			return nil, fmt.Errorf("read password: %w", err)
		}
		plain, err := tillit_crypto.DecryptKey([]byte(k.PrivKey), password)
		if err != nil {
			return nil, err
		}
		return plain, nil
	}
	priv, err := base64.RawURLEncoding.DecodeString(k.PrivKey)
	if err != nil {
		return nil, fmt.Errorf("invalid stored private key: %w", err)
	}
	return priv, nil
}

// fetchAndCachePubkey GETs the user from the server, verifies that
// sha256(pubkey) matches the expected ID, and caches the public key so
// subsequent signatures from this user can be verified offline.
func fetchAndCachePubkey(s *localstore.Store, serverURL, userID string) error {
	u, err := fetchUser(serverURL, userID)
	if err != nil {
		return err
	}
	pubBytes, err := base64.RawURLEncoding.DecodeString(u.PubKey)
	if err != nil {
		return fmt.Errorf("server returned invalid pubkey encoding: %w", err)
	}
	hash := sha256.Sum256(pubBytes)
	expected := base64.RawURLEncoding.EncodeToString(hash[:])
	if expected != userID {
		return fmt.Errorf("server returned pubkey whose hash %s does not match requested ID %s", expected, userID)
	}
	return s.SaveCachedUser(&localstore.CachedUser{
		ID:        u.ID,
		Username:  u.Username,
		PubKey:    u.PubKey,
		Algorithm: u.Algorithm,
		FetchedAt: time.Now().UTC(),
	})
}

func storeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	dir := filepath.Join(home, ".tillit")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("cannot create %s: %w", dir, err)
	}
	return dir, nil
}

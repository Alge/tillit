package commands

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"

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

// activeSignerAndID loads the active key from the store and returns the signer and user ID.
func activeSignerAndID(s *localstore.Store) (tillit_crypto.Signer, string, error) {
	keyName, err := s.GetActiveKey()
	if err != nil {
		return nil, "", fmt.Errorf("no active key — run 'tillit init' first")
	}
	k, err := s.GetKey(keyName)
	if err != nil {
		return nil, "", err
	}
	privBytes, err := base64.RawURLEncoding.DecodeString(k.PrivKey)
	if err != nil {
		return nil, "", fmt.Errorf("invalid stored private key: %w", err)
	}
	signer, err := tillit_crypto.LoadSigner(k.Algorithm, privBytes)
	if err != nil {
		return nil, "", fmt.Errorf("failed loading signer: %w", err)
	}
	hash := sha256.Sum256(signer.PublicKey())
	userID := base64.RawURLEncoding.EncodeToString(hash[:])
	return signer, userID, nil
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

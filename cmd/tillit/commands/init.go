package commands

import (
	"fmt"

	"github.com/Alge/tillit/crypto"
)

func Init(args []string) error {
	s, err := openStore()
	if err != nil {
		return err
	}
	defer s.Close()

	// Check if a default key already exists
	if _, err := s.GetKey("default"); err == nil {
		fmt.Println("tillit already initialized (key 'default' exists)")
		return nil
	}

	signer, err := crypto.NewEd25519Signer()
	if err != nil {
		return fmt.Errorf("failed generating key: %w", err)
	}

	key, err := buildStoredKey("default", signer)
	if err != nil {
		return err
	}
	if err := s.SaveKey(key); err != nil {
		return fmt.Errorf("failed saving key: %w", err)
	}
	if err := s.SetActiveKey("default"); err != nil {
		return fmt.Errorf("failed setting active key: %w", err)
	}

	fmt.Printf("Initialized tillit\n")
	fmt.Printf("Active key: default (%s)\n", key.Algorithm)
	fmt.Printf("Public key: %s\n", key.PubKey)
	return nil
}

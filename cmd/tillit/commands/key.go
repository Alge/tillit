package commands

import (
	"encoding/base64"
	"fmt"
	"os"

	"github.com/Alge/tillit/crypto"
	"github.com/Alge/tillit/localstore"
)

func Key(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: tillit key <generate|list|show|use> [args]")
	}
	switch args[0] {
	case "generate":
		return keyGenerate(args[1:])
	case "list":
		return keyList()
	case "show":
		return keyShow(args[1:])
	case "use":
		return keyUse(args[1:])
	default:
		return fmt.Errorf("unknown key subcommand: %s", args[0])
	}
}

func keyGenerate(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: tillit key generate <name> [ed25519|slh-dsa-shake-128s]")
	}
	name := args[0]
	algorithm := "ed25519"
	if len(args) > 1 {
		algorithm = args[1]
	}

	signer, err := crypto.NewSigner(algorithm)
	if err != nil {
		return fmt.Errorf("failed generating key: %w", err)
	}

	s, err := openStore()
	if err != nil {
		return err
	}
	defer s.Close()

	key := &localstore.Key{
		Name:      name,
		Algorithm: signer.Algorithm(),
		PubKey:    base64.RawURLEncoding.EncodeToString(signer.PublicKey()),
		PrivKey:   base64.RawURLEncoding.EncodeToString(signer.PrivateKey()),
	}
	if err := s.SaveKey(key); err != nil {
		return fmt.Errorf("failed saving key: %w", err)
	}

	fmt.Printf("Generated key '%s' (%s)\n", name, algorithm)
	fmt.Printf("Public key: %s\n", key.PubKey)
	return nil
}

func keyList() error {
	s, err := openStore()
	if err != nil {
		return err
	}
	defer s.Close()

	keys, err := s.ListKeys()
	if err != nil {
		return fmt.Errorf("failed listing keys: %w", err)
	}

	active, _ := s.GetActiveKey()

	if len(keys) == 0 {
		fmt.Println("No keys found. Run 'tillit init' or 'tillit key generate <name>'.")
		return nil
	}

	for _, k := range keys {
		marker := "  "
		if k.Name == active {
			marker = "* "
		}
		fmt.Printf("%s%s (%s)\n", marker, k.Name, k.Algorithm)
	}
	return nil
}

func keyShow(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: tillit key show <name>")
	}
	s, err := openStore()
	if err != nil {
		return err
	}
	defer s.Close()

	k, err := s.GetKey(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "key %q not found\n", args[0])
		return err
	}
	fmt.Printf("Name:      %s\n", k.Name)
	fmt.Printf("Algorithm: %s\n", k.Algorithm)
	fmt.Printf("Public key: %s\n", k.PubKey)
	return nil
}

func keyUse(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: tillit key use <name>")
	}
	s, err := openStore()
	if err != nil {
		return err
	}
	defer s.Close()

	if _, err := s.GetKey(args[0]); err != nil {
		return fmt.Errorf("key %q not found", args[0])
	}
	if err := s.SetActiveKey(args[0]); err != nil {
		return fmt.Errorf("failed setting active key: %w", err)
	}
	fmt.Printf("Active key set to '%s'\n", args[0])
	return nil
}

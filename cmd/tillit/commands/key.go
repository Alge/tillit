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
		return fmt.Errorf("usage: tillit key <generate|list|show|use|passwd> [args]")
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
	case "passwd":
		return keyPasswd(args[1:])
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

	key, err := buildStoredKey(name, signer)
	if err != nil {
		return err
	}
	if err := s.SaveKey(key); err != nil {
		return fmt.Errorf("failed saving key: %w", err)
	}

	fmt.Printf("Generated key '%s' (%s)\n", name, algorithm)
	fmt.Printf("Public key: %s\n", key.PubKey)
	return nil
}

// buildStoredKey turns a freshly-generated signer into a Key suitable
// for SaveKey, prompting the user for an optional password. An empty
// password leaves the private key in plaintext (with a printed
// warning); a non-empty password produces an encrypted JSON envelope.
func buildStoredKey(name string, signer crypto.Signer) (*localstore.Key, error) {
	pwd, err := promptPasswordTwice(
		fmt.Sprintf("Password for new key %q (empty to skip encryption): ", name),
		"Confirm password: ",
	)
	if err != nil {
		return nil, err
	}
	privField, err := encodePrivKeyField(signer.PrivateKey(), pwd)
	if err != nil {
		return nil, err
	}
	return &localstore.Key{
		Name:      name,
		Algorithm: signer.Algorithm(),
		PubKey:    base64.RawURLEncoding.EncodeToString(signer.PublicKey()),
		PrivKey:   privField,
	}, nil
}

// encodePrivKeyField returns either a base64url-encoded plaintext
// private key (when password is empty) or an EncryptKey envelope.
// The plaintext path prints a warning so the user knows their key is
// stored unprotected.
func encodePrivKeyField(privBytes, password []byte) (string, error) {
	if len(password) == 0 {
		fmt.Fprintln(os.Stderr, "warning: storing private key unencrypted on disk")
		return base64.RawURLEncoding.EncodeToString(privBytes), nil
	}
	envelope, err := crypto.EncryptKey(privBytes, password)
	if err != nil {
		return "", fmt.Errorf("encrypt key: %w", err)
	}
	return string(envelope), nil
}

// keyPasswd changes (or removes) the password protecting a stored
// key. The user is prompted for the current password if the key is
// encrypted, then for the new one (twice). An empty new password
// stores the private key in plaintext.
func keyPasswd(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: tillit key passwd <name>")
	}
	name := args[0]

	s, err := openStore()
	if err != nil {
		return err
	}
	defer s.Close()

	k, err := s.GetKey(name)
	if err != nil {
		return err
	}
	plain, err := decodePrivateKey(k)
	if err != nil {
		return err
	}
	newPwd, err := promptPasswordTwice(
		"New password (empty to remove encryption): ",
		"Confirm new password: ",
	)
	if err != nil {
		return err
	}
	encoded, err := encodePrivKeyField(plain, newPwd)
	if err != nil {
		return err
	}
	k.PrivKey = encoded
	if err := s.SaveKey(k); err != nil {
		return fmt.Errorf("save: %w", err)
	}
	if len(newPwd) == 0 {
		fmt.Printf("Removed encryption from key %q\n", name)
	} else {
		fmt.Printf("Updated password on key %q\n", name)
	}
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

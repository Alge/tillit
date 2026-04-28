package commands

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/Alge/tillit/localstore"
	"github.com/Alge/tillit/models"
)

func Register(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: tillit register <server_url> [alias]")
	}
	serverURL := strings.TrimRight(args[0], "/")
	alias := ""
	if len(args) > 1 {
		alias = args[1]
	}

	s, err := openStore()
	if err != nil {
		return err
	}
	defer s.Close()

	keyName, err := s.GetActiveKey()
	if err != nil {
		return fmt.Errorf("no active key — run 'tillit init' first")
	}
	k, err := s.GetKey(keyName)
	if err != nil {
		return err
	}

	pubBytes, err := base64.RawURLEncoding.DecodeString(k.PubKey)
	if err != nil {
		return fmt.Errorf("invalid stored pubkey: %w", err)
	}
	hash := sha256.Sum256(pubBytes)
	userID := base64.RawURLEncoding.EncodeToString(hash[:])

	u := &models.User{
		ID:        userID,
		Username:  keyName,
		PubKey:    k.PubKey,
		Algorithm: k.Algorithm,
	}

	body, err := json.Marshal(u)
	if err != nil {
		return err
	}

	resp, err := http.Post(serverURL+"/v1/users", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed contacting server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned %s", resp.Status)
	}

	var created models.User
	json.NewDecoder(resp.Body).Decode(&created)

	if err := s.SaveServer(&localstore.Server{
		URL:    serverURL,
		Alias:  alias,
		UserID: created.ID,
	}); err != nil {
		return fmt.Errorf("failed saving server: %w", err)
	}

	fmt.Printf("Registered on %s\n", serverURL)
	fmt.Printf("User ID: %s\n", created.ID)
	return nil
}

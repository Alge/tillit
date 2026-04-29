package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	tillit_crypto "github.com/Alge/tillit/crypto"
	"github.com/Alge/tillit/localstore"
	"github.com/Alge/tillit/models"
)

// Mirror dispatches the cross-device sync subcommands. Unlike publish
// (which pushes signatures publicly so anyone can sync them), mirror
// pushes them to your own server with the private flag set — only
// you can pull them back, on another device.
//
//	tillit mirror push <server>   upload your local sigs/conns privately
//	tillit mirror pull <server>   fetch your private rows back into local cache
func Mirror(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: tillit mirror <push|pull> <server>")
	}
	sub := args[0]
	rest := args[1:]
	switch sub {
	case "push":
		return mirrorPush(rest)
	case "pull":
		return mirrorPull(rest)
	default:
		return fmt.Errorf("unknown mirror subcommand %q (expected push or pull)", sub)
	}
}

func mirrorPush(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: tillit mirror push <server>")
	}
	serverURL := strings.TrimRight(args[0], "/")

	s, err := openStore()
	if err != nil {
		return err
	}
	defer s.Close()
	signer, userID, err := activeSignerAndID(s)
	if err != nil {
		return err
	}

	sigs, err := s.GetCachedSignaturesBySigner(userID)
	if err != nil {
		return fmt.Errorf("read local signatures: %w", err)
	}
	conns, err := s.GetCachedConnectionsBySigner(userID)
	if err != nil {
		return fmt.Errorf("read local connections: %w", err)
	}

	pushedSigs, pushedConns := 0, 0
	for _, sig := range sigs {
		if err := postPrivateSignature(serverURL, userID, signer, sig); err != nil {
			return fmt.Errorf("push signature %s: %w", sig.ID, err)
		}
		pushedSigs++
	}
	for _, conn := range conns {
		if err := postPrivateConnection(serverURL, userID, signer, conn); err != nil {
			return fmt.Errorf("push connection %s: %w", conn.ID, err)
		}
		pushedConns++
	}
	fmt.Printf("Mirrored %d signature(s) and %d connection(s) privately to %s\n",
		pushedSigs, pushedConns, serverURL)
	fmt.Println("(server keeps these visible only to you; pull them on another device with 'tillit mirror pull')")
	return nil
}

func mirrorPull(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: tillit mirror pull <server>")
	}
	serverURL := strings.TrimRight(args[0], "/")

	s, err := openStore()
	if err != nil {
		return err
	}
	defer s.Close()
	signer, userID, err := activeSignerAndID(s)
	if err != nil {
		return err
	}

	sigs, err := fetchUserSignaturesAuth(serverURL, userID, signer)
	if err != nil {
		return fmt.Errorf("fetch signatures: %w", err)
	}
	conns, err := fetchUserConnectionsAuth(serverURL, userID, signer)
	if err != nil {
		return fmt.Errorf("fetch connections: %w", err)
	}

	now := time.Now().UTC()
	pulled := 0
	for _, sig := range sigs {
		if err := s.SaveCachedSignature(&localstore.CachedSignature{
			ID:         sig.ID,
			Signer:     sig.Signer,
			Payload:    sig.Payload,
			Algorithm:  sig.Algorithm,
			Sig:        sig.Sig,
			UploadedAt: sig.UploadedAt,
			Revoked:    sig.Revoked,
			RevokedAt:  sig.RevokedAt,
			FetchedAt:  now,
		}); err != nil {
			return fmt.Errorf("save signature %s: %w", sig.ID, err)
		}
		pulled++
	}
	pulledConns := 0
	for _, conn := range conns {
		if err := s.SaveCachedConnection(&localstore.CachedConnection{
			ID:        conn.ID,
			Signer:    conn.Owner,
			OtherID:   conn.OtherID,
			Payload:   conn.Payload,
			Algorithm: conn.Algorithm,
			Sig:       conn.Sig,
			CreatedAt: conn.CreatedAt,
			Revoked:   conn.Revoked,
			RevokedAt: conn.RevokedAt,
			FetchedAt: now,
		}); err != nil {
			return fmt.Errorf("save connection %s: %w", conn.ID, err)
		}
		pulledConns++
	}
	fmt.Printf("Pulled %d signature(s) and %d connection(s) from your private store at %s\n",
		pulled, pulledConns, serverURL)
	return nil
}

func postPrivateSignature(serverURL, userID string, signer tillit_crypto.Signer, sig *localstore.CachedSignature) error {
	private := false
	body, err := json.Marshal(struct {
		ID        string `json:"id"`
		Payload   string `json:"payload"`
		Algorithm string `json:"algorithm"`
		Sig       string `json:"sig"`
		Public    *bool  `json:"public"`
	}{
		ID:        sig.ID,
		Payload:   sig.Payload,
		Algorithm: sig.Algorithm,
		Sig:       sig.Sig,
		Public:    &private,
	})
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/v1/users/%s/signatures", serverURL, userID)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if err := withAuth(req, signer, userID); err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("server returned %s", resp.Status)
	}
	return nil
}

func postPrivateConnection(serverURL, userID string, signer tillit_crypto.Signer, conn *localstore.CachedConnection) error {
	private := false
	body, err := json.Marshal(struct {
		ID        string `json:"id"`
		Payload   string `json:"payload"`
		Algorithm string `json:"algorithm"`
		Sig       string `json:"sig"`
		Public    *bool  `json:"public"`
	}{
		ID:        conn.ID,
		Payload:   conn.Payload,
		Algorithm: conn.Algorithm,
		Sig:       conn.Sig,
		Public:    &private,
	})
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/v1/users/%s/connections", serverURL, userID)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if err := withAuth(req, signer, userID); err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	// Connection endpoint returns 201 on create or 204 on revocation.
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("server returned %s", resp.Status)
	}
	return nil
}

func fetchUserSignaturesAuth(serverURL, userID string, signer tillit_crypto.Signer) ([]*models.Signature, error) {
	url := fmt.Sprintf("%s/v1/users/%s/signatures", serverURL, userID)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if err := withAuth(req, signer, userID); err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned %s", resp.Status)
	}
	var sigs []*models.Signature
	if err := json.NewDecoder(resp.Body).Decode(&sigs); err != nil {
		return nil, err
	}
	return sigs, nil
}

func fetchUserConnectionsAuth(serverURL, userID string, signer tillit_crypto.Signer) ([]*models.Connection, error) {
	url := fmt.Sprintf("%s/v1/users/%s/connections", serverURL, userID)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if err := withAuth(req, signer, userID); err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned %s", resp.Status)
	}
	var conns []*models.Connection
	if err := json.NewDecoder(resp.Body).Decode(&conns); err != nil {
		return nil, err
	}
	return conns, nil
}

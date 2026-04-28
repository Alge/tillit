package commands

import (
	"fmt"
	"time"

	"github.com/Alge/tillit/localstore"
)

func Sync(args []string) error {
	s, err := openStore()
	if err != nil {
		return err
	}
	defer s.Close()

	peers, err := s.ListPeers()
	if err != nil {
		return fmt.Errorf("failed listing peers: %w", err)
	}
	if len(peers) == 0 {
		fmt.Println("No peers configured. Use 'tillit trust <id@url>' to add peers.")
		return nil
	}

	now := time.Now().UTC()
	total := 0

	for _, peer := range peers {
		if peer.Distrusted {
			continue
		}
		sigs, err := fetchUserSignatures(peer.ServerURL, peer.ID, nil)
		if err != nil {
			fmt.Printf("  [%s] fetch failed: %v\n", peer.ID, err)
			continue
		}
		for _, sig := range sigs {
			cached := &localstore.CachedSignature{
				ID:         sig.ID,
				Signer:     sig.Signer,
				Payload:    sig.Payload,
				Algorithm:  sig.Algorithm,
				Sig:        sig.Sig,
				UploadedAt: sig.UploadedAt,
				Revoked:    sig.Revoked,
				FetchedAt:  now,
			}
			if sig.RevokedAt != nil {
				cached.RevokedAt = sig.RevokedAt
			}
			if err := s.SaveCachedSignature(cached); err != nil {
				fmt.Printf("  warning: failed caching sig %s: %v\n", sig.ID, err)
			}
		}
		fmt.Printf("Synced %d signatures from %s\n", len(sigs), peer.ID)
		total += len(sigs)
	}

	fmt.Printf("Total: %d signatures cached\n", total)
	return nil
}

func Publish(args []string) error {
	s, err := openStore()
	if err != nil {
		return err
	}
	defer s.Close()

	_, userID, err := activeSignerAndID(s)
	if err != nil {
		return err
	}

	servers, err := s.ListServers()
	if err != nil {
		return fmt.Errorf("failed listing servers: %w", err)
	}
	if len(servers) == 0 {
		return fmt.Errorf("no servers registered — run 'tillit register <server_url>' first")
	}

	cachedSigs, err := s.GetCachedSignaturesBySigner(userID)
	if err != nil {
		return fmt.Errorf("failed reading local cache: %w", err)
	}
	if len(cachedSigs) == 0 {
		fmt.Println("Nothing to publish.")
		return nil
	}

	for _, srv := range servers {
		existing, err := fetchUserSignatures(srv.URL, userID, nil)
		if err != nil {
			fmt.Printf("  [%s] failed fetching existing signatures: %v\n", srv.URL, err)
			continue
		}
		existingIDs := make(map[string]bool, len(existing))
		for _, e := range existing {
			existingIDs[e.ID] = true
		}

		pushed := 0
		for _, cached := range cachedSigs {
			if existingIDs[cached.ID] {
				continue
			}
			req := sigUploadRequest{
				Payload:   cached.Payload,
				Algorithm: cached.Algorithm,
				Sig:       cached.Sig,
			}
			if _, err := uploadSignature(srv.URL, userID, req); err != nil {
				fmt.Printf("  [%s] push failed for %s: %v\n", srv.URL, cached.ID, err)
				continue
			}
			pushed++
		}
		fmt.Printf("Pushed %d new signatures to %s\n", pushed, srv.URL)
	}
	return nil
}

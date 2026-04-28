package commands

import (
	"fmt"
	"time"

	"github.com/Alge/tillit/localstore"
	"github.com/Alge/tillit/models"
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
	totalSigs, totalConns := 0, 0

	for _, peer := range peers {
		if peer.Distrusted {
			continue
		}

		// Make sure we have this peer's pubkey before trusting any data from
		// their server.
		if _, err := s.GetCachedUser(peer.ID); err != nil {
			if err := fetchAndCachePubkey(s, peer.ServerURL, peer.ID); err != nil {
				fmt.Printf("  [%s] failed fetching pubkey: %v\n", peer.ID, err)
				continue
			}
		}

		conns, err := fetchUserConnections(peer.ServerURL, peer.ID, peer.LastSyncedAt)
		if err != nil {
			fmt.Printf("  [%s] connection fetch failed: %v\n", peer.ID, err)
		}
		connsCached := 0
		for _, c := range conns {
			if err := verifySigned(s, peer.ID, c.Payload, c.Algorithm, c.Sig); err != nil {
				fmt.Printf("  [%s] dropped connection %s: %v\n", peer.ID, c.ID, err)
				continue
			}
			cached := &localstore.CachedConnection{
				ID:        c.ID,
				Signer:    peer.ID,
				OtherID:   c.OtherID,
				Payload:   c.Payload,
				Algorithm: c.Algorithm,
				Sig:       c.Sig,
				CreatedAt: c.CreatedAt,
				Revoked:   c.Revoked,
				FetchedAt: now,
			}
			if c.RevokedAt != nil {
				cached.RevokedAt = c.RevokedAt
			}
			if err := s.SaveCachedConnection(cached); err != nil {
				fmt.Printf("  warning: failed caching connection %s: %v\n", c.ID, err)
				continue
			}
			connsCached++
		}

		sigs, err := fetchUserSignatures(peer.ServerURL, peer.ID, peer.LastSyncedAt)
		if err != nil {
			fmt.Printf("  [%s] signature fetch failed: %v\n", peer.ID, err)
			continue
		}
		sigsCached := 0
		for _, sig := range sigs {
			if err := verifySigned(s, peer.ID, sig.Payload, sig.Algorithm, sig.Sig); err != nil {
				fmt.Printf("  [%s] dropped signature %s: %v\n", peer.ID, sig.ID, err)
				continue
			}
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
				continue
			}
			sigsCached++
		}

		fmt.Printf("Synced %d signature(s) and %d connection(s) from %s\n",
			sigsCached, connsCached, peer.ID)
		totalSigs += sigsCached
		totalConns += connsCached

		if err := s.SetPeerLastSyncedAt(peer.ID, now); err != nil {
			fmt.Printf("  warning: failed updating last-synced for %s: %v\n", peer.ID, err)
		}
	}

	fmt.Printf("Total: %d signature(s) and %d connection(s) cached\n", totalSigs, totalConns)
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
	cachedConns, err := s.GetCachedConnectionsBySigner(userID)
	if err != nil {
		return fmt.Errorf("failed reading cached connections: %w", err)
	}

	if len(cachedSigs) == 0 && len(cachedConns) == 0 {
		fmt.Println("Nothing to publish.")
		return nil
	}

	now := time.Now().UTC()
	for _, srv := range servers {
		sigsPushed := 0
		for _, cached := range cachedSigs {
			pushed, err := s.IsPushed(cached.ID, localstore.ItemSignature, srv.URL)
			if err != nil {
				fmt.Printf("  [%s] push-state read failed: %v\n", srv.URL, err)
				continue
			}
			if pushed {
				continue
			}
			req := sigUploadRequest{
				Payload:   cached.Payload,
				Algorithm: cached.Algorithm,
				Sig:       cached.Sig,
			}
			if _, err := uploadSignature(srv.URL, userID, req); err != nil {
				fmt.Printf("  [%s] signature push failed for %s: %v\n", srv.URL, cached.ID, err)
				continue
			}
			if err := s.RecordPush(cached.ID, localstore.ItemSignature, srv.URL, now); err != nil {
				fmt.Printf("  [%s] warning: failed recording push: %v\n", srv.URL, err)
			}
			sigsPushed++
		}

		connsPushed := 0
		for _, cached := range cachedConns {
			// Only push connections marked public (or revocations of any
			// connection — server-side revoke uses target_id from payload).
			if !connectionShouldBePushed(cached) {
				continue
			}
			pushed, err := s.IsPushed(cached.ID, localstore.ItemConnection, srv.URL)
			if err != nil {
				fmt.Printf("  [%s] push-state read failed: %v\n", srv.URL, err)
				continue
			}
			if pushed {
				continue
			}
			req := connUploadRequest{
				ID:        cached.ID,
				Payload:   cached.Payload,
				Algorithm: cached.Algorithm,
				Sig:       cached.Sig,
			}
			if err := uploadConnection(srv.URL, userID, req); err != nil {
				fmt.Printf("  [%s] connection push failed for %s: %v\n", srv.URL, cached.ID, err)
				continue
			}
			if err := s.RecordPush(cached.ID, localstore.ItemConnection, srv.URL, now); err != nil {
				fmt.Printf("  [%s] warning: failed recording push: %v\n", srv.URL, err)
			}
			connsPushed++
		}

		fmt.Printf("Pushed %d signature(s) and %d connection(s) to %s\n",
			sigsPushed, connsPushed, srv.URL)
	}
	return nil
}

// connectionShouldBePushed decides whether a cached connection record
// should be uploaded to a server. Public connections and any revocation
// payload (regardless of whether the original was public) get pushed;
// private connections stay local.
func connectionShouldBePushed(c *localstore.CachedConnection) bool {
	p, err := models.ParsePayload([]byte(c.Payload))
	if err != nil {
		return false
	}
	if p.Type == models.PayloadTypeConnectionRevocation {
		return true
	}
	if p.Type == models.PayloadTypeConnection {
		return p.Public
	}
	return false
}

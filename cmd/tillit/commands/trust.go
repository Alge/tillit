package commands

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tillit_crypto "github.com/Alge/tillit/crypto"
	"github.com/Alge/tillit/localstore"
	"github.com/Alge/tillit/models"
)

// parsePeer splits "userID@https://server.example.com" into (id, serverURL).
func parsePeer(arg string) (id, serverURL string, err error) {
	at := strings.LastIndex(arg, "@")
	if at < 1 {
		return "", "", fmt.Errorf("peer must be in the form <userID>@<server_url>, got %q", arg)
	}
	return arg[:at], arg[at+1:], nil
}

func Trust(args []string) error {
	// usage: tillit trust <userID@server_url> [--depth N] [--public] [--veto-only]
	if len(args) == 0 {
		return fmt.Errorf("usage: tillit trust <userID@server_url> [--depth N] [--public] [--veto-only]")
	}

	id, serverURL, err := parsePeer(args[0])
	if err != nil {
		return err
	}

	depth := 1
	public := false
	vetoOnly := false
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--depth":
			if i+1 >= len(args) {
				return fmt.Errorf("--depth requires a value")
			}
			i++
			d, err := strconv.Atoi(args[i])
			if err != nil || d < 0 {
				return fmt.Errorf("--depth must be a non-negative integer")
			}
			depth = d
		case "--public":
			public = true
		case "--veto-only":
			vetoOnly = true
		default:
			return fmt.Errorf("unknown flag: %s", args[i])
		}
	}

	s, err := openStore()
	if err != nil {
		return err
	}
	defer s.Close()

	signer, userID, err := activeSignerAndID(s)
	if err != nil {
		return err
	}

	if err := fetchAndCachePubkey(s, serverURL, id); err != nil {
		return fmt.Errorf("failed fetching peer pubkey: %w", err)
	}

	if err := recordTrustChange(s, signer, userID, &localstore.Peer{
		ID:         id,
		ServerURL:  serverURL,
		TrustDepth: depth,
		Public:     public,
		Distrusted: false,
		VetoOnly:   vetoOnly,
	}); err != nil {
		return err
	}

	fmt.Printf("Trusting %s@%s (depth=%d", id, serverURL, depth)
	if public {
		fmt.Print(", public")
	}
	if vetoOnly {
		fmt.Print(", veto-only")
	}
	fmt.Println(")")
	return nil
}

// recordTrustChange revokes any existing active connection from userID to
// peer.ID and writes a fresh signed connection payload reflecting the new
// trust parameters. The peer record is also upserted.
func recordTrustChange(s *localstore.Store, signer tillit_crypto.Signer, userID string, peer *localstore.Peer) error {
	now := time.Now().UTC()

	if err := revokeActiveConnection(s, signer, userID, peer.ID, now); err != nil {
		return err
	}

	connPayload := &models.Payload{
		Type:         models.PayloadTypeConnection,
		Signer:       userID,
		OtherID:      peer.ID,
		Public:       peer.Public,
		Trust:        !peer.Distrusted && !peer.VetoOnly,
		TrustExtends: peer.TrustDepth,
	}
	signed, err := signPayload(signer, connPayload)
	if err != nil {
		return err
	}
	if err := s.SaveCachedConnection(&localstore.CachedConnection{
		ID:        signed.ID,
		Signer:    userID,
		OtherID:   peer.ID,
		Payload:   signed.Payload,
		Algorithm: signed.Algorithm,
		Sig:       signed.Sig,
		CreatedAt: now,
		FetchedAt: now,
	}); err != nil {
		return fmt.Errorf("failed saving connection: %w", err)
	}

	if err := s.SavePeer(peer); err != nil {
		return fmt.Errorf("failed saving peer: %w", err)
	}
	return nil
}

func revokeActiveConnection(s *localstore.Store, signer tillit_crypto.Signer, userID, otherID string, now time.Time) error {
	existing, err := s.GetActiveConnection(userID, otherID)
	if err != nil {
		return fmt.Errorf("failed checking existing connection: %w", err)
	}
	if existing == nil {
		return nil
	}

	revPayload := &models.Payload{
		Type:     models.PayloadTypeConnectionRevocation,
		Signer:   userID,
		TargetID: existing.ID,
	}
	signed, err := signPayload(signer, revPayload)
	if err != nil {
		return err
	}
	revokedAt := now
	if err := s.SaveCachedConnection(&localstore.CachedConnection{
		ID:        signed.ID,
		Signer:    userID,
		OtherID:   otherID,
		Payload:   signed.Payload,
		Algorithm: signed.Algorithm,
		Sig:       signed.Sig,
		CreatedAt: now,
		FetchedAt: now,
	}); err != nil {
		return fmt.Errorf("failed saving revocation: %w", err)
	}

	// Mark the superseded connection as revoked locally too.
	existing.Revoked = true
	existing.RevokedAt = &revokedAt
	if err := s.SaveCachedConnection(existing); err != nil {
		return fmt.Errorf("failed marking superseded connection as revoked: %w", err)
	}
	return nil
}

func Distrust(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: tillit distrust <userID@server_url>")
	}

	id, serverURL, err := parsePeer(args[0])
	if err != nil {
		return err
	}

	s, err := openStore()
	if err != nil {
		return err
	}
	defer s.Close()

	signer, userID, err := activeSignerAndID(s)
	if err != nil {
		return err
	}

	// Distrust is a local concept — we don't expose it on the server, but
	// we do revoke any previously-published trust connection so peers stop
	// inheriting from this person via us.
	if err := recordTrustChange(s, signer, userID, &localstore.Peer{
		ID:         id,
		ServerURL:  serverURL,
		Distrusted: true,
	}); err != nil {
		return err
	}

	fmt.Printf("Distrusting %s@%s\n", id, serverURL)
	return nil
}

func Forget(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: tillit forget <userID@server_url>")
	}

	id, _, err := parsePeer(args[0])
	if err != nil {
		return err
	}

	s, err := openStore()
	if err != nil {
		return err
	}
	defer s.Close()

	signer, userID, err := activeSignerAndID(s)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	if err := revokeActiveConnection(s, signer, userID, id, now); err != nil {
		return err
	}

	if err := s.RemovePeer(id); err != nil {
		return fmt.Errorf("failed removing peer: %w", err)
	}

	fmt.Printf("Removed %s from peers\n", id)
	return nil
}

func TrustList(args []string) error {
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
		fmt.Println("No peers configured.")
		return nil
	}

	for _, p := range peers {
		if p.Distrusted {
			fmt.Printf("  DISTRUST  %s@%s\n", p.ID, p.ServerURL)
		} else if p.VetoOnly {
			extra := ""
			if p.Public {
				extra = ", public"
			}
			fmt.Printf("  veto-only %s@%s (depth=%d%s)\n", p.ID, p.ServerURL, p.TrustDepth, extra)
		} else {
			extra := ""
			if p.Public {
				extra = ", public"
			}
			fmt.Printf("  trust     %s@%s (depth=%d%s)\n", p.ID, p.ServerURL, p.TrustDepth, extra)
		}
	}
	return nil
}

package commands

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Alge/tillit/localstore"
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

	if err := s.SavePeer(&localstore.Peer{
		ID:         id,
		ServerURL:  serverURL,
		TrustDepth: depth,
		Public:     public,
		Distrusted: false,
		VetoOnly:   vetoOnly,
	}); err != nil {
		return fmt.Errorf("failed saving peer: %w", err)
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

	if err := s.SavePeer(&localstore.Peer{
		ID:         id,
		ServerURL:  serverURL,
		Distrusted: true,
	}); err != nil {
		return fmt.Errorf("failed saving peer: %w", err)
	}

	fmt.Printf("Distrusting %s@%s\n", id, serverURL)
	return nil
}

func Untrust(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: tillit untrust <userID@server_url>")
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

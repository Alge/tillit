package commands

import (
	"fmt"

	"github.com/Alge/tillit/localstore"
)

// Status prints a summary of the local store: the active key, peer
// counts, cached data, and any registered servers with their sync /
// pending-push state. Useful before any server is registered too.
func Status(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("usage: tillit status")
	}

	s, err := openStore()
	if err != nil {
		return err
	}
	defer s.Close()

	// Active key. Don't bail if absent — pre-init users still get a useful
	// hint.
	keyName, _ := s.GetActiveKey()
	if keyName == "" {
		fmt.Println("No active key. Run 'tillit init' to generate one.")
		return nil
	}
	key, err := s.GetKey(keyName)
	if err != nil {
		return fmt.Errorf("failed reading active key: %w", err)
	}
	_, userID, err := activeSignerAndID(s)
	if err != nil {
		return err
	}

	fmt.Printf("Active key: %s (%s)\n", key.Name, key.Algorithm)
	fmt.Printf("User ID:    %s\n", userID)
	fmt.Println()

	if err := printPeerSummary(s); err != nil {
		return err
	}
	if err := printDataSummary(s, userID); err != nil {
		return err
	}
	return printServerSummary(s, userID)
}

func printPeerSummary(s *localstore.Store) error {
	peers, err := s.ListPeers()
	if err != nil {
		return fmt.Errorf("list peers: %w", err)
	}
	trusted, distrusted, vetoOnly := 0, 0, 0
	for _, p := range peers {
		switch {
		case p.Distrusted:
			distrusted++
		case p.VetoOnly:
			vetoOnly++
		default:
			trusted++
		}
	}
	fmt.Printf("Peers: %d total (%d trusted, %d veto-only, %d distrusted)\n",
		len(peers), trusted, vetoOnly, distrusted)
	return nil
}

func printDataSummary(s *localstore.Store, userID string) error {
	mySigs, err := s.GetCachedSignaturesBySigner(userID)
	if err != nil {
		return fmt.Errorf("read signatures: %w", err)
	}
	myConns, err := s.GetCachedConnectionsBySigner(userID)
	if err != nil {
		return fmt.Errorf("read connections: %w", err)
	}
	fmt.Printf("Local data: %d signature(s), %d connection(s) signed by you\n",
		len(mySigs), len(myConns))
	fmt.Println()
	return nil
}

func printServerSummary(s *localstore.Store, userID string) error {
	servers, err := s.ListServers()
	if err != nil {
		return fmt.Errorf("list servers: %w", err)
	}
	if len(servers) == 0 {
		fmt.Println("No servers registered.")
		fmt.Println("Use 'tillit register <url>' to start publishing.")
		return nil
	}

	myConns, _ := s.GetCachedConnectionsBySigner(userID)
	mySigs, _ := s.GetCachedSignaturesBySigner(userID)

	totalPending := 0
	for _, srv := range servers {
		pendingConns, err := countPending(s, myConns, localstore.ItemConnection, srv.URL,
			func(c *localstore.CachedConnection) string { return c.ID },
			connectionShouldBePushed)
		if err != nil {
			return err
		}
		pendingSigs, err := countPending(s, mySigs, localstore.ItemSignature, srv.URL,
			func(s *localstore.CachedSignature) string { return s.ID },
			func(*localstore.CachedSignature) bool { return true })
		if err != nil {
			return err
		}

		fmt.Printf("%s\n", srv.URL)
		if srv.LastSyncedAt == nil {
			fmt.Println("  last synced: never")
		} else {
			fmt.Printf("  last synced: %s\n", srv.LastSyncedAt.Format("2006-01-02 15:04:05 MST"))
		}
		if pendingConns == 0 && pendingSigs == 0 {
			fmt.Println("  pending push: none")
		} else {
			fmt.Printf("  pending push: %d connection(s), %d signature(s)\n", pendingConns, pendingSigs)
			totalPending += pendingConns + pendingSigs
		}
	}
	if totalPending > 0 {
		fmt.Printf("\nRun 'tillit publish' to push %d pending item(s).\n", totalPending)
	}
	return nil
}

func countPending[T any](s *localstore.Store, items []T, kind localstore.ItemType, serverURL string,
	idOf func(T) string, shouldPush func(T) bool) (int, error) {
	n := 0
	for _, item := range items {
		if !shouldPush(item) {
			continue
		}
		pushed, err := s.IsPushed(idOf(item), kind, serverURL)
		if err != nil {
			return 0, fmt.Errorf("failed checking push state: %w", err)
		}
		if !pushed {
			n++
		}
	}
	return n, nil
}

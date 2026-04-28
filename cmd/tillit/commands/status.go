package commands

import (
	"fmt"

	"github.com/Alge/tillit/localstore"
)

func Status(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("usage: tillit status")
	}

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

	myConns, err := s.GetCachedConnectionsBySigner(userID)
	if err != nil {
		return fmt.Errorf("failed reading cached connections: %w", err)
	}
	mySigs, err := s.GetCachedSignaturesBySigner(userID)
	if err != nil {
		return fmt.Errorf("failed reading cached signatures: %w", err)
	}

	if len(servers) == 0 {
		fmt.Println("No servers registered.")
		if len(myConns)+len(mySigs) > 0 {
			fmt.Printf("%d local connection(s), %d local signature(s) — register a server with 'tillit register' to publish.\n",
				len(myConns), len(mySigs))
		}
		return nil
	}

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

package commands

import (
	"fmt"
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

	if len(servers) == 0 {
		fmt.Println("No servers registered.")
		if len(myConns) > 0 {
			fmt.Printf("%d local connections — register a server with 'tillit register' to publish.\n", len(myConns))
		}
		return nil
	}

	totalPending := 0
	for _, srv := range servers {
		var pending []string
		for _, c := range myConns {
			pushed, err := s.IsPushed(c.ID, "connection", srv.URL)
			if err != nil {
				return fmt.Errorf("failed checking push state: %w", err)
			}
			if !pushed {
				pending = append(pending, c.ID)
			}
		}
		fmt.Printf("%s\n", srv.URL)
		if srv.LastSyncedAt == nil {
			fmt.Println("  last synced: never")
		} else {
			fmt.Printf("  last synced: %s\n", srv.LastSyncedAt.Format("2006-01-02 15:04:05 MST"))
		}
		if len(pending) == 0 {
			fmt.Println("  pending push: none")
		} else {
			fmt.Printf("  pending push: %d connection(s)\n", len(pending))
			totalPending += len(pending)
		}
	}

	if totalPending > 0 {
		fmt.Printf("\nRun 'tillit publish' to push %d pending item(s).\n", totalPending)
	}
	return nil
}

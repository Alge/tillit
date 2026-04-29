package commands

import (
	"fmt"
	"io"
	"os"

	"github.com/Alge/tillit/localstore"
)

// Delete removes a locally-cached signature outright, but only if it
// has not been pushed to any server yet. Once published, a signature
// is part of the public record and must be revoked instead — other
// peers may already have synced it. The trust model says you can only
// destroy your own pre-publish state, so the signer-ownership check
// matters less here than the pushed-or-not check.
func Delete(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: tillit delete <signature_id>")
	}
	s, err := openStore()
	if err != nil {
		return err
	}
	defer s.Close()
	return runDelete(s, os.Stdout, args[0])
}

func runDelete(s *localstore.Store, w io.Writer, id string) error {
	if id == "" {
		return fmt.Errorf("signature id is empty")
	}
	sig, err := s.LookupCachedSignature(id)
	if err != nil {
		return err
	}

	pushed, err := s.HasBeenPushed(sig.ID, localstore.ItemSignature)
	if err != nil {
		return fmt.Errorf("check push state: %w", err)
	}
	if pushed {
		return fmt.Errorf("signature %s has already been pushed to a server — use 'tillit revoke' instead\n  (deletion is only allowed for signatures that no peer has had a chance to sync)",
			sig.ID)
	}

	if err := s.DeleteCachedSignature(sig.ID); err != nil {
		return fmt.Errorf("delete: %w", err)
	}
	fmt.Fprintf(w, "Deleted unpushed signature %s\n", sig.ID)
	return nil
}

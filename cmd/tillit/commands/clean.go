package commands

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/Alge/tillit/localstore"
	"github.com/Alge/tillit/resolver"
)

// Clean prunes cached signatures/connections/users by signers who
// are no longer reachable in the active user's trust graph. The
// trust set is computed via the same Resolver.TrustSet code path
// that powers `query` and `check` — anything outside it has no
// effect on any verdict, so it's safe to drop. The user is shown a
// summary and asked Y/n before any rows are touched.
func Clean(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("usage: tillit clean")
	}
	s, err := openStore()
	if err != nil {
		return err
	}
	defer s.Close()
	return runClean(s, os.Stdout)
}

// pruneCandidates lists what would be deleted, without touching the
// store. Exposed so the command and tests can share the
// trust-set-derived selection logic.
type pruneCandidates struct {
	Signatures  []*localstore.CachedSignature
	Connections []*localstore.CachedConnection
	Users       []*localstore.CachedUser
}

func (p pruneCandidates) Total() int {
	return len(p.Signatures) + len(p.Connections) + len(p.Users)
}

func runClean(s *localstore.Store, out io.Writer) error {
	userID, err := activeUserID(s)
	if err != nil {
		return fmt.Errorf("clean needs an active key — %w", err)
	}

	candidates, err := findPruneCandidates(s, userID)
	if err != nil {
		return err
	}
	if candidates.Total() == 0 {
		fmt.Fprintln(out, "Nothing to clean — every cached row belongs to a signer in your trust graph.")
		return nil
	}

	printPruneSummary(out, candidates)

	answer, err := promptLine("Proceed with deletion? [y/N]: ")
	if err != nil {
		return fmt.Errorf("read confirmation: %w", err)
	}
	if !isAffirmative(answer) {
		fmt.Fprintln(out, "Aborted. No changes made.")
		return nil
	}

	deleted := applyPrune(s, candidates, out)
	fmt.Fprintf(out, "Cleaned: %d signature(s), %d connection(s), %d cached user(s).\n",
		deleted.Signatures, deleted.Connections, deleted.Users)
	return nil
}

// findPruneCandidates inspects every cached row and returns those
// whose signer is NOT in the viewer's trust set. Distrusted peers
// are excluded from the trust set by buildTrustSet, so their cached
// data is included here — by design, per the user's policy.
func findPruneCandidates(s *localstore.Store, viewer string) (pruneCandidates, error) {
	r := resolver.New(s, viewer)
	entries, err := r.TrustSet(viewer)
	if err != nil {
		return pruneCandidates{}, fmt.Errorf("compute trust set: %w", err)
	}
	live := make(map[string]bool, len(entries))
	for _, e := range entries {
		live[e.SignerID] = true
	}

	allSigs, err := s.ListAllCachedSignatures()
	if err != nil {
		return pruneCandidates{}, fmt.Errorf("list signatures: %w", err)
	}
	allConns, err := s.ListAllCachedConnections()
	if err != nil {
		return pruneCandidates{}, fmt.Errorf("list connections: %w", err)
	}
	allUsers, err := s.ListCachedUsers()
	if err != nil {
		return pruneCandidates{}, fmt.Errorf("list cached users: %w", err)
	}

	var c pruneCandidates
	for _, sig := range allSigs {
		if !live[sig.Signer] {
			c.Signatures = append(c.Signatures, sig)
		}
	}
	for _, conn := range allConns {
		if !live[conn.Signer] {
			c.Connections = append(c.Connections, conn)
		}
	}
	for _, u := range allUsers {
		if !live[u.ID] {
			c.Users = append(c.Users, u)
		}
	}
	return c, nil
}

// printPruneSummary writes a per-signer breakdown of what's about to
// be deleted. Output is sorted for stability and capped at a
// readable number of lines per signer; large pools collapse into a
// "+N more" tail.
func printPruneSummary(out io.Writer, c pruneCandidates) {
	bySigner := map[string]*pruneSummaryRow{}
	add := func(signer string) *pruneSummaryRow {
		row, ok := bySigner[signer]
		if !ok {
			row = &pruneSummaryRow{}
			bySigner[signer] = row
		}
		return row
	}
	for _, sig := range c.Signatures {
		add(sig.Signer).sigs++
	}
	for _, conn := range c.Connections {
		add(conn.Signer).conns++
	}
	for _, u := range c.Users {
		add(u.ID).users++
	}

	signers := make([]string, 0, len(bySigner))
	for s := range bySigner {
		signers = append(signers, s)
	}
	sort.Strings(signers)

	fmt.Fprintf(out, "Would prune cached data for %d signer(s) outside your trust graph:\n", len(signers))
	for _, signer := range signers {
		row := bySigner[signer]
		var parts []string
		if row.sigs > 0 {
			parts = append(parts, fmt.Sprintf("%d sig", row.sigs))
		}
		if row.conns > 0 {
			parts = append(parts, fmt.Sprintf("%d conn", row.conns))
		}
		if row.users > 0 {
			parts = append(parts, "pubkey-cache")
		}
		fmt.Fprintf(out, "  %s — %s\n", shortID(signer), strings.Join(parts, ", "))
	}
	fmt.Fprintf(out, "Totals: %d signature(s), %d connection(s), %d cached user(s).\n",
		len(c.Signatures), len(c.Connections), len(c.Users))
	fmt.Fprintln(out)
}

type pruneSummaryRow struct {
	sigs, conns, users int
}

// applyPrune deletes the candidates from the store, accumulating
// counts of successful deletions. Errors on individual rows are
// printed as warnings but don't abort the run — cleanup is
// best-effort.
func applyPrune(s *localstore.Store, c pruneCandidates, warn io.Writer) (counts struct{ Signatures, Connections, Users int }) {
	for _, sig := range c.Signatures {
		if err := s.DeleteCachedSignature(sig.ID); err != nil {
			fmt.Fprintf(warn, "  warning: signature %s: %v\n", sig.ID, err)
			continue
		}
		counts.Signatures++
	}
	for _, conn := range c.Connections {
		if err := s.DeleteCachedConnection(conn.ID); err != nil {
			fmt.Fprintf(warn, "  warning: connection %s: %v\n", conn.ID, err)
			continue
		}
		counts.Connections++
	}
	for _, u := range c.Users {
		if err := s.DeleteCachedUser(u.ID); err != nil {
			fmt.Fprintf(warn, "  warning: cached user %s: %v\n", u.ID, err)
			continue
		}
		counts.Users++
	}
	return counts
}

// isAffirmative reports whether a Y/n response counts as a "yes".
// Empty (just-Enter) is "no" — safer default for a destructive
// operation.
func isAffirmative(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "y", "yes":
		return true
	}
	return false
}


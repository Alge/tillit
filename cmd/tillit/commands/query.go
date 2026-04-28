package commands

import (
	"fmt"
	"strings"

	"github.com/Alge/tillit/resolver"
)

func Query(args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: tillit query <ecosystem> <package_id>")
	}
	ecosystem, packageID := args[0], args[1]

	s, err := openStore()
	if err != nil {
		return err
	}
	defer s.Close()

	_, userID, err := activeSignerAndID(s)
	if err != nil {
		return err
	}

	r := resolver.New(s, userID)
	pv, err := r.Package(userID, ecosystem, packageID)
	if err != nil {
		return fmt.Errorf("resolve package: %w", err)
	}

	if len(pv.Versions) == 0 {
		fmt.Printf("No trusted decisions about %s/%s.\n", ecosystem, packageID)
		fmt.Println("Try 'tillit sync' to fetch fresh data from your peers.")
		return nil
	}

	fmt.Printf("Versions for %s/%s:\n", ecosystem, packageID)
	for _, g := range resolver.GroupVersions(pv) {
		fmt.Printf("  %-22s %s%s\n", versionRange(g), g.Status, decisionSummary(g))
	}
	return nil
}

func versionRange(g resolver.VersionRange) string {
	if g.From == g.To {
		return g.From
	}
	return g.From + " — " + g.To
}

// decisionSummary returns a short suffix listing the signers (and reasons,
// when present) behind the verdict — enough context to understand why
// without dumping the full structure.
func decisionSummary(g resolver.VersionRange) string {
	signers := map[string]bool{}
	var reasons []string
	for _, d := range g.Decisions {
		signers[d.SignerID] = true
		if d.Reason != "" {
			reasons = append(reasons, d.Reason)
		}
	}
	if len(signers) == 0 {
		return ""
	}

	var names []string
	for s := range signers {
		names = append(names, shortID(s))
	}
	out := " (" + strings.Join(names, ", ")
	if len(reasons) > 0 {
		// Show first reason inline; trim if multiple.
		first := reasons[0]
		if len(first) > 60 {
			first = first[:57] + "..."
		}
		out += ": " + first
		if len(reasons) > 1 {
			out += fmt.Sprintf(" +%d more", len(reasons)-1)
		}
	}
	out += ")"
	return out
}

// shortID truncates a long pubkey-hash ID for display.
func shortID(id string) string {
	if len(id) <= 12 {
		return id
	}
	return id[:8] + "…"
}

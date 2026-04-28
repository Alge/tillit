package commands

import (
	"fmt"
	"sort"
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

	versions := make([]string, 0, len(pv.Versions))
	for v := range pv.Versions {
		versions = append(versions, v)
	}
	cmp := versionComparatorFor(ecosystem)
	sort.Slice(versions, func(i, j int) bool {
		return cmp(versions[i], versions[j]) < 0
	})

	fmt.Printf("Versions for %s/%s:\n", ecosystem, packageID)
	for _, v := range versions {
		ver := pv.Versions[v]
		fmt.Printf("  %-22s %s%s\n", v, ver.Status, decisionSummary(ver))
	}
	return nil
}

// versionComparatorFor returns the comparison function from the ecosystem's
// adapter, falling back to the resolver's generic comparator when no
// adapter is registered.
func versionComparatorFor(ecosystem string) func(a, b string) int {
	if a, ok := adapterForEcosystem(ecosystem); ok {
		return a.CompareVersions
	}
	return resolver.CompareVersions
}

// decisionSummary returns a short suffix listing the signers (and reasons,
// when present) behind the verdict — enough context to understand why
// without dumping the full structure.
func decisionSummary(v resolver.Verdict) string {
	signers := map[string]bool{}
	var reasons []string
	for _, d := range v.Decisions {
		signers[d.SignerID] = true
		if d.Reason != "" {
			reasons = append(reasons, d.Reason)
		}
	}
	if len(signers) == 0 {
		return ""
	}

	names := make([]string, 0, len(signers))
	for s := range signers {
		names = append(names, shortID(s))
	}
	sort.Strings(names)
	out := " (" + strings.Join(names, ", ")
	if len(reasons) > 0 {
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

func shortID(id string) string {
	if len(id) <= 12 {
		return id
	}
	return id[:8] + "…"
}

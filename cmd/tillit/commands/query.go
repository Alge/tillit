package commands

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Alge/tillit/resolver"
)

func Query(args []string) error {
	verbose := false
	positional := args[:0]
	for _, a := range args {
		switch a {
		case "--verbose", "-v":
			verbose = true
		default:
			positional = append(positional, a)
		}
	}
	if len(positional) != 2 {
		return fmt.Errorf("usage: tillit query <ecosystem> <package_id> [--verbose]")
	}
	ecosystem, packageID := positional[0], positional[1]

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

	if len(pv.Spans) == 0 {
		fmt.Printf("No trusted decisions about %s/%s.\n", ecosystem, packageID)
		fmt.Println("Try 'tillit sync' to fetch fresh data from your peers.")
		return nil
	}

	fmt.Printf("Versions for %s/%s:\n", ecosystem, packageID)
	for _, span := range pv.Spans {
		printSpan(span, verbose)
	}
	return nil
}

func printSpan(span resolver.VersionSpan, verbose bool) {
	label := span.From
	if span.From != span.To {
		label = span.From + " — " + span.To
	}
	if !verbose {
		fmt.Printf("  %-22s %s%s\n", label, span.Status, decisionsSummary(span.Decisions))
		return
	}
	fmt.Printf("  %-22s %s\n", label, span.Status)
	for _, d := range span.Decisions {
		fmt.Println("    " + verboseDecisionLine(d))
	}
}

// verboseDecisionLine renders one ContributingDecision in detail: the
// signature kind (exact/delta), level, signer, trust path (when
// transitive), and reason (when present).
func verboseDecisionLine(d resolver.ContributingDecision) string {
	var head string
	switch d.Kind {
	case resolver.KindExact:
		head = fmt.Sprintf("%s @ %s", d.Level, d.Version)
	case resolver.KindDelta:
		head = fmt.Sprintf("%s delta %s → %s", d.Level, d.FromVersion, d.ToVersion)
	default:
		head = d.Level
	}

	by := "by " + shortID(d.SignerID)
	if d.VetoOnly {
		by += " (veto-only)"
	}
	if len(d.Path) > 1 {
		by += " via " + strings.Join(shortPath(d.Path[:len(d.Path)-1]), " → ")
	}

	out := head + " " + by
	if d.Reason != "" {
		out += ": " + d.Reason
	}
	return out
}

func shortPath(p []string) []string {
	out := make([]string, len(p))
	for i, id := range p {
		out[i] = shortID(id)
	}
	return out
}

// decisionsSummary returns a short suffix listing the signers (and reasons,
// when present) behind the verdict — enough context to understand why
// without dumping the full structure.
func decisionsSummary(ds []resolver.ContributingDecision) string {
	signers := map[string]bool{}
	var reasons []string
	for _, d := range ds {
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

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

	userID, err := activeUserID(s)
	if err != nil {
		return err
	}

	r := resolver.New(s, userID)
	pv, err := r.Package(userID, ecosystem, packageID)
	if err != nil {
		return fmt.Errorf("resolve package: %w", err)
	}

	if len(pv.Spans) == 0 {
		if verbose && len(pv.Revoked) > 0 {
			fmt.Printf("No active decisions about %s/%s — all have been revoked.\n", ecosystem, packageID)
			printRevokedSection(pv.Revoked)
			return nil
		}
		fmt.Printf("No trusted decisions about %s/%s.\n", ecosystem, packageID)
		if !verbose && len(pv.Revoked) > 0 {
			fmt.Printf("(%d revoked signature(s) hidden — re-run with --verbose to see them.)\n", len(pv.Revoked))
		}
		fmt.Println("Try 'tillit sync' to fetch fresh data from your peers.")
		return nil
	}

	fmt.Printf("Versions for %s/%s:\n", ecosystem, packageID)
	for _, span := range pv.Spans {
		printSpan(span, verbose)
	}
	if verbose && len(pv.Revoked) > 0 {
		printRevokedSection(pv.Revoked)
	}
	return nil
}

// printRevokedSection lists revoked signatures from trusted signers so
// the user can see what was withdrawn. Only called in --verbose mode.
func printRevokedSection(revoked []resolver.ContributingDecision) {
	fmt.Println("Revoked:")
	for _, d := range revoked {
		fmt.Println("  " + verboseDecisionLine(d) + " [revoked]")
	}
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
	if d.SignatureID != "" {
		out += " " + shortHash(d.SignatureID)
	}
	if d.Reason != "" {
		out += ": " + d.Reason
	}
	return out
}

// shortHash returns a 12-char hex prefix of a content-hash signature ID.
// 48 bits gives a birthday-collision ceiling of millions of signatures,
// well past any realistic per-user store. No leading sigil — Git-style:
// context (column position, the word "by" preceding it) tells the reader
// it's a hash, and a bare hex string survives shell paste without quoting.
func shortHash(id string) string {
	const n = 12
	if len(id) <= n {
		return id
	}
	return id[:n]
}

func shortPath(p []string) []string {
	out := make([]string, len(p))
	for i, id := range p {
		out[i] = shortID(id)
	}
	return out
}

// decisionsSummary returns a short suffix listing the signers (with the
// short hash of one contributing signature each) and reasons behind the
// verdict — enough context to understand why without dumping the full
// structure. The hash lets the user feed it to `inspect` for details.
func decisionsSummary(ds []resolver.ContributingDecision) string {
	type signerInfo struct {
		hash string
	}
	signers := map[string]signerInfo{}
	var reasons []string
	for _, d := range ds {
		if _, ok := signers[d.SignerID]; !ok {
			signers[d.SignerID] = signerInfo{hash: d.SignatureID}
		}
		if d.Reason != "" {
			reasons = append(reasons, d.Reason)
		}
	}
	if len(signers) == 0 {
		return ""
	}

	names := make([]string, 0, len(signers))
	for s := range signers {
		names = append(names, s)
	}
	sort.Strings(names)

	parts := make([]string, len(names))
	for i, s := range names {
		entry := shortID(s)
		if h := signers[s].hash; h != "" {
			entry += " " + shortHash(h)
		}
		parts[i] = entry
	}
	out := " (" + strings.Join(parts, ", ")
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

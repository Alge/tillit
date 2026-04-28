// Package resolver computes trust verdicts about packages from a local
// cache populated by sync. All resolution is local — no network calls.
//
// The resolver answers one underlying question:
//
//   "From <viewer>'s perspective, what verdict does each known version of
//   <package> get?"
//
// The point-version query (Version) is a thin wrapper that picks one entry
// out of the package-wide result.
//
// The viewer parameter lets the resolver answer questions from any user's
// perspective, not just the local active user. When viewer matches the
// local active user, the resolver consults the peers table for direct
// edges (which carries local-only trust modes like veto-only). For other
// viewers it relies on the public cached_connections records.
package resolver

import (
	"github.com/Alge/tillit/localstore"
)

type Resolver struct {
	store        *localstore.Store
	activeUserID string
}

// New constructs a Resolver. activeUserID is the local user — the resolver
// uses the peers table for direct edges from this user (since veto-only is
// only stored locally). Pass empty string if no active user is configured.
func New(store *localstore.Store, activeUserID string) *Resolver {
	return &Resolver{store: store, activeUserID: activeUserID}
}

// Status is the resolved verdict level. Unlike DecisionLevel (what someone
// signs), Status includes Unknown — the absence of any decision.
type Status string

const (
	StatusAllowed  Status = "allowed"
	StatusVetted   Status = "vetted"
	StatusRejected Status = "rejected"
	StatusUnknown  Status = "unknown"
)

// Verdict is the resolved status for a single version, plus the decisions
// from the trust graph that contributed to it.
type Verdict struct {
	Status    Status
	Decisions []ContributingDecision
}

// PackageVerdict summarises everything we know about a package from the
// viewer's perspective as a sorted, merged list of trust spans. Each
// VersionSpan covers a contiguous run of versions sharing one status.
// A delta(A, B) signature contributes a span [A, B] (extending trust to
// every version in between) when its base is trusted; an exact decision
// contributes a single-version span.
type PackageVerdict struct {
	Ecosystem string
	PackageID string
	Spans     []VersionSpan
}

// VersionSpan covers a contiguous run of versions sharing one Status.
// From == To for single-version spans (exact decisions). For broader
// spans, From is the lowest covered version and To is the highest.
type VersionSpan struct {
	From      string
	To        string
	Status    Status
	Decisions []ContributingDecision
}

// ContributingDecision is one signed decision from one signer in the trust
// graph that contributed to a verdict. Path traces the trust chain from
// the viewer to the signer.
type ContributingDecision struct {
	SignerID    string
	Path        []string // viewer → A → B → SignerID
	Level       string   // the payload's DecisionLevel
	Reason      string
	SignatureID string
	VetoOnly    bool // the path included a veto-only edge
}

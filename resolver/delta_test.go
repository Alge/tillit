package resolver

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Alge/tillit/localstore"
	"github.com/Alge/tillit/models"
	"github.com/google/uuid"
)

func addDeltaDecision(t *testing.T, s *localstore.Store, signer, ecosystem, pkg, from, to string, level models.DecisionLevel, reason string) string {
	t.Helper()
	now := time.Now().UTC()
	payload := &models.Payload{
		Type:        models.PayloadTypeDeltaDecision,
		Signer:      signer,
		Ecosystem:   ecosystem,
		PackageID:   pkg,
		FromVersion: from,
		ToVersion:   to,
		Level:       level,
		Reason:      reason,
	}
	bytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	id := uuid.NewString()
	if err := s.SaveCachedSignature(&localstore.CachedSignature{
		ID:         id,
		Signer:     signer,
		Payload:    string(bytes),
		Algorithm:  "ed25519",
		Sig:        "fake",
		UploadedAt: now,
		FetchedAt:  now,
	}); err != nil {
		t.Fatalf("SaveCachedSignature: %v", err)
	}
	return id
}

func TestPackage_DeltaChainExtendsTrust(t *testing.T) {
	s := newTestStore(t)
	addPeer(t, s, &localstore.Peer{ID: "alice", ServerURL: "https://x", TrustDepth: 0})
	addDecision(t, s, "alice", "go", "p", "v1.0.0", models.DecisionVetted, "")
	addDeltaDecision(t, s, "alice", "go", "p", "v1.0.0", "v1.1.0", models.DecisionVetted, "")

	r := New(s, "me")
	if verdictFor(t, r, "v1.0.0").Status != StatusVetted {
		t.Error("v1.0.0 should be vetted")
	}
	if verdictFor(t, r, "v1.1.0").Status != StatusVetted {
		t.Error("v1.1.0 should be vetted via delta")
	}
}

func TestPackage_DeltaCoversIntermediateVersions(t *testing.T) {
	s := newTestStore(t)
	addPeer(t, s, &localstore.Peer{ID: "alice", ServerURL: "https://x", TrustDepth: 0})
	// Reviewing v1.0.0 → v1.5.0 implicitly trusts every version in
	// between, since the cumulative diff includes all intermediate changes.
	addDecision(t, s, "alice", "go", "p", "v1.0.0", models.DecisionVetted, "")
	addDeltaDecision(t, s, "alice", "go", "p", "v1.0.0", "v1.5.0", models.DecisionVetted, "")

	r := New(s, "me")
	for _, ver := range []string{"v1.0.0", "v1.1.0", "v1.2.5", "v1.5.0"} {
		if verdictFor(t, r, ver).Status != StatusVetted {
			t.Errorf("%s should be vetted (in delta range)", ver)
		}
	}
}

func TestPackage_DeltaWithoutBaseTrust_NotApplied(t *testing.T) {
	s := newTestStore(t)
	addPeer(t, s, &localstore.Peer{ID: "alice", ServerURL: "https://x", TrustDepth: 0})
	addDeltaDecision(t, s, "alice", "go", "p", "v1.0.0", "v1.1.0", models.DecisionVetted, "")

	r := New(s, "me")
	if v := verdictFor(t, r, "v1.1.0"); v.Status != StatusUnknown {
		t.Errorf("v1.1.0 should be unknown — base v1.0.0 isn't trusted, got %q", v.Status)
	}
}

func TestPackage_DeltaRejectIsUnconditional(t *testing.T) {
	s := newTestStore(t)
	addPeer(t, s, &localstore.Peer{ID: "alice", ServerURL: "https://x", TrustDepth: 0})
	addDeltaDecision(t, s, "alice", "go", "p", "v1.0.0", "v1.1.0", models.DecisionRejected, "introduced backdoor")

	r := New(s, "me")
	if v := verdictFor(t, r, "v1.1.0"); v.Status != StatusRejected {
		t.Errorf("expected rejected from delta alone, got %q", v.Status)
	}
}

func TestPackage_DeltaChainMultipleHops(t *testing.T) {
	s := newTestStore(t)
	addPeer(t, s, &localstore.Peer{ID: "alice", ServerURL: "https://x", TrustDepth: 0})
	addDecision(t, s, "alice", "go", "p", "v1.0.0", models.DecisionVetted, "")
	addDeltaDecision(t, s, "alice", "go", "p", "v1.0.0", "v1.1.0", models.DecisionVetted, "")
	addDeltaDecision(t, s, "alice", "go", "p", "v1.1.0", "v1.2.0", models.DecisionVetted, "")
	addDeltaDecision(t, s, "alice", "go", "p", "v1.2.0", "v1.3.0", models.DecisionVetted, "")

	r := New(s, "me")
	for _, ver := range []string{"v1.0.0", "v1.1.0", "v1.2.0", "v1.3.0"} {
		if verdictFor(t, r, ver).Status != StatusVetted {
			t.Errorf("%s should be vetted (chain)", ver)
		}
	}
}

func TestPackage_DeltaChainBreaksOnRejected(t *testing.T) {
	s := newTestStore(t)
	addPeer(t, s, &localstore.Peer{ID: "alice", ServerURL: "https://x", TrustDepth: 0})
	addPeer(t, s, &localstore.Peer{ID: "bob", ServerURL: "https://x", TrustDepth: 0})
	addDecision(t, s, "alice", "go", "p", "v1.0.0", models.DecisionVetted, "")
	addDeltaDecision(t, s, "alice", "go", "p", "v1.0.0", "v1.1.0", models.DecisionVetted, "")
	addDecision(t, s, "bob", "go", "p", "v1.1.0", models.DecisionRejected, "CVE")
	addDeltaDecision(t, s, "alice", "go", "p", "v1.1.0", "v1.2.0", models.DecisionVetted, "")

	r := New(s, "me")
	if verdictFor(t, r, "v1.1.0").Status != StatusRejected {
		t.Error("v1.1.0 should be rejected (exact)")
	}
	if v := verdictFor(t, r, "v1.2.0"); v.Status != StatusUnknown {
		t.Errorf("v1.2.0 should not be trusted — delta base v1.1.0 is rejected, got %q", v.Status)
	}
}

func TestPackage_VetoOnlySignerDeltaApprovalIgnored(t *testing.T) {
	s := newTestStore(t)
	addPeer(t, s, &localstore.Peer{ID: "alice", ServerURL: "https://x", TrustDepth: 0, VetoOnly: true})
	addDeltaDecision(t, s, "alice", "go", "p", "v1.0.0", "v1.1.0", models.DecisionVetted, "looks fine")

	r := New(s, "me")
	if v := verdictFor(t, r, "v1.1.0"); v.Status != StatusUnknown {
		t.Errorf("veto-only signer's delta approval must be ignored, got %q", v.Status)
	}
}

func TestPackage_VetoOnlySignerDeltaRejectionCounts(t *testing.T) {
	s := newTestStore(t)
	addPeer(t, s, &localstore.Peer{ID: "cve-bot", ServerURL: "https://x", TrustDepth: 0, VetoOnly: true})
	addDeltaDecision(t, s, "cve-bot", "go", "p", "v1.0.0", "v1.1.0", models.DecisionRejected, "CVE-2024-...")

	r := New(s, "me")
	if v := verdictFor(t, r, "v1.1.0"); v.Status != StatusRejected {
		t.Errorf("expected rejected from veto-only delta, got %q", v.Status)
	}
}

func TestPackage_DeltaSpansMergeIntoLongestChain(t *testing.T) {
	s := newTestStore(t)
	addPeer(t, s, &localstore.Peer{ID: "alice", ServerURL: "https://x", TrustDepth: 0})
	addDecision(t, s, "alice", "go", "p", "v3.0.0", models.DecisionAllowed, "")
	addDeltaDecision(t, s, "alice", "go", "p", "v3.0.0", "v3.5.0", models.DecisionAllowed, "")
	addDeltaDecision(t, s, "alice", "go", "p", "v3.0.0", "v3.7.0", models.DecisionAllowed, "")

	r := New(s, "me")
	pv, _ := r.Package("me", "go", "p")

	// All three sigs share v3.0.0 and have the same status — should
	// merge into one span [v3.0.0, v3.7.0].
	if len(pv.Spans) != 1 {
		t.Fatalf("expected 1 merged span, got %d: %+v", len(pv.Spans), pv.Spans)
	}
	span := pv.Spans[0]
	if span.From != "v3.0.0" || span.To != "v3.7.0" || span.Status != StatusAllowed {
		t.Errorf("unexpected span: %+v", span)
	}
}

func TestPackage_DisjointSpansStaySeparate(t *testing.T) {
	s := newTestStore(t)
	addPeer(t, s, &localstore.Peer{ID: "alice", ServerURL: "https://x", TrustDepth: 0})
	addDecision(t, s, "alice", "go", "p", "v2.1.0", models.DecisionAllowed, "")
	addDecision(t, s, "alice", "go", "p", "v3.0.0", models.DecisionAllowed, "")
	addDeltaDecision(t, s, "alice", "go", "p", "v3.0.0", "v3.5.0", models.DecisionAllowed, "")

	r := New(s, "me")
	pv, _ := r.Package("me", "go", "p")
	if len(pv.Spans) != 2 {
		t.Fatalf("expected 2 spans (v2.1.0 alone + v3.0.0–v3.5.0), got %d: %+v", len(pv.Spans), pv.Spans)
	}
	if pv.Spans[0].From != "v2.1.0" || pv.Spans[0].To != "v2.1.0" {
		t.Errorf("first span = %+v", pv.Spans[0])
	}
	if pv.Spans[1].From != "v3.0.0" || pv.Spans[1].To != "v3.5.0" {
		t.Errorf("second span = %+v", pv.Spans[1])
	}
}

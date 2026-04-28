package resolver

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Alge/tillit/localstore"
	"github.com/Alge/tillit/models"
	"github.com/google/uuid"
)

func addDiffDecision(t *testing.T, s *localstore.Store, signer, ecosystem, pkg, from, to string, level models.DecisionLevel, reason string) string {
	t.Helper()
	now := time.Now().UTC()
	payload := &models.Payload{
		Type:        models.PayloadTypeDiffDecision,
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

func TestPackage_DiffChainExtendsTrust(t *testing.T) {
	s := newTestStore(t)
	addPeer(t, s, &localstore.Peer{ID: "alice", ServerURL: "https://x", TrustDepth: 0})
	// Exact vetted on v1.0.0; diff vetted from v1.0.0 to v1.1.0 — both
	// should be trusted from me.
	addDecision(t, s, "alice", "go", "p", "v1.0.0", models.DecisionVetted, "")
	addDiffDecision(t, s, "alice", "go", "p", "v1.0.0", "v1.1.0", models.DecisionVetted, "")

	r := New(s, "me")
	pv, _ := r.Package("me", "go", "p")
	if v := pv.Versions["v1.0.0"]; v.Status != StatusVetted {
		t.Errorf("v1.0.0 = %q, want vetted", v.Status)
	}
	if v := pv.Versions["v1.1.0"]; v.Status != StatusVetted {
		t.Errorf("v1.1.0 = %q, want vetted (extended via diff)", v.Status)
	}
}

func TestPackage_DiffWithoutBaseTrust_NotApplied(t *testing.T) {
	s := newTestStore(t)
	addPeer(t, s, &localstore.Peer{ID: "alice", ServerURL: "https://x", TrustDepth: 0})
	// Only the diff signed, no base — diff confers no trust on its own.
	addDiffDecision(t, s, "alice", "go", "p", "v1.0.0", "v1.1.0", models.DecisionVetted, "")

	r := New(s, "me")
	pv, _ := r.Package("me", "go", "p")
	if _, ok := pv.Versions["v1.1.0"]; ok {
		t.Errorf("v1.1.0 should be unknown — base v1.0.0 isn't trusted, got %+v", pv.Versions)
	}
}

func TestPackage_DiffRejectIsUnconditional(t *testing.T) {
	s := newTestStore(t)
	addPeer(t, s, &localstore.Peer{ID: "alice", ServerURL: "https://x", TrustDepth: 0})
	// A diff REJECTION applies regardless of whether the base is trusted —
	// "I looked at this diff and it's bad" stands alone.
	addDiffDecision(t, s, "alice", "go", "p", "v1.0.0", "v1.1.0", models.DecisionRejected, "introduced backdoor")

	r := New(s, "me")
	pv, _ := r.Package("me", "go", "p")
	if v := pv.Versions["v1.1.0"]; v.Status != StatusRejected {
		t.Errorf("expected rejected from diff alone, got %+v", v)
	}
}

func TestPackage_DiffChainMultipleHops(t *testing.T) {
	s := newTestStore(t)
	addPeer(t, s, &localstore.Peer{ID: "alice", ServerURL: "https://x", TrustDepth: 0})
	// v1.0.0 vetted; diffs v1.0→v1.1, v1.1→v1.2, v1.2→v1.3 — all should
	// be trusted by chain.
	addDecision(t, s, "alice", "go", "p", "v1.0.0", models.DecisionVetted, "")
	addDiffDecision(t, s, "alice", "go", "p", "v1.0.0", "v1.1.0", models.DecisionVetted, "")
	addDiffDecision(t, s, "alice", "go", "p", "v1.1.0", "v1.2.0", models.DecisionVetted, "")
	addDiffDecision(t, s, "alice", "go", "p", "v1.2.0", "v1.3.0", models.DecisionVetted, "")

	r := New(s, "me")
	pv, _ := r.Package("me", "go", "p")
	for _, ver := range []string{"v1.0.0", "v1.1.0", "v1.2.0", "v1.3.0"} {
		if v := pv.Versions[ver]; v.Status != StatusVetted {
			t.Errorf("%s = %q, want vetted (chain)", ver, v.Status)
		}
	}
}

func TestPackage_DiffChainBreaksOnRejected(t *testing.T) {
	s := newTestStore(t)
	addPeer(t, s, &localstore.Peer{ID: "alice", ServerURL: "https://x", TrustDepth: 0})
	addPeer(t, s, &localstore.Peer{ID: "bob", ServerURL: "https://x", TrustDepth: 0})
	// alice vetted v1.0.0, diff'd to v1.1.0
	// bob rejected v1.1.0 → v1.1.0 verdict is rejected
	// alice diff-vetted v1.1.0 → v1.2.0 — should NOT extend trust because
	// v1.1.0 is rejected.
	addDecision(t, s, "alice", "go", "p", "v1.0.0", models.DecisionVetted, "")
	addDiffDecision(t, s, "alice", "go", "p", "v1.0.0", "v1.1.0", models.DecisionVetted, "")
	addDecision(t, s, "bob", "go", "p", "v1.1.0", models.DecisionRejected, "CVE")
	addDiffDecision(t, s, "alice", "go", "p", "v1.1.0", "v1.2.0", models.DecisionVetted, "")

	r := New(s, "me")
	pv, _ := r.Package("me", "go", "p")
	if v := pv.Versions["v1.1.0"]; v.Status != StatusRejected {
		t.Errorf("v1.1.0 = %q, want rejected", v.Status)
	}
	if _, ok := pv.Versions["v1.2.0"]; ok {
		t.Errorf("v1.2.0 should not be trusted — diff base v1.1.0 is rejected, got %+v", pv.Versions["v1.2.0"])
	}
}

func TestPackage_VetoOnlySignerDiffApprovalIgnored(t *testing.T) {
	s := newTestStore(t)
	addPeer(t, s, &localstore.Peer{ID: "alice", ServerURL: "https://x", TrustDepth: 0, VetoOnly: true})
	addDiffDecision(t, s, "alice", "go", "p", "v1.0.0", "v1.1.0", models.DecisionVetted, "looks fine")

	r := New(s, "me")
	pv, _ := r.Package("me", "go", "p")
	if _, ok := pv.Versions["v1.1.0"]; ok {
		t.Errorf("veto-only signer's diff approval must be ignored, got %+v", pv.Versions)
	}
}

func TestPackage_VetoOnlySignerDiffRejectionCounts(t *testing.T) {
	s := newTestStore(t)
	addPeer(t, s, &localstore.Peer{ID: "cve-bot", ServerURL: "https://x", TrustDepth: 0, VetoOnly: true})
	addDiffDecision(t, s, "cve-bot", "go", "p", "v1.0.0", "v1.1.0", models.DecisionRejected, "CVE-2024-...")

	r := New(s, "me")
	pv, _ := r.Package("me", "go", "p")
	if v := pv.Versions["v1.1.0"]; v.Status != StatusRejected {
		t.Errorf("expected rejected from veto-only diff, got %+v", v)
	}
}

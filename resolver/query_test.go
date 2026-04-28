package resolver

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Alge/tillit/localstore"
	"github.com/Alge/tillit/models"
	"github.com/google/uuid"
)

// addDecision writes a cached_signatures row for an exact-version
// vetting decision by signer.
func addDecision(t *testing.T, s *localstore.Store, signer, ecosystem, pkg, version string, level models.DecisionLevel, reason string) string {
	t.Helper()
	now := time.Now().UTC()
	payload := &models.Payload{
		Type:      models.PayloadTypeDecision,
		Signer:    signer,
		Ecosystem: ecosystem,
		PackageID: pkg,
		Version:   version,
		Level:     level,
		Reason:    reason,
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

func revokeSignature(t *testing.T, s *localstore.Store, id string) {
	t.Helper()
	c, err := s.GetCachedSignature(id)
	if err != nil {
		t.Fatalf("GetCachedSignature: %v", err)
	}
	now := time.Now().UTC()
	c.Revoked = true
	c.RevokedAt = &now
	if err := s.SaveCachedSignature(c); err != nil {
		t.Fatalf("re-save revoked: %v", err)
	}
}

// --- Package query --------------------------------------------------------

func TestPackage_NoTrustedSigners_EmptyMap(t *testing.T) {
	s := newTestStore(t)
	// alice is not trusted by me.
	addDecision(t, s, "alice", "go", "github.com/foo/bar", "v1.0.0", models.DecisionVetted, "")

	r := New(s, "me")
	pv, err := r.Package("me", "go", "github.com/foo/bar")
	if err != nil {
		t.Fatalf("Package: %v", err)
	}
	if len(pv.Versions) != 0 {
		t.Errorf("expected empty Versions map, got %+v", pv.Versions)
	}
}

func TestPackage_SingleTrustedSigner(t *testing.T) {
	s := newTestStore(t)
	addPeer(t, s, &localstore.Peer{ID: "alice", ServerURL: "https://x", TrustDepth: 0})
	addDecision(t, s, "alice", "go", "github.com/foo/bar", "v1.0.0", models.DecisionVetted, "looks fine")

	r := New(s, "me")
	pv, _ := r.Package("me", "go", "github.com/foo/bar")
	v, ok := pv.Versions["v1.0.0"]
	if !ok {
		t.Fatalf("expected v1.0.0 in result, got %+v", pv.Versions)
	}
	if v.Status != StatusVetted {
		t.Errorf("Status = %q, want vetted", v.Status)
	}
	if len(v.Decisions) != 1 || v.Decisions[0].SignerID != "alice" {
		t.Errorf("expected 1 decision from alice, got %+v", v.Decisions)
	}
}

func TestPackage_MultipleVersions(t *testing.T) {
	s := newTestStore(t)
	addPeer(t, s, &localstore.Peer{ID: "alice", ServerURL: "https://x", TrustDepth: 0})
	addDecision(t, s, "alice", "go", "p", "v1.0.0", models.DecisionAllowed, "")
	addDecision(t, s, "alice", "go", "p", "v1.1.0", models.DecisionVetted, "")
	addDecision(t, s, "alice", "go", "p", "v2.0.0", models.DecisionRejected, "broken")

	r := New(s, "me")
	pv, _ := r.Package("me", "go", "p")
	if len(pv.Versions) != 3 {
		t.Fatalf("expected 3 versions, got %d", len(pv.Versions))
	}
	if pv.Versions["v1.0.0"].Status != StatusAllowed {
		t.Errorf("v1.0.0 status = %q", pv.Versions["v1.0.0"].Status)
	}
	if pv.Versions["v1.1.0"].Status != StatusVetted {
		t.Errorf("v1.1.0 status = %q", pv.Versions["v1.1.0"].Status)
	}
	if pv.Versions["v2.0.0"].Status != StatusRejected {
		t.Errorf("v2.0.0 status = %q", pv.Versions["v2.0.0"].Status)
	}
}

func TestPackage_RejectedWinsOverVetted(t *testing.T) {
	s := newTestStore(t)
	addPeer(t, s, &localstore.Peer{ID: "alice", ServerURL: "https://x", TrustDepth: 0})
	addPeer(t, s, &localstore.Peer{ID: "bob", ServerURL: "https://x", TrustDepth: 0})
	addDecision(t, s, "alice", "go", "p", "v1.0.0", models.DecisionVetted, "")
	addDecision(t, s, "bob", "go", "p", "v1.0.0", models.DecisionRejected, "CVE")

	r := New(s, "me")
	pv, _ := r.Package("me", "go", "p")
	if pv.Versions["v1.0.0"].Status != StatusRejected {
		t.Errorf("rejected must win, got %+v", pv.Versions["v1.0.0"])
	}
}

func TestPackage_VettedWinsOverAllowed(t *testing.T) {
	s := newTestStore(t)
	addPeer(t, s, &localstore.Peer{ID: "alice", ServerURL: "https://x", TrustDepth: 0})
	addPeer(t, s, &localstore.Peer{ID: "bob", ServerURL: "https://x", TrustDepth: 0})
	addDecision(t, s, "alice", "go", "p", "v1.0.0", models.DecisionAllowed, "")
	addDecision(t, s, "bob", "go", "p", "v1.0.0", models.DecisionVetted, "code review done")

	r := New(s, "me")
	pv, _ := r.Package("me", "go", "p")
	if pv.Versions["v1.0.0"].Status != StatusVetted {
		t.Errorf("vetted must beat allowed, got %+v", pv.Versions["v1.0.0"])
	}
}

func TestPackage_VetoOnlyApprovalIgnored(t *testing.T) {
	s := newTestStore(t)
	// alice is veto-only — her vetted decision is dropped, only rejections count.
	addPeer(t, s, &localstore.Peer{ID: "alice", ServerURL: "https://x", TrustDepth: 0, VetoOnly: true})
	addDecision(t, s, "alice", "go", "p", "v1.0.0", models.DecisionVetted, "trusted me bro")

	r := New(s, "me")
	pv, _ := r.Package("me", "go", "p")
	if _, ok := pv.Versions["v1.0.0"]; ok {
		t.Errorf("veto-only signer's vetted decision must be ignored, got %+v", pv.Versions)
	}
}

func TestPackage_VetoOnlyRejectionCounts(t *testing.T) {
	s := newTestStore(t)
	addPeer(t, s, &localstore.Peer{ID: "alice", ServerURL: "https://x", TrustDepth: 0, VetoOnly: true})
	addDecision(t, s, "alice", "go", "p", "v1.0.0", models.DecisionRejected, "CVE-2024-...")

	r := New(s, "me")
	pv, _ := r.Package("me", "go", "p")
	v, ok := pv.Versions["v1.0.0"]
	if !ok || v.Status != StatusRejected {
		t.Errorf("veto-only signer's rejection must count, got %+v", pv.Versions)
	}
	if len(v.Decisions) != 1 || !v.Decisions[0].VetoOnly {
		t.Errorf("expected decision flagged VetoOnly=true, got %+v", v.Decisions)
	}
}

func TestPackage_RevokedSignatureIgnored(t *testing.T) {
	s := newTestStore(t)
	addPeer(t, s, &localstore.Peer{ID: "alice", ServerURL: "https://x", TrustDepth: 0})
	id := addDecision(t, s, "alice", "go", "p", "v1.0.0", models.DecisionVetted, "")
	revokeSignature(t, s, id)

	r := New(s, "me")
	pv, _ := r.Package("me", "go", "p")
	if _, ok := pv.Versions["v1.0.0"]; ok {
		t.Errorf("revoked signature must not contribute, got %+v", pv.Versions)
	}
}

func TestPackage_FiltersByEcosystemAndPackage(t *testing.T) {
	s := newTestStore(t)
	addPeer(t, s, &localstore.Peer{ID: "alice", ServerURL: "https://x", TrustDepth: 0})
	addDecision(t, s, "alice", "go", "github.com/foo/bar", "v1.0.0", models.DecisionVetted, "")
	addDecision(t, s, "alice", "go", "github.com/foo/baz", "v1.0.0", models.DecisionVetted, "")
	addDecision(t, s, "alice", "pip", "github.com/foo/bar", "v1.0.0", models.DecisionVetted, "")

	r := New(s, "me")
	pv, _ := r.Package("me", "go", "github.com/foo/bar")
	if len(pv.Versions) != 1 {
		t.Errorf("expected only the matching package/ecosystem, got %+v", pv.Versions)
	}
}

func TestPackage_TransitiveTrust(t *testing.T) {
	s := newTestStore(t)
	// me trusts alice with depth=1; alice trusts bob (TrustExtends=0);
	// bob's signature should count.
	addPeer(t, s, &localstore.Peer{ID: "alice", ServerURL: "https://x", TrustDepth: 1})
	addConnection(t, s, "alice", "bob", 0)
	addDecision(t, s, "bob", "go", "p", "v1.0.0", models.DecisionVetted, "")

	r := New(s, "me")
	pv, _ := r.Package("me", "go", "p")
	v, ok := pv.Versions["v1.0.0"]
	if !ok || v.Status != StatusVetted {
		t.Errorf("expected vetted via transitive trust, got %+v", pv.Versions)
	}
}

// --- Version query --------------------------------------------------------

func TestVersion_KnownVersion(t *testing.T) {
	s := newTestStore(t)
	addPeer(t, s, &localstore.Peer{ID: "alice", ServerURL: "https://x", TrustDepth: 0})
	addDecision(t, s, "alice", "go", "p", "v1.0.0", models.DecisionVetted, "")

	r := New(s, "me")
	v, _ := r.Version("me", "go", "p", "v1.0.0")
	if v.Status != StatusVetted {
		t.Errorf("Version = %q, want vetted", v.Status)
	}
}

func TestVersion_UnknownVersion(t *testing.T) {
	s := newTestStore(t)
	addPeer(t, s, &localstore.Peer{ID: "alice", ServerURL: "https://x", TrustDepth: 0})
	addDecision(t, s, "alice", "go", "p", "v1.0.0", models.DecisionVetted, "")

	r := New(s, "me")
	v, _ := r.Version("me", "go", "p", "v9.9.9")
	if v.Status != StatusUnknown {
		t.Errorf("expected unknown for absent version, got %q", v.Status)
	}
	if len(v.Decisions) != 0 {
		t.Errorf("unknown verdict should have no decisions, got %+v", v.Decisions)
	}
}

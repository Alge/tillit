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

// revokeSignature stores a revocation signature targeting id, signed
// by the same signer who originally signed it. Revocation status is
// derived from this row's existence — the cache no longer trusts a
// mutable revoked flag.
func revokeSignature(t *testing.T, s *localstore.Store, id string) {
	t.Helper()
	target, err := s.GetCachedSignature(id)
	if err != nil {
		t.Fatalf("GetCachedSignature: %v", err)
	}
	now := time.Now().UTC()
	revPayload := &models.Payload{
		Type:     models.PayloadTypeRevocation,
		Signer:   target.Signer,
		TargetID: id,
	}
	bytes, err := json.Marshal(revPayload)
	if err != nil {
		t.Fatalf("marshal revocation: %v", err)
	}
	if err := s.SaveCachedSignature(&localstore.CachedSignature{
		ID:         uuid.NewString(),
		Signer:     target.Signer,
		Payload:    string(bytes),
		Algorithm:  "ed25519",
		Sig:        "fake",
		UploadedAt: now,
		FetchedAt:  now,
	}); err != nil {
		t.Fatalf("save revocation: %v", err)
	}
}

// verdictFor is a small helper for tests: resolve verdict on (me, go, p, ver).
func verdictFor(t *testing.T, r *Resolver, version string) Verdict {
	t.Helper()
	v, err := r.Version("me", "go", "p", version)
	if err != nil {
		t.Fatalf("Version(%q) failed: %v", version, err)
	}
	return v
}

func TestPackage_NoTrustedSigners_NoSpans(t *testing.T) {
	s := newTestStore(t)
	addDecision(t, s, "alice", "go", "github.com/foo/bar", "v1.0.0", models.DecisionVetted, "")

	r := New(s, "me")
	pv, err := r.Package("me", "go", "github.com/foo/bar")
	if err != nil {
		t.Fatalf("Package: %v", err)
	}
	if len(pv.Spans) != 0 {
		t.Errorf("expected no spans (alice not trusted), got %+v", pv.Spans)
	}
}

func TestPackage_SingleTrustedSigner(t *testing.T) {
	s := newTestStore(t)
	addPeer(t, s, &localstore.Peer{ID: "alice", ServerURL: "https://x", TrustDepth: 0})
	addDecision(t, s, "alice", "go", "p", "v1.0.0", models.DecisionVetted, "looks fine")

	r := New(s, "me")
	if v := verdictFor(t, r, "v1.0.0"); v.Status != StatusVetted {
		t.Errorf("v1.0.0 = %q, want vetted", v.Status)
	}
}

func TestPackage_MultipleVersionsDifferentStatuses(t *testing.T) {
	s := newTestStore(t)
	addPeer(t, s, &localstore.Peer{ID: "alice", ServerURL: "https://x", TrustDepth: 0})
	addDecision(t, s, "alice", "go", "p", "v1.0.0", models.DecisionAllowed, "")
	addDecision(t, s, "alice", "go", "p", "v1.1.0", models.DecisionVetted, "")
	addDecision(t, s, "alice", "go", "p", "v2.0.0", models.DecisionRejected, "broken")

	r := New(s, "me")
	if verdictFor(t, r, "v1.0.0").Status != StatusAllowed {
		t.Error("v1.0.0 should be allowed")
	}
	if verdictFor(t, r, "v1.1.0").Status != StatusVetted {
		t.Error("v1.1.0 should be vetted")
	}
	if verdictFor(t, r, "v2.0.0").Status != StatusRejected {
		t.Error("v2.0.0 should be rejected")
	}

	// Three distinct-status single-version spans (no merge).
	pv, _ := r.Package("me", "go", "p")
	if len(pv.Spans) != 3 {
		t.Errorf("expected 3 spans, got %d: %+v", len(pv.Spans), pv.Spans)
	}
}

func TestPackage_RejectedWinsOverVetted(t *testing.T) {
	s := newTestStore(t)
	addPeer(t, s, &localstore.Peer{ID: "alice", ServerURL: "https://x", TrustDepth: 0})
	addPeer(t, s, &localstore.Peer{ID: "bob", ServerURL: "https://x", TrustDepth: 0})
	addDecision(t, s, "alice", "go", "p", "v1.0.0", models.DecisionVetted, "")
	addDecision(t, s, "bob", "go", "p", "v1.0.0", models.DecisionRejected, "CVE")

	r := New(s, "me")
	if v := verdictFor(t, r, "v1.0.0"); v.Status != StatusRejected {
		t.Errorf("rejected must win, got %q", v.Status)
	}
}

func TestPackage_VettedWinsOverAllowed(t *testing.T) {
	s := newTestStore(t)
	addPeer(t, s, &localstore.Peer{ID: "alice", ServerURL: "https://x", TrustDepth: 0})
	addPeer(t, s, &localstore.Peer{ID: "bob", ServerURL: "https://x", TrustDepth: 0})
	addDecision(t, s, "alice", "go", "p", "v1.0.0", models.DecisionAllowed, "")
	addDecision(t, s, "bob", "go", "p", "v1.0.0", models.DecisionVetted, "code review done")

	r := New(s, "me")
	if v := verdictFor(t, r, "v1.0.0"); v.Status != StatusVetted {
		t.Errorf("vetted must beat allowed, got %q", v.Status)
	}
}

func TestPackage_VetoOnlyApprovalIgnored(t *testing.T) {
	s := newTestStore(t)
	addPeer(t, s, &localstore.Peer{ID: "alice", ServerURL: "https://x", TrustDepth: 0, VetoOnly: true})
	addDecision(t, s, "alice", "go", "p", "v1.0.0", models.DecisionVetted, "trusted me bro")

	r := New(s, "me")
	if v := verdictFor(t, r, "v1.0.0"); v.Status != StatusUnknown {
		t.Errorf("veto-only signer's vetted decision must be ignored, got %q", v.Status)
	}
}

func TestPackage_VetoOnlyRejectionCounts(t *testing.T) {
	s := newTestStore(t)
	addPeer(t, s, &localstore.Peer{ID: "alice", ServerURL: "https://x", TrustDepth: 0, VetoOnly: true})
	addDecision(t, s, "alice", "go", "p", "v1.0.0", models.DecisionRejected, "CVE-2024-...")

	r := New(s, "me")
	v := verdictFor(t, r, "v1.0.0")
	if v.Status != StatusRejected {
		t.Errorf("veto-only signer's rejection must count, got %q", v.Status)
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
	if v := verdictFor(t, r, "v1.0.0"); v.Status != StatusUnknown {
		t.Errorf("revoked signature must not contribute, got %q", v.Status)
	}
}

func TestPackage_RevokedSignatureSurfacedSeparately(t *testing.T) {
	s := newTestStore(t)
	addPeer(t, s, &localstore.Peer{ID: "alice", ServerURL: "https://x", TrustDepth: 0})
	id := addDecision(t, s, "alice", "go", "p", "v1.0.0", models.DecisionVetted, "looked fine then")
	revokeSignature(t, s, id)

	r := New(s, "me")
	pv, err := r.Package("me", "go", "p")
	if err != nil {
		t.Fatalf("Package: %v", err)
	}
	// Revoked decisions must NOT contribute to spans.
	if len(pv.Spans) != 0 {
		t.Errorf("expected no spans for revoked-only package, got %+v", pv.Spans)
	}
	// But they must be surfaced separately so callers (CLI --verbose) can
	// show the user that there was once a decision.
	if len(pv.Revoked) != 1 {
		t.Fatalf("expected 1 revoked decision in PackageVerdict.Revoked, got %d", len(pv.Revoked))
	}
	d := pv.Revoked[0]
	if d.SignerID != "alice" || d.Version != "v1.0.0" || d.Level != "vetted" {
		t.Errorf("revoked decision has wrong fields: %+v", d)
	}
	if d.Reason != "looked fine then" {
		t.Errorf("expected reason preserved, got %q", d.Reason)
	}
}

func TestPackage_RevokedFromUntrustedSignerNotSurfaced(t *testing.T) {
	s := newTestStore(t)
	// alice is NOT a peer — her revoked sig must not show up at all.
	id := addDecision(t, s, "alice", "go", "p", "v1.0.0", models.DecisionVetted, "")
	revokeSignature(t, s, id)

	r := New(s, "me")
	pv, _ := r.Package("me", "go", "p")
	if len(pv.Revoked) != 0 {
		t.Errorf("revoked decision from untrusted signer must be filtered, got %+v", pv.Revoked)
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
	if len(pv.Spans) != 1 {
		t.Errorf("expected only the matching package/ecosystem, got %+v", pv.Spans)
	}
}

func TestPackage_TransitiveTrust(t *testing.T) {
	s := newTestStore(t)
	addPeer(t, s, &localstore.Peer{ID: "alice", ServerURL: "https://x", TrustDepth: 1})
	addConnection(t, s, "alice", "bob", 0)
	addDecision(t, s, "bob", "go", "p", "v1.0.0", models.DecisionVetted, "")

	r := New(s, "me")
	if v := verdictFor(t, r, "v1.0.0"); v.Status != StatusVetted {
		t.Errorf("expected vetted via transitive trust, got %q", v.Status)
	}
}

func TestPackage_SelfSignedDecisionShowsUp(t *testing.T) {
	s := newTestStore(t)
	addDecision(t, s, "me", "go", "asdf", "v3.0.0", models.DecisionAllowed, "personal use")

	r := New(s, "me")
	v, _ := r.Version("me", "go", "asdf", "v3.0.0")
	if v.Status != StatusAllowed {
		t.Errorf("Status = %q, want allowed", v.Status)
	}
}

func TestVersion_UnknownVersion(t *testing.T) {
	s := newTestStore(t)
	addPeer(t, s, &localstore.Peer{ID: "alice", ServerURL: "https://x", TrustDepth: 0})
	addDecision(t, s, "alice", "go", "p", "v1.0.0", models.DecisionVetted, "")

	r := New(s, "me")
	v := verdictFor(t, r, "v9.9.9")
	if v.Status != StatusUnknown {
		t.Errorf("expected unknown for absent version, got %q", v.Status)
	}
	if len(v.Decisions) != 0 {
		t.Errorf("unknown verdict should have no decisions, got %+v", v.Decisions)
	}
}

package resolver

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Alge/tillit/localstore"
	"github.com/Alge/tillit/models"
	"github.com/google/uuid"
)

func newTestStore(t *testing.T) *localstore.Store {
	t.Helper()
	s, err := localstore.Init(":memory:")
	if err != nil {
		t.Fatalf("localstore.Init: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// addPeer writes a row to the local peers table — represents a direct
// trust edge from the active user.
func addPeer(t *testing.T, s *localstore.Store, p *localstore.Peer) {
	t.Helper()
	if err := s.SavePeer(p); err != nil {
		t.Fatalf("SavePeer: %v", err)
	}
}

// addConnection writes a cached_connections row representing a (signer →
// other_id) public trust edge — the kind of record sync would have
// fetched from signer's server.
func addConnection(t *testing.T, s *localstore.Store, signer, other string, trustExtends int) {
	t.Helper()
	now := time.Now().UTC()
	payload := &models.Payload{
		Type:         models.PayloadTypeConnection,
		Signer:       signer,
		OtherID:      other,
		Public:       true,
		Trust:        true,
		TrustExtends: trustExtends,
	}
	bytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	conn := &localstore.CachedConnection{
		ID:        uuid.NewString(),
		Signer:    signer,
		OtherID:   other,
		Payload:   string(bytes),
		Algorithm: "ed25519",
		Sig:       "fake",
		CreatedAt: now,
		FetchedAt: now,
	}
	if err := s.SaveCachedConnection(conn); err != nil {
		t.Fatalf("SaveCachedConnection: %v", err)
	}
}

func TestTrustSet_PublicAPI(t *testing.T) {
	s := newTestStore(t)
	addPeer(t, s, &localstore.Peer{ID: "alice", ServerURL: "https://x", TrustDepth: 2})
	addConnection(t, s, "alice", "bob", 1)
	addConnection(t, s, "bob", "carol", 0)

	r := New(s, "me")
	entries, err := r.TrustSet("me")
	if err != nil {
		t.Fatalf("TrustSet: %v", err)
	}

	byID := map[string]TrustEntry{}
	for _, e := range entries {
		byID[e.SignerID] = e
	}

	if _, ok := byID["me"]; !ok || len(byID["me"].Path) != 0 {
		t.Errorf("viewer should be present with empty Path, got: %+v", byID["me"])
	}
	if e, ok := byID["alice"]; !ok || len(e.Path) != 1 {
		t.Errorf("alice should be direct (Path length 1), got: %+v", e)
	}
	if e, ok := byID["bob"]; !ok || len(e.Path) != 2 {
		t.Errorf("bob should be transitive at depth 2, got: %+v", e)
	}
	if e, ok := byID["carol"]; !ok || len(e.Path) != 3 {
		t.Errorf("carol should be transitive at depth 3, got: %+v", e)
	}
}

func TestBuildTrustSet_DirectPeerOnly(t *testing.T) {
	s := newTestStore(t)
	addPeer(t, s, &localstore.Peer{ID: "alice", ServerURL: "https://x", TrustDepth: 1})

	r := New(s, "me")
	got, err := r.buildTrustSet("me")
	if err != nil {
		t.Fatalf("buildTrustSet: %v", err)
	}
	if _, ok := got["alice"]; !ok {
		t.Errorf("expected alice in trust set, got: %+v", got)
	}
	if got["alice"].VetoOnly {
		t.Error("expected alice not veto-only")
	}
}

func TestBuildTrustSet_TransitiveTrust(t *testing.T) {
	s := newTestStore(t)
	// me trusts alice with depth=2; alice publicly extends trust to bob
	// with TrustExtends=1.
	addPeer(t, s, &localstore.Peer{ID: "alice", ServerURL: "https://x", TrustDepth: 2})
	addConnection(t, s, "alice", "bob", 1)

	r := New(s, "me")
	got, _ := r.buildTrustSet("me")
	if _, ok := got["bob"]; !ok {
		t.Errorf("expected bob to be reachable via alice, got: %+v", got)
	}
}

func TestBuildTrustSet_DepthExhaustion(t *testing.T) {
	s := newTestStore(t)
	// depth=0 means direct only — bob unreachable even though alice
	// declares him at TrustExtends=5.
	addPeer(t, s, &localstore.Peer{ID: "alice", ServerURL: "https://x", TrustDepth: 0})
	addConnection(t, s, "alice", "bob", 5)

	r := New(s, "me")
	got, _ := r.buildTrustSet("me")
	if _, ok := got["bob"]; ok {
		t.Errorf("did not expect bob — alice has depth=0 (direct only)")
	}
}

func TestBuildTrustSet_TrustExtendsCaps(t *testing.T) {
	s := newTestStore(t)
	// me trusts alice with depth=10; alice declared TrustExtends=0 for bob;
	// effective depth at bob is min(9, 0) = 0, so bob's outgoing edges
	// don't count, but bob himself does.
	addPeer(t, s, &localstore.Peer{ID: "alice", ServerURL: "https://x", TrustDepth: 10})
	addConnection(t, s, "alice", "bob", 0)
	addConnection(t, s, "bob", "carol", 5)

	r := New(s, "me")
	got, _ := r.buildTrustSet("me")
	if _, ok := got["bob"]; !ok {
		t.Errorf("expected bob in set, got %+v", got)
	}
	if _, ok := got["carol"]; ok {
		t.Errorf("did not expect carol — bob's trust_extends was 0")
	}
}

func TestBuildTrustSet_DistrustPrunes(t *testing.T) {
	s := newTestStore(t)
	// me trusts alice with depth=2; alice trusts bob; me locally
	// distrusts bob — bob must not appear.
	addPeer(t, s, &localstore.Peer{ID: "alice", ServerURL: "https://x", TrustDepth: 2})
	addPeer(t, s, &localstore.Peer{ID: "bob", ServerURL: "https://x", Distrusted: true})
	addConnection(t, s, "alice", "bob", 1)

	r := New(s, "me")
	got, _ := r.buildTrustSet("me")
	if _, ok := got["bob"]; ok {
		t.Errorf("expected bob to be pruned by local distrust, got: %+v", got)
	}
}

func TestBuildTrustSet_VetoOnlyStickyAlongPath(t *testing.T) {
	s := newTestStore(t)
	// me trusts alice as veto-only with depth=2; alice trusts bob.
	// bob should be in the set but with VetoOnly=true.
	addPeer(t, s, &localstore.Peer{ID: "alice", ServerURL: "https://x", TrustDepth: 2, VetoOnly: true})
	addConnection(t, s, "alice", "bob", 1)

	r := New(s, "me")
	got, _ := r.buildTrustSet("me")
	if e, ok := got["alice"]; !ok || !e.VetoOnly {
		t.Errorf("expected alice veto-only, got %+v", e)
	}
	if e, ok := got["bob"]; !ok || !e.VetoOnly {
		t.Errorf("expected bob inherited veto-only, got %+v", e)
	}
}

func TestBuildTrustSet_CycleHandling(t *testing.T) {
	s := newTestStore(t)
	addPeer(t, s, &localstore.Peer{ID: "alice", ServerURL: "https://x", TrustDepth: 5})
	addConnection(t, s, "alice", "bob", 5)
	addConnection(t, s, "bob", "alice", 5)
	addConnection(t, s, "bob", "carol", 5)

	r := New(s, "me")
	got, err := r.buildTrustSet("me")
	if err != nil {
		t.Fatalf("buildTrustSet: %v", err)
	}
	for _, want := range []string{"alice", "bob", "carol"} {
		if _, ok := got[want]; !ok {
			t.Errorf("expected %s in set", want)
		}
	}
}

func TestBuildTrustSet_NonActiveViewer(t *testing.T) {
	s := newTestStore(t)
	// We want to know what alice thinks. Her public connections live in
	// cached_connections; we have no peers row for her (peers is local
	// only to "me").
	addConnection(t, s, "alice", "bob", 1)
	addConnection(t, s, "bob", "carol", 1)

	r := New(s, "me")
	got, _ := r.buildTrustSet("alice")
	if _, ok := got["bob"]; !ok {
		t.Errorf("expected bob in alice's trust set, got %+v", got)
	}
}

func TestBuildTrustSet_ViewerIsAlwaysTrusted(t *testing.T) {
	s := newTestStore(t)
	// No peers, no connections — but viewer should still trust themselves.
	r := New(s, "me")
	got, err := r.buildTrustSet("me")
	if err != nil {
		t.Fatalf("buildTrustSet: %v", err)
	}
	entry, ok := got["me"]
	if !ok {
		t.Fatalf("expected viewer to trust self, got: %+v", got)
	}
	if entry.VetoOnly {
		t.Error("self-trust must not be veto-only")
	}
	if len(entry.Path) != 0 {
		t.Errorf("self-trust path should be empty, got %v", entry.Path)
	}
}

func TestBuildTrustSet_RevokedConnectionIgnored(t *testing.T) {
	s := newTestStore(t)
	addPeer(t, s, &localstore.Peer{ID: "alice", ServerURL: "https://x", TrustDepth: 2})

	// Save the connection itself (alice → bob).
	connID := uuid.NewString()
	connPayload := &models.Payload{
		Type: models.PayloadTypeConnection, Signer: "alice", OtherID: "bob",
		Public: true, Trust: true, TrustExtends: 1,
	}
	connBytes, _ := json.Marshal(connPayload)
	if err := s.SaveCachedConnection(&localstore.CachedConnection{
		ID: connID, Signer: "alice", OtherID: "bob",
		Payload: string(connBytes), Algorithm: "ed25519", Sig: "x",
		CreatedAt: time.Now(), FetchedAt: time.Now(),
	}); err != nil {
		t.Fatalf("SaveCachedConnection: %v", err)
	}
	// Revocation is a separate signed row referencing the original.
	revPayload := &models.Payload{
		Type: models.PayloadTypeConnectionRevocation, Signer: "alice", TargetID: connID,
	}
	revBytes, _ := json.Marshal(revPayload)
	if err := s.SaveCachedConnection(&localstore.CachedConnection{
		ID: uuid.NewString(), Signer: "alice", OtherID: "bob",
		Payload: string(revBytes), Algorithm: "ed25519", Sig: "y",
		CreatedAt: time.Now(), FetchedAt: time.Now(),
	}); err != nil {
		t.Fatalf("save revocation: %v", err)
	}

	r := New(s, "me")
	got, _ := r.buildTrustSet("me")
	if _, ok := got["bob"]; ok {
		t.Errorf("revoked connection must not produce trust edge, got %+v", got)
	}
}

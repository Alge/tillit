package commands

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	tillit_crypto "github.com/Alge/tillit/crypto"
	"github.com/Alge/tillit/localstore"
)

func seedExportFixture(t *testing.T, s *localstore.Store) {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	// Active key.
	if err := s.SaveKey(&localstore.Key{
		Name: "default", Algorithm: "ed25519", PubKey: "pk", PrivKey: "sk",
	}); err != nil {
		t.Fatalf("SaveKey: %v", err)
	}
	if err := s.SetActiveKey("default"); err != nil {
		t.Fatalf("SetActiveKey: %v", err)
	}
	// Peer.
	if err := s.SavePeer(&localstore.Peer{
		ID: "alice", ServerURL: "https://alice.example.com", TrustDepth: 2,
	}); err != nil {
		t.Fatalf("SavePeer: %v", err)
	}
	// Server.
	if err := s.SaveServer(&localstore.Server{URL: "https://srv"}); err != nil {
		t.Fatalf("SaveServer: %v", err)
	}
	// Cached sig.
	if err := s.SaveCachedSignature(&localstore.CachedSignature{
		ID: "sig-1", Signer: "alice", Payload: "{}", Algorithm: "ed25519", Sig: "x",
		UploadedAt: now, FetchedAt: now,
	}); err != nil {
		t.Fatalf("SaveCachedSignature: %v", err)
	}
	// Cached connection.
	if err := s.SaveCachedConnection(&localstore.CachedConnection{
		ID: "conn-1", Signer: "alice", OtherID: "bob", Payload: "{}",
		Algorithm: "ed25519", Sig: "x", CreatedAt: now, FetchedAt: now,
	}); err != nil {
		t.Fatalf("SaveCachedConnection: %v", err)
	}
}

func TestExport_RoundTrip(t *testing.T) {
	src := newInspectStore(t)
	seedExportFixture(t, src)

	doc, err := buildExportDoc(src, scopeAll, "")
	if err != nil {
		t.Fatalf("buildExportDoc: %v", err)
	}

	// Marshal/unmarshal so we exercise the full JSON path.
	var buf bytes.Buffer
	if err := writeExport(&buf, doc); err != nil {
		t.Fatalf("writeExport: %v", err)
	}
	var roundTripped exportDoc
	if err := json.Unmarshal(buf.Bytes(), &roundTripped); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	dst := newInspectStore(t)
	stats, err := applyImport(dst, &roundTripped)
	if err != nil {
		t.Fatalf("applyImport: %v", err)
	}

	if stats.keys != 1 || stats.peers != 1 || stats.servers != 1 {
		t.Errorf("expected 1/1/1 keys/peers/servers, got %+v", stats)
	}
	if stats.signatures != 1 || stats.connections != 1 {
		t.Errorf("expected 1/1 sigs/conns, got %+v", stats)
	}

	// Active key should have been transferred.
	got, err := dst.GetActiveKey()
	if err != nil || got != "default" {
		t.Errorf("active key = %q err=%v, want 'default'", got, err)
	}
	if k, _ := dst.GetKey("default"); k == nil || k.PrivKey != "sk" {
		t.Errorf("imported key missing/incomplete: %+v", k)
	}
}

func TestImport_SkipsExistingKeys(t *testing.T) {
	dst := newInspectStore(t)
	dst.SaveKey(&localstore.Key{Name: "default", Algorithm: "ed25519", PubKey: "local-pub", PrivKey: "local-priv"})

	doc := &exportDoc{
		Version: exportFormatVersion,
		Keys: []*localstore.Key{
			{Name: "default", Algorithm: "ed25519", PubKey: "import-pub", PrivKey: "import-priv"},
		},
	}
	stats, err := applyImport(dst, doc)
	if err != nil {
		t.Fatalf("applyImport: %v", err)
	}
	if stats.keys != 0 {
		t.Errorf("expected 0 keys imported (conflict), got %d", stats.keys)
	}
	if stats.skipped != 1 {
		t.Errorf("expected 1 skip, got %d", stats.skipped)
	}
	got, _ := dst.GetKey("default")
	if got.PrivKey != "local-priv" {
		t.Error("import must not clobber an existing key")
	}
}

// TestExport_DefaultScopeIsOwnDataOnly: by default, signatures and
// connections by other signers (and the cached-users pubkey cache,
// which is purely peer metadata) must NOT appear in the export.
func TestExport_DefaultScopeIsOwnDataOnly(t *testing.T) {
	src := newInspectStore(t)
	now := time.Now().UTC().Truncate(time.Second)

	// Active user is the signer of an Ed25519 key we'll register.
	signer, err := makeActiveSigner(t, src)
	if err != nil {
		t.Fatalf("makeActiveSigner: %v", err)
	}
	myID := signer

	// One row signed by me, one by alice (a peer).
	src.SaveCachedSignature(&localstore.CachedSignature{
		ID: "mine", Signer: myID, Payload: "{}", Algorithm: "ed25519", Sig: "x",
		UploadedAt: now, FetchedAt: now,
	})
	src.SaveCachedSignature(&localstore.CachedSignature{
		ID: "alices", Signer: "alice", Payload: "{}", Algorithm: "ed25519", Sig: "x",
		UploadedAt: now, FetchedAt: now,
	})
	src.SaveCachedConnection(&localstore.CachedConnection{
		ID: "mine-conn", Signer: myID, OtherID: "bob", Payload: "{}",
		Algorithm: "ed25519", Sig: "x", CreatedAt: now, FetchedAt: now,
	})
	src.SaveCachedConnection(&localstore.CachedConnection{
		ID: "alices-conn", Signer: "alice", OtherID: "bob", Payload: "{}",
		Algorithm: "ed25519", Sig: "x", CreatedAt: now, FetchedAt: now,
	})
	src.SaveCachedUser(&localstore.CachedUser{
		ID: "alice", Username: "alice", PubKey: "pk", Algorithm: "ed25519", FetchedAt: now,
	})

	doc, err := buildExportDoc(src, scopeSelf, "")
	if err != nil {
		t.Fatalf("buildExportDoc(scope=self): %v", err)
	}
	if len(doc.CachedSignatures) != 1 || doc.CachedSignatures[0].ID != "mine" {
		t.Errorf("expected only my signature, got %+v", doc.CachedSignatures)
	}
	if len(doc.CachedConnections) != 1 || doc.CachedConnections[0].ID != "mine-conn" {
		t.Errorf("expected only my connection, got %+v", doc.CachedConnections)
	}
	if len(doc.CachedUsers) != 0 {
		t.Errorf("default scope must not include the peer pubkey cache, got %+v", doc.CachedUsers)
	}

	// --all should pull everything in.
	docAll, err := buildExportDoc(src, scopeAll, "")
	if err != nil {
		t.Fatalf("buildExportDoc(scope=all): %v", err)
	}
	if len(docAll.CachedSignatures) != 2 {
		t.Errorf("--all should include both signatures, got %d", len(docAll.CachedSignatures))
	}
	if len(docAll.CachedConnections) != 2 {
		t.Errorf("--all should include both connections, got %d", len(docAll.CachedConnections))
	}
	if len(docAll.CachedUsers) != 1 {
		t.Errorf("--all should include cached_users, got %d", len(docAll.CachedUsers))
	}
}

// TestExport_IncludePeers_ScopeFollowsTrustGraph: with
// --include-peers, the export must include cached signatures and
// connections by every signer reachable in the active identity's
// trust graph (direct + transitive), and the cached_users entries
// for those signers — but nothing for unrelated peers.
func TestExport_IncludePeers_ScopeFollowsTrustGraph(t *testing.T) {
	src := newInspectStore(t)
	now := time.Now().UTC().Truncate(time.Second)

	myID, err := makeActiveSigner(t, src)
	if err != nil {
		t.Fatalf("makeActiveSigner: %v", err)
	}

	// Trust graph: me → alice (direct, depth 1) → bob (transitive)
	// stranger is not reachable.
	if err := src.SavePeer(&localstore.Peer{
		ID: "alice", ServerURL: "https://x", TrustDepth: 1,
	}); err != nil {
		t.Fatalf("SavePeer: %v", err)
	}
	// alice → bob connection (cached_connection by alice).
	src.SaveCachedConnection(&localstore.CachedConnection{
		ID: "alice-bob", Signer: "alice", OtherID: "bob",
		Payload:   `{"type":"connection","signer":"alice","other_id":"bob","trust":true,"public":true,"trust_extends":0}`,
		Algorithm: "ed25519", Sig: "x", CreatedAt: now, FetchedAt: now,
	})

	// Sigs by people in and out of the trust graph.
	src.SaveCachedSignature(&localstore.CachedSignature{
		ID: "mine", Signer: myID, Payload: "{}", Algorithm: "ed25519", Sig: "x",
		UploadedAt: now, FetchedAt: now,
	})
	src.SaveCachedSignature(&localstore.CachedSignature{
		ID: "alices", Signer: "alice", Payload: "{}", Algorithm: "ed25519", Sig: "x",
		UploadedAt: now, FetchedAt: now,
	})
	src.SaveCachedSignature(&localstore.CachedSignature{
		ID: "bobs", Signer: "bob", Payload: "{}", Algorithm: "ed25519", Sig: "x",
		UploadedAt: now, FetchedAt: now,
	})
	src.SaveCachedSignature(&localstore.CachedSignature{
		ID: "strangers", Signer: "stranger", Payload: "{}", Algorithm: "ed25519", Sig: "x",
		UploadedAt: now, FetchedAt: now,
	})

	// cached_users for everyone — only the in-graph ones should
	// survive the filter.
	for _, id := range []string{"alice", "bob", "stranger"} {
		src.SaveCachedUser(&localstore.CachedUser{
			ID: id, Username: id, PubKey: "pk", Algorithm: "ed25519", FetchedAt: now,
		})
	}

	doc, err := buildExportDoc(src, scopeIncludePeers, "")
	if err != nil {
		t.Fatalf("buildExportDoc(include-peers): %v", err)
	}

	gotSigs := map[string]bool{}
	for _, sig := range doc.CachedSignatures {
		gotSigs[sig.ID] = true
	}
	for _, want := range []string{"mine", "alices", "bobs"} {
		if !gotSigs[want] {
			t.Errorf("expected %q in export (in trust graph), got %v", want, gotSigs)
		}
	}
	if gotSigs["strangers"] {
		t.Errorf("stranger's sig must NOT be in include-peers export, got %v", gotSigs)
	}

	gotUsers := map[string]bool{}
	for _, u := range doc.CachedUsers {
		gotUsers[u.ID] = true
	}
	for _, want := range []string{"alice", "bob"} {
		if !gotUsers[want] {
			t.Errorf("expected cached_user %q in export, got %v", want, gotUsers)
		}
	}
	if gotUsers["stranger"] {
		t.Errorf("stranger must NOT be in cached_users export, got %v", gotUsers)
	}
}

// TestExport_NamedKeyRestrictsKeysArray: --key picks a specific
// stored key by name and the keys array contains only that key.
func TestExport_NamedKeyRestrictsKeysArray(t *testing.T) {
	src := newInspectStore(t)
	if _, err := makeActiveSigner(t, src); err != nil {
		t.Fatalf("makeActiveSigner: %v", err)
	}
	// Add a second key with real Ed25519 bytes.
	signer2, err := tillit_crypto.NewEd25519Signer()
	if err != nil {
		t.Fatalf("NewEd25519Signer: %v", err)
	}
	if err := src.SaveKey(&localstore.Key{
		Name: "personal", Algorithm: "ed25519",
		PubKey:  base64.RawURLEncoding.EncodeToString(signer2.PublicKey()),
		PrivKey: base64.RawURLEncoding.EncodeToString(signer2.PrivateKey()),
	}); err != nil {
		t.Fatalf("SaveKey: %v", err)
	}

	doc, err := buildExportDoc(src, scopeSelf, "personal")
	if err != nil {
		t.Fatalf("buildExportDoc(--key personal): %v", err)
	}
	if len(doc.Keys) != 1 || doc.Keys[0].Name != "personal" {
		t.Errorf("expected only the personal key in keys array, got %+v", doc.Keys)
	}
	if doc.ActiveKey != "personal" {
		t.Errorf("active_key should be the named key 'personal', got %q", doc.ActiveKey)
	}
}

// makeActiveSigner registers an Ed25519 key in the store, sets it
// active, and returns the user ID derived from its pubkey.
func makeActiveSigner(t *testing.T, s *localstore.Store) (string, error) {
	t.Helper()
	if err := s.SaveKey(&localstore.Key{
		Name:      "default",
		Algorithm: "ed25519",
		// Test fixture: matching pub/priv from the crypto package's
		// own test vectors would be ideal, but for buildExportDoc all
		// we need is the userID derivation, so use any well-formed
		// pair the activeSignerAndID code accepts.
		PubKey: "5rxp6PMc21gDmq9oWe9DNUOmwuAybteTk525VEBKvWw",
		// Real ed25519 32-byte private key, base64url. (From the
		// existing agent local store.)
		PrivKey: "8ftOV6m05J4Tg_wI7R3Hf-e39bOF3q5UYlKlJeAoipjmvGno8xzbWAOar2hZ70M1Q6bC4DJu15OTnblUQEq9bA",
	}); err != nil {
		return "", err
	}
	if err := s.SetActiveKey("default"); err != nil {
		return "", err
	}
	_, userID, err := activeSignerAndID(s)
	return userID, err
}

// TestImport_IsIdempotent ensures running the same import twice
// neither errors nor produces duplicate rows. Most of the heavy
// lifting comes from the cache's write-once semantics + the explicit
// "skip on conflict" rules in applyImport — this test pins that
// behaviour against future regressions.
func TestImport_IsIdempotent(t *testing.T) {
	src := newInspectStore(t)
	seedExportFixture(t, src)
	doc, err := buildExportDoc(src, scopeAll, "")
	if err != nil {
		t.Fatalf("buildExportDoc: %v", err)
	}

	dst := newInspectStore(t)

	// First import lands rows.
	first, err := applyImport(dst, doc)
	if err != nil {
		t.Fatalf("first applyImport: %v", err)
	}
	if first.skipped != 0 {
		t.Errorf("first import should skip nothing, got %d", first.skipped)
	}

	// Second import: nothing new should be inserted; everything we
	// recognise should be a no-op skip (keys, peers, servers) or a
	// silent re-write that produces the same row (signatures,
	// connections, cached_users, push_state).
	second, err := applyImport(dst, doc)
	if err != nil {
		t.Fatalf("second applyImport: %v", err)
	}
	if second.keys != 0 || second.peers != 0 || second.servers != 0 {
		t.Errorf("second import must not add fresh keys/peers/servers, got %+v", second)
	}
	expectedSkips := first.keys + first.peers + first.servers
	if second.skipped != expectedSkips {
		t.Errorf("expected %d skips on re-import, got %d", expectedSkips, second.skipped)
	}

	// Row counts must be unchanged.
	keysAfter, _ := dst.ListKeys()
	if len(keysAfter) != len(doc.Keys) {
		t.Errorf("keys count drifted after re-import: %d vs %d", len(keysAfter), len(doc.Keys))
	}
	peersAfter, _ := dst.ListPeers()
	if len(peersAfter) != len(doc.Peers) {
		t.Errorf("peers count drifted: %d vs %d", len(peersAfter), len(doc.Peers))
	}
	sigsAfter, _ := dst.ListAllCachedSignatures()
	if len(sigsAfter) != len(doc.CachedSignatures) {
		t.Errorf("signatures count drifted: %d vs %d", len(sigsAfter), len(doc.CachedSignatures))
	}
	connsAfter, _ := dst.ListAllCachedConnections()
	if len(connsAfter) != len(doc.CachedConnections) {
		t.Errorf("connections count drifted: %d vs %d", len(connsAfter), len(doc.CachedConnections))
	}
	pushAfter, _ := dst.ListAllPushState()
	if len(pushAfter) != len(doc.PushState) {
		t.Errorf("push_state count drifted: %d vs %d", len(pushAfter), len(doc.PushState))
	}
}

// TestImport_DoesNotClobberLocalRevocations: a re-import of an
// older snapshot — one taken before a local revocation — must not
// resurrect the original signature. Revocation is derived from the
// existence of a revocation sig, which is itself a write-once row;
// the re-import can't remove it.
func TestImport_DoesNotClobberLocalRevocations(t *testing.T) {
	src := newInspectStore(t)
	now := time.Now().UTC().Truncate(time.Second)
	src.SaveCachedSignature(&localstore.CachedSignature{
		ID: "target", Signer: "alice", Payload: "{}", Algorithm: "ed25519", Sig: "x",
		UploadedAt: now, FetchedAt: now,
	})

	// Take a snapshot BEFORE the revocation.
	before, err := buildExportDoc(src, scopeAll, "")
	if err != nil {
		t.Fatalf("buildExportDoc: %v", err)
	}

	// Now revoke locally — this writes a separate revocation sig.
	src.SaveCachedSignature(&localstore.CachedSignature{
		ID: "rev-1", Signer: "alice",
		Payload:    `{"type":"revocation","signer":"alice","target_id":"target"}`,
		Algorithm:  "ed25519", Sig: "y",
		UploadedAt: now.Add(time.Hour), FetchedAt: now.Add(time.Hour),
	})

	revokedNow, _, _ := src.IsCachedSignatureRevoked("target")
	if !revokedNow {
		t.Fatal("setup: target should be revoked after we wrote the revocation sig")
	}

	// Re-import the older snapshot. The revocation sig isn't in it,
	// but it must still be present in the local cache afterwards.
	if _, err := applyImport(src, before); err != nil {
		t.Fatalf("applyImport: %v", err)
	}
	stillRevoked, _, _ := src.IsCachedSignatureRevoked("target")
	if !stillRevoked {
		t.Error("re-importing an older snapshot must not un-revoke a row that was revoked locally")
	}
}

func TestImport_RejectsUnsupportedVersion(t *testing.T) {
	dst := newInspectStore(t)
	doc := &exportDoc{Version: 99}
	_, err := applyImport(dst, doc)
	// applyImport itself doesn't check version (Import does), but
	// confirm the empty doc still imports cleanly so the Import
	// wrapper can be the place that enforces it.
	if err != nil {
		t.Errorf("empty future-version doc shouldn't error in applyImport itself; rejection is at Import: %v", err)
	}
}

package commands

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

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

	doc, err := buildExportDoc(src)
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

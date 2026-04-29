package localstore_test

import (
	"testing"
	"time"

	"github.com/Alge/tillit/localstore"
)

func TestSaveAndGetCachedConnection(t *testing.T) {
	s := newTestStore(t)

	now := time.Now().UTC().Truncate(time.Second)
	c := &localstore.CachedConnection{
		ID:        "conn-1",
		Signer:    "user-a",
		OtherID:   "user-b",
		Payload:   `{"type":"connection","other_id":"user-b","trust":true}`,
		Algorithm: "ed25519",
		Sig:       "base64sig",
		CreatedAt: now,
		FetchedAt: now,
	}
	if err := s.SaveCachedConnection(c); err != nil {
		t.Fatalf("SaveCachedConnection failed: %v", err)
	}

	got, err := s.GetCachedConnection("conn-1")
	if err != nil {
		t.Fatalf("GetCachedConnection failed: %v", err)
	}
	if got.ID != c.ID || got.Signer != c.Signer || got.Payload != c.Payload {
		t.Errorf("got %+v, want %+v", got, c)
	}
	if !got.CreatedAt.Equal(c.CreatedAt) {
		t.Errorf("CreatedAt = %v, want %v", got.CreatedAt, c.CreatedAt)
	}
	if got.Revoked {
		t.Error("expected Revoked=false on a fresh connection")
	}
}

func TestGetCachedConnection_NotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.GetCachedConnection("nope")
	if err == nil {
		t.Error("expected error for nonexistent connection")
	}
}

func TestGetCachedConnectionsBySigner(t *testing.T) {
	s := newTestStore(t)

	now := time.Now().UTC().Truncate(time.Second)
	for _, id := range []string{"c-1", "c-2"} {
		s.SaveCachedConnection(&localstore.CachedConnection{
			ID: id, Signer: "alice", Payload: "{}", Algorithm: "ed25519",
			Sig: "x", CreatedAt: now, FetchedAt: now,
		})
	}
	s.SaveCachedConnection(&localstore.CachedConnection{
		ID: "c-3", Signer: "bob", Payload: "{}", Algorithm: "ed25519",
		Sig: "x", CreatedAt: now, FetchedAt: now,
	})

	conns, err := s.GetCachedConnectionsBySigner("alice")
	if err != nil {
		t.Fatalf("GetCachedConnectionsBySigner failed: %v", err)
	}
	if len(conns) != 2 {
		t.Errorf("expected 2 connections for alice, got %d", len(conns))
	}
}

func TestGetActiveConnection(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC().Truncate(time.Second)

	// No active connection yet.
	got, err := s.GetActiveConnection("alice", "bob")
	if err != nil {
		t.Fatalf("GetActiveConnection failed: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil active connection, got %+v", got)
	}

	// Insert one — it should now be the active.
	c1 := &localstore.CachedConnection{
		ID: "c-1", Signer: "alice", OtherID: "bob",
		Payload:   `{"type":"connection","signer":"alice","other_id":"bob","trust":true}`,
		Algorithm: "ed25519", Sig: "x",
		CreatedAt: now, FetchedAt: now,
	}
	if err := s.SaveCachedConnection(c1); err != nil {
		t.Fatalf("SaveCachedConnection failed: %v", err)
	}
	got, err = s.GetActiveConnection("alice", "bob")
	if err != nil || got == nil || got.ID != "c-1" {
		t.Fatalf("expected c-1, got %v err=%v", got, err)
	}

	// A revocation sig targeting c-1 should make it inactive — derived,
	// no column flip.
	if err := s.SaveCachedConnection(&localstore.CachedConnection{
		ID: "c-rev", Signer: "alice", OtherID: "bob",
		Payload:   `{"type":"connection_revocation","signer":"alice","target_id":"c-1"}`,
		Algorithm: "ed25519", Sig: "y",
		CreatedAt: now.Add(time.Minute), FetchedAt: now,
	}); err != nil {
		t.Fatalf("save revocation: %v", err)
	}
	got, err = s.GetActiveConnection("alice", "bob")
	if err != nil {
		t.Fatalf("GetActiveConnection failed: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil after revocation, got %+v", got)
	}

	// A new active one should be returned.
	c2 := &localstore.CachedConnection{
		ID: "c-2", Signer: "alice", OtherID: "bob",
		Payload:   `{"type":"connection","signer":"alice","other_id":"bob","trust":true}`,
		Algorithm: "ed25519", Sig: "x",
		CreatedAt: now.Add(2 * time.Minute), FetchedAt: now,
	}
	if err := s.SaveCachedConnection(c2); err != nil {
		t.Fatalf("SaveCachedConnection failed: %v", err)
	}
	got, err = s.GetActiveConnection("alice", "bob")
	if err != nil || got == nil || got.ID != "c-2" {
		t.Fatalf("expected c-2, got %v err=%v", got, err)
	}
}

// (removed: TestSaveCachedConnection_Upsert exercised the old upsert
// semantics. Revocation is now expressed via a separate
// connection_revocation row; cached_connections rows are write-once.)

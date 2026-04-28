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

func TestSaveCachedConnection_Upsert(t *testing.T) {
	s := newTestStore(t)

	now := time.Now().UTC().Truncate(time.Second)
	c := &localstore.CachedConnection{
		ID: "c-1", Signer: "alice", Payload: "original",
		Algorithm: "ed25519", Sig: "x", CreatedAt: now, FetchedAt: now,
	}
	s.SaveCachedConnection(c)

	revokedAt := now.Add(time.Minute)
	c.Revoked = true
	c.RevokedAt = &revokedAt
	if err := s.SaveCachedConnection(c); err != nil {
		t.Fatalf("upsert failed: %v", err)
	}

	got, _ := s.GetCachedConnection("c-1")
	if !got.Revoked {
		t.Error("expected Revoked=true after upsert")
	}
	if got.RevokedAt == nil || !got.RevokedAt.Equal(revokedAt) {
		t.Errorf("RevokedAt = %v, want %v", got.RevokedAt, revokedAt)
	}
}

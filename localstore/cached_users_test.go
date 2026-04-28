package localstore_test

import (
	"testing"
	"time"

	"github.com/Alge/tillit/localstore"
)

func TestSaveAndGetCachedUser(t *testing.T) {
	s := newTestStore(t)

	now := time.Now().UTC().Truncate(time.Second)
	u := &localstore.CachedUser{
		ID:        "alice-id",
		Username:  "alice",
		PubKey:    "base64key",
		Algorithm: "ed25519",
		FetchedAt: now,
	}
	if err := s.SaveCachedUser(u); err != nil {
		t.Fatalf("SaveCachedUser failed: %v", err)
	}

	got, err := s.GetCachedUser("alice-id")
	if err != nil {
		t.Fatalf("GetCachedUser failed: %v", err)
	}
	if got.ID != u.ID || got.PubKey != u.PubKey || got.Algorithm != u.Algorithm {
		t.Errorf("got %+v, want %+v", got, u)
	}
	if !got.FetchedAt.Equal(u.FetchedAt) {
		t.Errorf("FetchedAt = %v, want %v", got.FetchedAt, u.FetchedAt)
	}
}

func TestGetCachedUser_NotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.GetCachedUser("nope")
	if err == nil {
		t.Error("expected error for nonexistent cached user")
	}
}

func TestSaveCachedUser_Upsert(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC().Truncate(time.Second)

	u := &localstore.CachedUser{ID: "alice-id", Username: "alice", PubKey: "k1", Algorithm: "ed25519", FetchedAt: now}
	s.SaveCachedUser(u)

	u.PubKey = "k2"
	u.FetchedAt = now.Add(time.Hour)
	if err := s.SaveCachedUser(u); err != nil {
		t.Fatalf("upsert failed: %v", err)
	}

	got, _ := s.GetCachedUser("alice-id")
	if got.PubKey != "k2" {
		t.Errorf("expected updated pubkey, got %q", got.PubKey)
	}
}

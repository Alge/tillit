package localstore_test

import (
	"testing"

	"github.com/Alge/tillit/localstore"
)

func newTestStore(t *testing.T) *localstore.Store {
	t.Helper()
	s, err := localstore.Init(":memory:")
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestInit(t *testing.T) {
	// Init must succeed and tables must exist
	newTestStore(t)
}

func TestSaveAndGetKey(t *testing.T) {
	s := newTestStore(t)

	key := &localstore.Key{
		Name:      "work",
		Algorithm: "ed25519",
		PubKey:    "pubkeybase64",
		PrivKey:   "privkeybase64",
	}

	if err := s.SaveKey(key); err != nil {
		t.Fatalf("SaveKey failed: %v", err)
	}

	got, err := s.GetKey("work")
	if err != nil {
		t.Fatalf("GetKey failed: %v", err)
	}

	if got.Name != key.Name || got.Algorithm != key.Algorithm ||
		got.PubKey != key.PubKey || got.PrivKey != key.PrivKey {
		t.Errorf("got %+v, want %+v", got, key)
	}
}

func TestGetKey_NotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.GetKey("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent key")
	}
}

func TestListKeys(t *testing.T) {
	s := newTestStore(t)

	for _, name := range []string{"work", "personal"} {
		if err := s.SaveKey(&localstore.Key{Name: name, Algorithm: "ed25519", PubKey: "pk", PrivKey: "sk"}); err != nil {
			t.Fatalf("SaveKey failed: %v", err)
		}
	}

	keys, err := s.ListKeys()
	if err != nil {
		t.Fatalf("ListKeys failed: %v", err)
	}
	if len(keys) != 2 {
		t.Errorf("expected 2 keys, got %d", len(keys))
	}
}

func TestActiveKey(t *testing.T) {
	s := newTestStore(t)

	s.SaveKey(&localstore.Key{Name: "work", Algorithm: "ed25519", PubKey: "pk", PrivKey: "sk"})

	if err := s.SetActiveKey("work"); err != nil {
		t.Fatalf("SetActiveKey failed: %v", err)
	}

	name, err := s.GetActiveKey()
	if err != nil {
		t.Fatalf("GetActiveKey failed: %v", err)
	}
	if name != "work" {
		t.Errorf("active key = %q, want %q", name, "work")
	}
}

func TestActiveKey_NoActiveKey(t *testing.T) {
	s := newTestStore(t)
	_, err := s.GetActiveKey()
	if err == nil {
		t.Error("expected error when no active key is set")
	}
}

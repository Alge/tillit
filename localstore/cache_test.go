package localstore_test

import (
	"testing"
	"time"

	"github.com/Alge/tillit/localstore"
)

func TestSaveAndGetCachedSignature(t *testing.T) {
	s := newTestStore(t)

	sig := &localstore.CachedSignature{
		ID:         "sig-1",
		Signer:     "user-a",
		Payload:    `{"type":"decision","level":"vetted"}`,
		Algorithm:  "ed25519",
		Sig:        "base64sig",
		UploadedAt: time.Now().UTC().Truncate(time.Second),
		FetchedAt:  time.Now().UTC().Truncate(time.Second),
	}
	if err := s.SaveCachedSignature(sig); err != nil {
		t.Fatalf("SaveCachedSignature failed: %v", err)
	}

	got, err := s.GetCachedSignature("sig-1")
	if err != nil {
		t.Fatalf("GetCachedSignature failed: %v", err)
	}
	if got.ID != sig.ID || got.Signer != sig.Signer || got.Payload != sig.Payload {
		t.Errorf("got %+v, want %+v", got, sig)
	}
	if !got.UploadedAt.Equal(sig.UploadedAt) {
		t.Errorf("UploadedAt = %v, want %v", got.UploadedAt, sig.UploadedAt)
	}
}

func TestGetCachedSignature_NotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.GetCachedSignature("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent cached signature")
	}
}

func TestGetCachedSignaturesBySigner(t *testing.T) {
	s := newTestStore(t)

	now := time.Now().UTC().Truncate(time.Second)
	for _, id := range []string{"sig-1", "sig-2"} {
		s.SaveCachedSignature(&localstore.CachedSignature{
			ID: id, Signer: "user-a", Payload: "{}", Algorithm: "ed25519",
			Sig: "x", UploadedAt: now, FetchedAt: now,
		})
	}
	s.SaveCachedSignature(&localstore.CachedSignature{
		ID: "sig-3", Signer: "user-b", Payload: "{}", Algorithm: "ed25519",
		Sig: "x", UploadedAt: now, FetchedAt: now,
	})

	sigs, err := s.GetCachedSignaturesBySigner("user-a")
	if err != nil {
		t.Fatalf("GetCachedSignaturesBySigner failed: %v", err)
	}
	if len(sigs) != 2 {
		t.Errorf("expected 2 signatures for user-a, got %d", len(sigs))
	}
}

func TestLookupCachedSignature_FullID(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC().Truncate(time.Second)
	sig := &localstore.CachedSignature{
		ID: "a3f9d2c1b8e74f5a", Signer: "alice", Payload: "{}",
		Algorithm: "ed25519", Sig: "x", UploadedAt: now, FetchedAt: now,
	}
	s.SaveCachedSignature(sig)

	got, err := s.LookupCachedSignature("a3f9d2c1b8e74f5a")
	if err != nil {
		t.Fatalf("LookupCachedSignature failed: %v", err)
	}
	if got.ID != "a3f9d2c1b8e74f5a" {
		t.Errorf("got %q, want %q", got.ID, "a3f9d2c1b8e74f5a")
	}
}

func TestLookupCachedSignature_Prefix(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC().Truncate(time.Second)
	s.SaveCachedSignature(&localstore.CachedSignature{
		ID: "a3f9d2c1b8e74f5a", Signer: "alice", Payload: "{}",
		Algorithm: "ed25519", Sig: "x", UploadedAt: now, FetchedAt: now,
	})

	got, err := s.LookupCachedSignature("a3f9d2c1")
	if err != nil {
		t.Fatalf("LookupCachedSignature(prefix) failed: %v", err)
	}
	if got.ID != "a3f9d2c1b8e74f5a" {
		t.Errorf("got %q, want full ID", got.ID)
	}
}

func TestLookupCachedSignature_AmbiguousPrefix(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC().Truncate(time.Second)
	for _, id := range []string{"a3f9d2c1b8e74f5a", "a3f9d2c1b8e74f5b"} {
		s.SaveCachedSignature(&localstore.CachedSignature{
			ID: id, Signer: "alice", Payload: "{}",
			Algorithm: "ed25519", Sig: "x", UploadedAt: now, FetchedAt: now,
		})
	}

	_, err := s.LookupCachedSignature("a3f9d2c1")
	if err == nil {
		t.Fatal("expected ambiguous-prefix error, got nil")
	}
}

func TestLookupCachedSignature_NotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.LookupCachedSignature("nonexistent")
	if err == nil {
		t.Error("expected error for missing prefix")
	}
}

func TestDeleteCachedSignature(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC().Truncate(time.Second)
	s.SaveCachedSignature(&localstore.CachedSignature{
		ID: "sig-1", Signer: "alice", Payload: "{}",
		Algorithm: "ed25519", Sig: "x", UploadedAt: now, FetchedAt: now,
	})

	if err := s.DeleteCachedSignature("sig-1"); err != nil {
		t.Fatalf("DeleteCachedSignature: %v", err)
	}
	if _, err := s.GetCachedSignature("sig-1"); err == nil {
		t.Error("expected signature gone after delete")
	}
}

func TestDeleteCachedSignature_Missing(t *testing.T) {
	s := newTestStore(t)
	if err := s.DeleteCachedSignature("nope"); err == nil {
		t.Error("expected error deleting missing signature")
	}
}

// TestSaveCachedSignature_RowIsWriteOnce locks in that subsequent
// saves of the same id are silent no-ops — the cache stores
// immutable signed artifacts, so the first write of a row wins. (The
// id is a content hash, so any second write with the same id by
// definition has the same payload+sig anyway.)
func TestSaveCachedSignature_RowIsWriteOnce(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC().Truncate(time.Second)

	s.SaveCachedSignature(&localstore.CachedSignature{
		ID: "sig-1", Signer: "alice", Payload: "original",
		Algorithm: "ed25519", Sig: "x",
		UploadedAt: now, FetchedAt: now,
	})
	// Try to clobber with arbitrary changes — should be ignored.
	s.SaveCachedSignature(&localstore.CachedSignature{
		ID: "sig-1", Signer: "mallory", Payload: "tampered",
		Algorithm: "ed25519", Sig: "y",
		UploadedAt: now.Add(time.Hour), FetchedAt: now.Add(time.Hour),
	})

	got, err := s.GetCachedSignature("sig-1")
	if err != nil {
		t.Fatalf("GetCachedSignature: %v", err)
	}
	if got.Payload != "original" || got.Signer != "alice" || got.Sig != "x" {
		t.Errorf("immutability violated, got: %+v", got)
	}
}

// TestIsCachedSignatureRevoked: revocation status is purely derived
// from the existence of a revocation signature targeting the row,
// signed by the same signer (the only one allowed to revoke).
func TestIsCachedSignatureRevoked(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC().Truncate(time.Second)

	s.SaveCachedSignature(&localstore.CachedSignature{
		ID: "target", Signer: "alice",
		Payload:    `{"type":"decision","signer":"alice","ecosystem":"go","package_id":"p","version":"v1","level":"vetted"}`,
		Algorithm:  "ed25519", Sig: "x",
		UploadedAt: now, FetchedAt: now,
	})

	revoked, _, err := s.IsCachedSignatureRevoked("target")
	if err != nil {
		t.Fatalf("IsCachedSignatureRevoked: %v", err)
	}
	if revoked {
		t.Error("expected not-revoked before any revocation sig is recorded")
	}

	revAt := now.Add(time.Hour)
	s.SaveCachedSignature(&localstore.CachedSignature{
		ID: "rev-1", Signer: "alice",
		Payload:    `{"type":"revocation","signer":"alice","target_id":"target"}`,
		Algorithm:  "ed25519", Sig: "y",
		UploadedAt: revAt, FetchedAt: revAt,
	})

	revoked, when, err := s.IsCachedSignatureRevoked("target")
	if err != nil {
		t.Fatalf("IsCachedSignatureRevoked: %v", err)
	}
	if !revoked {
		t.Error("expected revoked after revocation sig recorded")
	}
	if when == nil || !when.Equal(revAt) {
		t.Errorf("expected revoked_at = %v (revocation upload time), got %v", revAt, when)
	}
}

// A revocation by a DIFFERENT signer must NOT cause the target to be
// considered revoked — server enforces that only the signer can
// revoke their own row, and the derived view honours the same rule.
func TestIsCachedSignatureRevoked_RejectsForeignRevocation(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC().Truncate(time.Second)
	s.SaveCachedSignature(&localstore.CachedSignature{
		ID: "target", Signer: "alice",
		Payload: `{"type":"decision","signer":"alice","ecosystem":"go","package_id":"p","version":"v1","level":"vetted"}`,
		Algorithm: "ed25519", Sig: "x", UploadedAt: now, FetchedAt: now,
	})
	// Bob tries to revoke Alice's signature.
	s.SaveCachedSignature(&localstore.CachedSignature{
		ID: "rev-bob", Signer: "bob",
		Payload: `{"type":"revocation","signer":"bob","target_id":"target"}`,
		Algorithm: "ed25519", Sig: "z", UploadedAt: now, FetchedAt: now,
	})
	revoked, _, _ := s.IsCachedSignatureRevoked("target")
	if revoked {
		t.Error("a foreign signer's revocation must not flip the target — only the signer can revoke their own")
	}
}

// (removed: TestSaveCachedSignature_Upsert tested upsert semantics that
// no longer apply — see TestSaveCachedSignature_RowIsWriteOnce.)

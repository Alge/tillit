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

func TestSaveCachedSignature_Upsert(t *testing.T) {
	s := newTestStore(t)

	now := time.Now().UTC().Truncate(time.Second)
	sig := &localstore.CachedSignature{
		ID: "sig-1", Signer: "user-a", Payload: "original",
		Algorithm: "ed25519", Sig: "x", UploadedAt: now, FetchedAt: now,
	}
	s.SaveCachedSignature(sig)

	sig.Payload = "updated"
	if err := s.SaveCachedSignature(sig); err != nil {
		t.Fatalf("upsert failed: %v", err)
	}

	got, _ := s.GetCachedSignature("sig-1")
	if got.Payload != "updated" {
		t.Errorf("expected updated payload, got %q", got.Payload)
	}
}

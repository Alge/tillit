package commands

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/Alge/tillit/localstore"
)

func TestRunDelete_RemovesUnpushed(t *testing.T) {
	s := newInspectStore(t)
	now := time.Now().UTC().Truncate(time.Second)
	s.SaveCachedSignature(&localstore.CachedSignature{
		ID: "abc123def456", Signer: "alice", Payload: "{}",
		Algorithm: "ed25519", Sig: "x", UploadedAt: now, FetchedAt: now,
	})

	var buf bytes.Buffer
	if err := runDelete(s, &buf, "abc123"); err != nil {
		t.Fatalf("runDelete: %v", err)
	}
	if !strings.Contains(buf.String(), "Deleted unpushed signature abc123def456") {
		t.Errorf("expected delete confirmation, got: %q", buf.String())
	}
	if _, err := s.GetCachedSignature("abc123def456"); err == nil {
		t.Error("signature should be gone")
	}
}

func TestRunDelete_RefusesPushed(t *testing.T) {
	s := newInspectStore(t)
	now := time.Now().UTC().Truncate(time.Second)
	s.SaveCachedSignature(&localstore.CachedSignature{
		ID: "pushed1", Signer: "alice", Payload: "{}",
		Algorithm: "ed25519", Sig: "x", UploadedAt: now, FetchedAt: now,
	})
	if err := s.RecordPush("pushed1", localstore.ItemSignature, "https://srv", now); err != nil {
		t.Fatalf("RecordPush: %v", err)
	}

	var buf bytes.Buffer
	err := runDelete(s, &buf, "pushed1")
	if err == nil {
		t.Fatal("expected error for pushed signature, got nil")
	}
	if !strings.Contains(err.Error(), "tillit revoke") {
		t.Errorf("error should mention revoke, got: %v", err)
	}
	// Signature must still exist.
	if _, err := s.GetCachedSignature("pushed1"); err != nil {
		t.Error("pushed signature should NOT have been deleted")
	}
}

func TestRunDelete_NotFound(t *testing.T) {
	s := newInspectStore(t)
	var buf bytes.Buffer
	if err := runDelete(s, &buf, "nope"); err == nil {
		t.Error("expected error for missing signature")
	}
}

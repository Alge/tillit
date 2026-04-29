package commands

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/Alge/tillit/localstore"
)

func newInspectStore(t *testing.T) *localstore.Store {
	t.Helper()
	s, err := localstore.Init(":memory:")
	if err != nil {
		t.Fatalf("localstore.Init failed: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// TestRunInspect_StripsLeadingHash locks in that the leading '#' from
// query output is stripped before lookup, so users can paste an ID
// straight from `query` (when their shell preserves the '#').
func TestRunInspect_StripsLeadingHash(t *testing.T) {
	s := newInspectStore(t)
	now := time.Now().UTC().Truncate(time.Second)
	s.SaveCachedSignature(&localstore.CachedSignature{
		ID:         "a3f9d2c1b8e74f5a",
		Signer:     "alice",
		Payload:    `{"type":"decision","signer":"alice","ecosystem":"go","package_id":"p","version":"v1","level":"vetted"}`,
		Algorithm:  "ed25519",
		Sig:        "x",
		UploadedAt: now,
		FetchedAt:  now,
	})

	var buf bytes.Buffer
	if err := runInspect(s, &buf, "#a3f9d2c1"); err != nil {
		t.Fatalf("runInspect failed: %v", err)
	}
	if !strings.Contains(buf.String(), "a3f9d2c1b8e74f5a") {
		t.Errorf("expected full ID in output, got: %q", buf.String())
	}
}

func TestRunInspect_ShowsRevocationPayload(t *testing.T) {
	s := newInspectStore(t)
	now := time.Now().UTC().Truncate(time.Second)
	revokedAt := now.Add(time.Hour)

	// Original (revoked) decision.
	s.SaveCachedSignature(&localstore.CachedSignature{
		ID:         "target123",
		Signer:     "alice",
		Payload:    `{"type":"decision","signer":"alice","ecosystem":"go","package_id":"p","version":"v1","level":"vetted","reason":"r"}`,
		Algorithm:  "ed25519",
		Sig:        "origsig",
		UploadedAt: now,
		FetchedAt:  now,
		Revoked:    true,
		RevokedAt:  &revokedAt,
	})
	// The revocation signature itself.
	s.SaveCachedSignature(&localstore.CachedSignature{
		ID:         "rev456",
		Signer:     "alice",
		Payload:    `{"type":"revocation","signer":"alice","target_id":"target123"}`,
		Algorithm:  "ed25519",
		Sig:        "revsig",
		UploadedAt: revokedAt,
		FetchedAt:  revokedAt,
	})

	var buf bytes.Buffer
	if err := runInspect(s, &buf, "target123"); err != nil {
		t.Fatalf("runInspect failed: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Revocation:") {
		t.Errorf("expected 'Revocation:' section, got:\n%s", out)
	}
	if !strings.Contains(out, "rev456") {
		t.Errorf("expected revocation ID rev456 in output, got:\n%s", out)
	}
	if !strings.Contains(out, `"type": "revocation"`) {
		t.Errorf("expected revocation payload in output, got:\n%s", out)
	}
	if !strings.Contains(out, `"target_id": "target123"`) {
		t.Errorf("expected target_id in revocation payload, got:\n%s", out)
	}
}

func TestRunInspect_NotRevokedHasNoRevocationSection(t *testing.T) {
	s := newInspectStore(t)
	now := time.Now().UTC().Truncate(time.Second)
	s.SaveCachedSignature(&localstore.CachedSignature{
		ID:         "alive",
		Signer:     "alice",
		Payload:    `{"type":"decision","signer":"alice","ecosystem":"go","package_id":"p","version":"v1","level":"vetted"}`,
		Algorithm:  "ed25519",
		Sig:        "x",
		UploadedAt: now,
		FetchedAt:  now,
	})

	var buf bytes.Buffer
	if err := runInspect(s, &buf, "alive"); err != nil {
		t.Fatalf("runInspect failed: %v", err)
	}
	if strings.Contains(buf.String(), "Revocation:") {
		t.Errorf("non-revoked signature should not show Revocation section, got:\n%s", buf.String())
	}
}

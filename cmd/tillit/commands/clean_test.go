package commands

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/Alge/tillit/localstore"
)

// seedCleanFixture sets up a store with: an active user, one direct
// peer (alice), one stranger signer outside any trust relationship,
// one distrusted peer (mallory), and cached rows for every flavour.
func seedCleanFixture(t *testing.T, s *localstore.Store) (myID string) {
	t.Helper()
	myID, err := makeActiveSigner(t, s)
	if err != nil {
		t.Fatalf("makeActiveSigner: %v", err)
	}
	if err := s.SavePeer(&localstore.Peer{
		ID: "alice", ServerURL: "https://x", TrustDepth: 0,
	}); err != nil {
		t.Fatalf("SavePeer alice: %v", err)
	}
	if err := s.SavePeer(&localstore.Peer{
		ID: "mallory", ServerURL: "https://y", Distrusted: true,
	}); err != nil {
		t.Fatalf("SavePeer mallory: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Second)
	for _, signer := range []string{myID, "alice", "mallory", "stranger"} {
		s.SaveCachedSignature(&localstore.CachedSignature{
			ID: "sig-" + signer, Signer: signer,
			Payload:    `{}`,
			Algorithm:  "ed25519", Sig: "x",
			UploadedAt: now, FetchedAt: now,
		})
		s.SaveCachedConnection(&localstore.CachedConnection{
			ID: "conn-" + signer, Signer: signer, OtherID: "other",
			Payload:    `{}`,
			Algorithm:  "ed25519", Sig: "x",
			CreatedAt: now, FetchedAt: now,
		})
		s.SaveCachedUser(&localstore.CachedUser{
			ID: signer, Username: signer, PubKey: "pk", Algorithm: "ed25519", FetchedAt: now,
		})
	}
	return myID
}

func TestFindPruneCandidates_KeepsTrustSetMembers(t *testing.T) {
	s := newInspectStore(t)
	myID := seedCleanFixture(t, s)

	candidates, err := findPruneCandidates(s, myID)
	if err != nil {
		t.Fatalf("findPruneCandidates: %v", err)
	}

	// Mine and alice are in the trust set — must NOT be pruned.
	for _, sig := range candidates.Signatures {
		if sig.Signer == myID || sig.Signer == "alice" {
			t.Errorf("unexpected prune of trust-set signer %q", sig.Signer)
		}
	}
	for _, c := range candidates.Connections {
		if c.Signer == myID || c.Signer == "alice" {
			t.Errorf("unexpected prune of trust-set connection by %q", c.Signer)
		}
	}
}

func TestFindPruneCandidates_PrunesDistrustedAndStrangers(t *testing.T) {
	s := newInspectStore(t)
	myID := seedCleanFixture(t, s)

	candidates, err := findPruneCandidates(s, myID)
	if err != nil {
		t.Fatalf("findPruneCandidates: %v", err)
	}

	gotSigners := map[string]bool{}
	for _, sig := range candidates.Signatures {
		gotSigners[sig.Signer] = true
	}
	for _, want := range []string{"mallory", "stranger"} {
		if !gotSigners[want] {
			t.Errorf("expected %q in prune candidates, got %v", want, gotSigners)
		}
	}
	if gotSigners[myID] || gotSigners["alice"] {
		t.Errorf("trust-set members must not be candidates, got %v", gotSigners)
	}
}

func TestRunClean_AppliesOnYes(t *testing.T) {
	s := newInspectStore(t)
	seedCleanFixture(t, s)

	defer withConfirmResponses(t, "y")()
	var out bytes.Buffer
	if err := runClean(s, &out); err != nil {
		t.Fatalf("runClean: %v", err)
	}

	// Mallory and stranger should be gone.
	for _, gone := range []string{"mallory", "stranger"} {
		if _, err := s.GetCachedSignature("sig-" + gone); err == nil {
			t.Errorf("sig-%s should have been pruned", gone)
		}
		if _, err := s.GetCachedUser(gone); err == nil {
			t.Errorf("cached_user %s should have been pruned", gone)
		}
	}
	if !strings.Contains(out.String(), "Cleaned:") {
		t.Errorf("expected cleanup confirmation, got: %s", out.String())
	}
}

func TestRunClean_AbortsOnNo(t *testing.T) {
	s := newInspectStore(t)
	seedCleanFixture(t, s)

	defer withConfirmResponses(t, "n")()
	var out bytes.Buffer
	if err := runClean(s, &out); err != nil {
		t.Fatalf("runClean: %v", err)
	}
	// Mallory's data must STILL be present.
	if _, err := s.GetCachedSignature("sig-mallory"); err != nil {
		t.Errorf("sig-mallory must remain after abort, got: %v", err)
	}
	if !strings.Contains(out.String(), "Aborted") {
		t.Errorf("expected abort message, got: %s", out.String())
	}
}

func TestRunClean_AbortsOnEmptyAnswer(t *testing.T) {
	s := newInspectStore(t)
	seedCleanFixture(t, s)

	defer withConfirmResponses(t, "")()
	var out bytes.Buffer
	if err := runClean(s, &out); err != nil {
		t.Fatalf("runClean: %v", err)
	}
	if _, err := s.GetCachedSignature("sig-mallory"); err != nil {
		t.Error("just-Enter should default to abort, but mallory's sig was deleted")
	}
}

func TestRunClean_NothingToClean(t *testing.T) {
	s := newInspectStore(t)
	myID, err := makeActiveSigner(t, s)
	if err != nil {
		t.Fatalf("makeActiveSigner: %v", err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	// Only the active user has anything cached — everything is in
	// the trust set.
	s.SaveCachedSignature(&localstore.CachedSignature{
		ID: "sig", Signer: myID, Payload: "{}", Algorithm: "ed25519", Sig: "x",
		UploadedAt: now, FetchedAt: now,
	})

	// No prompt should fire — confirmReader stays default.
	var out bytes.Buffer
	if err := runClean(s, &out); err != nil {
		t.Fatalf("runClean: %v", err)
	}
	if !strings.Contains(out.String(), "Nothing to clean") {
		t.Errorf("expected 'Nothing to clean' message, got: %s", out.String())
	}
}

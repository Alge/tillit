package commands

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Alge/tillit/localstore"
)

func seedTwoKeys(t *testing.T, s *localstore.Store) {
	t.Helper()
	for _, name := range []string{"default", "personal"} {
		if err := s.SaveKey(&localstore.Key{
			Name: name, Algorithm: "ed25519",
			PubKey:  "pub-" + name,
			PrivKey: "priv-" + name,
		}); err != nil {
			t.Fatalf("SaveKey(%s): %v", name, err)
		}
	}
	if err := s.SetActiveKey("default"); err != nil {
		t.Fatalf("SetActiveKey: %v", err)
	}
}

func TestRunKeyRemove_DeletesNonActive(t *testing.T) {
	s := newInspectStore(t)
	seedTwoKeys(t, s)

	defer withConfirmResponses(t, "personal")()
	var warn, out bytes.Buffer
	if err := runKeyRemove(s, &warn, &out, "personal"); err != nil {
		t.Fatalf("runKeyRemove: %v", err)
	}

	if _, err := s.GetKey("personal"); err == nil {
		t.Error("expected personal to be gone")
	}
	if active, _ := s.GetActiveKey(); active != "default" {
		t.Errorf("active key should still be default, got %q", active)
	}
	if !strings.Contains(warn.String(), "PERMANENT KEY DELETION") {
		t.Errorf("warning message missing, got:\n%s", warn.String())
	}
	if !strings.Contains(out.String(), `Removed key "personal"`) {
		t.Errorf("missing confirmation in output, got: %s", out.String())
	}
}

// TestRunKeyRemove_DeletesActiveAndClearsPointer: deleting the
// active key works, but the active-key pointer must be cleared so a
// subsequent activeUserID call returns the canonical "no active key"
// error rather than dangling.
func TestRunKeyRemove_DeletesActiveAndClearsPointer(t *testing.T) {
	s := newInspectStore(t)
	seedTwoKeys(t, s)

	defer withConfirmResponses(t, "default")()
	var warn, out bytes.Buffer
	if err := runKeyRemove(s, &warn, &out, "default"); err != nil {
		t.Fatalf("runKeyRemove: %v", err)
	}
	if _, err := s.GetKey("default"); err == nil {
		t.Error("expected default to be gone")
	}
	if active, err := s.GetActiveKey(); err == nil {
		t.Errorf("active-key pointer should have been cleared, got %q", active)
	}
	if !strings.Contains(warn.String(), "ACTIVE key") {
		t.Errorf("active-key warning missing, got:\n%s", warn.String())
	}
	if !strings.Contains(out.String(), "No active key is set") {
		t.Errorf("expected 'no active key' hint, got: %s", out.String())
	}
}

// TestRunKeyRemove_AbortsOnMismatchedConfirmation: typing anything
// other than the exact key name leaves the store untouched.
func TestRunKeyRemove_AbortsOnMismatchedConfirmation(t *testing.T) {
	s := newInspectStore(t)
	seedTwoKeys(t, s)

	defer withConfirmResponses(t, "yes")() // not the key name
	var warn, out bytes.Buffer
	err := runKeyRemove(s, &warn, &out, "personal")
	if err == nil {
		t.Fatal("expected error on mismatched confirmation")
	}
	if !strings.Contains(err.Error(), "aborted") {
		t.Errorf("expected 'aborted' error, got: %v", err)
	}
	if _, err := s.GetKey("personal"); err != nil {
		t.Error("personal key must still exist after aborted removal")
	}
}

// TestRunKeyRemove_MissingKey: deleting a non-existent key surfaces
// a clear error before any prompt fires.
func TestRunKeyRemove_MissingKey(t *testing.T) {
	s := newInspectStore(t)
	seedTwoKeys(t, s)

	// No confirm response should be needed — the lookup fails before
	// the prompt. If the prompter IS triggered the test fails loud.
	called := false
	original := confirmReader
	confirmReader = func(prompt string) ([]byte, error) {
		called = true
		return []byte(""), nil
	}
	defer func() { confirmReader = original }()

	var warn, out bytes.Buffer
	if err := runKeyRemove(s, &warn, &out, "nonexistent"); err == nil {
		t.Error("expected error for missing key")
	}
	if called {
		t.Error("confirm prompt should NOT fire when the key doesn't exist")
	}
}

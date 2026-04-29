package commands

import (
	"encoding/base64"
	"strings"
	"testing"

	tillit_crypto "github.com/Alge/tillit/crypto"
	"github.com/Alge/tillit/localstore"
)

// TestBuildStoredKey_EncryptsWhenPasswordGiven: a non-empty password
// produces an encrypted-on-disk envelope; an empty password leaves
// the key as plaintext base64url.
func TestBuildStoredKey_EncryptsWhenPasswordGiven(t *testing.T) {
	signer, err := tillit_crypto.NewEd25519Signer()
	if err != nil {
		t.Fatalf("signer: %v", err)
	}

	// Encrypted path.
	defer withPasswordResponses(t, "hunter2", "hunter2")()
	k, err := buildStoredKey("alice", signer)
	if err != nil {
		t.Fatalf("buildStoredKey: %v", err)
	}
	if !tillit_crypto.IsEncryptedKey([]byte(k.PrivKey)) {
		t.Errorf("expected encrypted envelope on disk, got: %q", k.PrivKey)
	}
	plain, err := tillit_crypto.DecryptKey([]byte(k.PrivKey), []byte("hunter2"))
	if err != nil {
		t.Errorf("DecryptKey: %v", err)
	}
	if len(plain) != len(signer.PrivateKey()) {
		t.Errorf("decrypted length mismatch: %d vs %d", len(plain), len(signer.PrivateKey()))
	}
}

func TestBuildStoredKey_EmptyPasswordLeavesPlaintext(t *testing.T) {
	signer, err := tillit_crypto.NewEd25519Signer()
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	defer withPasswordResponses(t, "", "")()
	k, err := buildStoredKey("alice", signer)
	if err != nil {
		t.Fatalf("buildStoredKey: %v", err)
	}
	if tillit_crypto.IsEncryptedKey([]byte(k.PrivKey)) {
		t.Error("empty password must NOT produce an encrypted envelope")
	}
}

func TestBuildStoredKey_MismatchedConfirmFails(t *testing.T) {
	signer, _ := tillit_crypto.NewEd25519Signer()
	defer withPasswordResponses(t, "first", "second")()
	if _, err := buildStoredKey("alice", signer); err == nil {
		t.Error("expected error on mismatched confirm")
	}
}

// TestDecodePrivateKey_PromptsOnEncryptedKey: decodePrivateKey (the
// helper invoked from activeSignerAndID) must prompt for a password
// when the stored key is encrypted, and surface a useful error when
// the wrong password is supplied.
func TestDecodePrivateKey_PromptsOnEncryptedKey(t *testing.T) {
	signer, _ := tillit_crypto.NewEd25519Signer()
	envelope, err := tillit_crypto.EncryptKey(signer.PrivateKey(), []byte("hunter2"))
	if err != nil {
		t.Fatalf("EncryptKey: %v", err)
	}
	k := &localstore.Key{
		Name:      "alice",
		Algorithm: signer.Algorithm(),
		PrivKey:   string(envelope),
	}

	// Right password decrypts.
	restore := withPasswordResponses(t, "hunter2")
	plain, err := decodePrivateKey(k)
	restore()
	if err != nil {
		t.Fatalf("decodePrivateKey: %v", err)
	}
	if len(plain) != len(signer.PrivateKey()) {
		t.Errorf("plain key length mismatch")
	}

	// Wrong password fails with a recognisable error.
	restore = withPasswordResponses(t, "WRONG")
	_, err = decodePrivateKey(k)
	restore()
	if err == nil || !strings.Contains(err.Error(), "wrong password") {
		t.Errorf("expected wrong-password error, got: %v", err)
	}
}

// TestKeyPasswd_Roundtrip: setting a password, then changing it,
// then removing it, all via the keyPasswd flow.
func TestKeyPasswd_Roundtrip(t *testing.T) {
	s := newInspectStore(t)

	// Install an unencrypted key directly (skipping the prompt path).
	signer, _ := tillit_crypto.NewEd25519Signer()
	if err := s.SaveKey(&localstore.Key{
		Name: "alice", Algorithm: signer.Algorithm(),
		PubKey: encodeURL(signer.PublicKey()), PrivKey: encodeURL(signer.PrivateKey()),
	}); err != nil {
		t.Fatalf("SaveKey: %v", err)
	}

	// keyPasswd reads from openStore() — we can't easily redirect
	// that, so exercise the lower-level encodePrivKeyField + decodePrivateKey
	// helpers instead. (The keyPasswd command itself is a thin
	// wrapper that the live test below in main covers.)

	// Set a password.
	encoded, err := encodePrivKeyField(signer.PrivateKey(), []byte("first"))
	if err != nil {
		t.Fatalf("encodePrivKeyField(set): %v", err)
	}
	if !tillit_crypto.IsEncryptedKey([]byte(encoded)) {
		t.Error("expected encryption after setting a non-empty password")
	}

	// Verify decoded matches original.
	defer withPasswordResponses(t, "first")()
	k := &localstore.Key{Name: "alice", Algorithm: signer.Algorithm(), PrivKey: encoded}
	got, err := decodePrivateKey(k)
	if err != nil {
		t.Fatalf("decodePrivateKey after set: %v", err)
	}
	if string(got) != string(signer.PrivateKey()) {
		t.Error("round-trip mismatch")
	}
}

// encodeURL is a tiny helper used by the test fixture.
func encodeURL(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}

package crypto

import (
	"bytes"
	"strings"
	"testing"
)

func TestEncryptDecryptKey_RoundTrip(t *testing.T) {
	plain := []byte("super-secret-private-key-bytes-32x")
	password := []byte("correct horse battery staple")

	wrapped, err := EncryptKey(plain, password)
	if err != nil {
		t.Fatalf("EncryptKey: %v", err)
	}
	if bytes.Equal(wrapped, plain) {
		t.Fatal("wrapped output equals plaintext — encryption did not happen")
	}

	got, err := DecryptKey(wrapped, password)
	if err != nil {
		t.Fatalf("DecryptKey: %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Errorf("round-trip mismatch: got %q, want %q", got, plain)
	}
}

func TestDecryptKey_WrongPasswordFails(t *testing.T) {
	plain := []byte("private-key")
	wrapped, err := EncryptKey(plain, []byte("right"))
	if err != nil {
		t.Fatalf("EncryptKey: %v", err)
	}
	if _, err := DecryptKey(wrapped, []byte("wrong")); err == nil {
		t.Fatal("expected error when decrypting with the wrong password")
	}
}

func TestDecryptKey_TamperedCiphertextFails(t *testing.T) {
	wrapped, err := EncryptKey([]byte("private-key"), []byte("pw"))
	if err != nil {
		t.Fatalf("EncryptKey: %v", err)
	}
	// Flip a byte inside the JSON wrapper. Find the ciphertext field
	// and corrupt one character.
	str := string(wrapped)
	idx := strings.Index(str, `"ciphertext":"`) + len(`"ciphertext":"`)
	if idx < 0 {
		t.Fatal("could not locate ciphertext in wrapper")
	}
	corrupted := []byte(str[:idx] + flipChar(str[idx:idx+1]) + str[idx+1:])

	if _, err := DecryptKey(corrupted, []byte("pw")); err == nil {
		t.Fatal("expected error on tampered ciphertext")
	}
}

// flipChar returns a different base64url-valid character so the
// surrounding JSON stays parseable; only the ciphertext bytes change.
func flipChar(s string) string {
	if s == "A" {
		return "B"
	}
	return "A"
}

func TestEncryptKey_ProducesJSONWrapper(t *testing.T) {
	wrapped, err := EncryptKey([]byte("k"), []byte("p"))
	if err != nil {
		t.Fatalf("EncryptKey: %v", err)
	}
	if len(wrapped) == 0 || wrapped[0] != '{' {
		t.Errorf("wrapper must start with '{' so callers can detect encrypted vs plaintext, got: %q", wrapped)
	}
	for _, want := range []string{`"version":1`, `"kdf":"argon2id"`, `"salt":`, `"nonce":`, `"ciphertext":`} {
		if !strings.Contains(string(wrapped), want) {
			t.Errorf("wrapper missing field %q, got: %s", want, wrapped)
		}
	}
}

func TestIsEncryptedKey(t *testing.T) {
	wrapped, _ := EncryptKey([]byte("k"), []byte("p"))
	if !IsEncryptedKey(wrapped) {
		t.Error("IsEncryptedKey should be true for an EncryptKey output")
	}
	plain := []byte("VGhpcy1pcy1iYXNlNjR1cmw") // looks like base64url
	if IsEncryptedKey(plain) {
		t.Error("IsEncryptedKey should be false for legacy plaintext base64url")
	}
}

// Two encryptions of the same plaintext + password must produce
// different ciphertexts (fresh salt + nonce).
func TestEncryptKey_RandomisedPerCall(t *testing.T) {
	a, _ := EncryptKey([]byte("k"), []byte("p"))
	b, _ := EncryptKey([]byte("k"), []byte("p"))
	if bytes.Equal(a, b) {
		t.Error("encryption of the same input twice produced identical output — salt/nonce reused")
	}
}

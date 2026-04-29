package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"golang.org/x/crypto/argon2"
)

// keyEnvelopeVersion identifies the encryption-at-rest format. Bump
// it whenever the algorithms or parameters change in a way old keys
// can't be read by the new code (or vice versa).
const keyEnvelopeVersion = 1

// argon2 parameters for password-based key derivation. Values follow
// OWASP's Password Storage Cheat Sheet recommendations for Argon2id.
// Parameters are stored alongside the ciphertext so we can raise
// these defaults later without invalidating older files.
const (
	argonMemoryKiB  = 64 * 1024 // 64 MiB
	argonIterations = 3
	argonParallel   = 2
	argonKeyLen     = 32 // AES-256
	saltLen         = 16
)

// keyEnvelope is the on-disk format produced by EncryptKey. The
// leading '{' character is what tells callers (LoadKey at startup,
// Import on read-in) that the value is encrypted rather than the
// legacy raw base64url-encoded plaintext.
type keyEnvelope struct {
	Version    int        `json:"version"`
	KDF        string     `json:"kdf"`
	KDFParams  argonParms `json:"kdf_params"`
	AEAD       string     `json:"aead"`
	Salt       string     `json:"salt"`
	Nonce      string     `json:"nonce"`
	Ciphertext string     `json:"ciphertext"`
}

type argonParms struct {
	M uint32 `json:"m"` // memory in KiB
	T uint32 `json:"t"` // iterations
	P uint8  `json:"p"` // parallelism
}

// EncryptKey wraps a private-key byte string in a password-derived
// AES-256-GCM envelope. The output is JSON-encoded UTF-8 bytes; pass
// it back to DecryptKey with the same password to recover the
// original. A fresh random salt and nonce are generated each call.
func EncryptKey(plain, password []byte) ([]byte, error) {
	if len(password) == 0 {
		return nil, fmt.Errorf("password must not be empty")
	}
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("salt: %w", err)
	}
	dk := argon2.IDKey(password, salt, argonIterations, argonMemoryKiB, argonParallel, argonKeyLen)

	block, err := aes.NewCipher(dk)
	if err != nil {
		return nil, fmt.Errorf("aes: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("nonce: %w", err)
	}
	ct := gcm.Seal(nil, nonce, plain, nil)

	env := keyEnvelope{
		Version: keyEnvelopeVersion,
		KDF:     "argon2id",
		KDFParams: argonParms{
			M: argonMemoryKiB,
			T: argonIterations,
			P: argonParallel,
		},
		AEAD:       "aes-256-gcm",
		Salt:       base64.RawURLEncoding.EncodeToString(salt),
		Nonce:      base64.RawURLEncoding.EncodeToString(nonce),
		Ciphertext: base64.RawURLEncoding.EncodeToString(ct),
	}
	return json.Marshal(env)
}

// DecryptKey reverses EncryptKey. Returns an error if the password
// is wrong, the envelope is tampered with, the format is unsupported,
// or the input isn't an encrypted envelope at all.
func DecryptKey(envelope, password []byte) ([]byte, error) {
	var env keyEnvelope
	if err := json.Unmarshal(envelope, &env); err != nil {
		return nil, fmt.Errorf("not a key envelope: %w", err)
	}
	if env.Version != keyEnvelopeVersion {
		return nil, fmt.Errorf("unsupported envelope version %d", env.Version)
	}
	if env.KDF != "argon2id" {
		return nil, fmt.Errorf("unsupported KDF %q", env.KDF)
	}
	if env.AEAD != "aes-256-gcm" {
		return nil, fmt.Errorf("unsupported AEAD %q", env.AEAD)
	}
	salt, err := base64.RawURLEncoding.DecodeString(env.Salt)
	if err != nil {
		return nil, fmt.Errorf("salt decode: %w", err)
	}
	nonce, err := base64.RawURLEncoding.DecodeString(env.Nonce)
	if err != nil {
		return nil, fmt.Errorf("nonce decode: %w", err)
	}
	ct, err := base64.RawURLEncoding.DecodeString(env.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("ciphertext decode: %w", err)
	}

	dk := argon2.IDKey(
		password, salt,
		env.KDFParams.T, env.KDFParams.M, env.KDFParams.P,
		argonKeyLen,
	)
	block, err := aes.NewCipher(dk)
	if err != nil {
		return nil, fmt.Errorf("aes: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}
	plain, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		// GCM authentication failure: most often a wrong password,
		// but could also be ciphertext tampering. Surface as a
		// password error since that's the common case and confusing
		// users with "tamper detected" on every typo is worse than
		// the rare opposite.
		return nil, fmt.Errorf("decrypt failed (wrong password or corrupted key)")
	}
	return plain, nil
}

// IsEncryptedKey returns true when raw is a passphrase-protected
// envelope produced by EncryptKey, false when it's legacy plaintext
// (base64url-encoded private key bytes). Detection is purely the
// leading byte: a JSON object opens with '{', and base64url alphabet
// never produces that character.
func IsEncryptedKey(raw []byte) bool {
	for _, b := range raw {
		if b == ' ' || b == '\t' || b == '\n' || b == '\r' {
			continue
		}
		return b == '{'
	}
	return false
}

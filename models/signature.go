package models

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"time"
)

type Signature struct {
	ID         string     `json:"id"`
	Signer     string     `json:"signer"`
	Payload    string     `json:"payload"`
	Algorithm  string     `json:"algorithm"`
	Sig        string     `json:"sig"`
	UploadedAt time.Time  `json:"uploaded_at"`
	Revoked    bool       `json:"revoked,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
}

// SignatureID returns the canonical, content-addressed identifier for a
// signature: sha256 over a length-prefixed framing of the payload and
// raw signature bytes. Length-prefixing prevents concat ambiguity (so
// payload="ab"+sig="cd" cannot collide with payload="abc"+sig="d").
//
// The same payload+sig pair always produces the same ID, regardless of
// which peer first stored it — making sync deduplication trivial and
// the ID self-authenticating.
func SignatureID(payload, sig string) string {
	h := sha256.New()
	var lenBuf [8]byte
	binary.BigEndian.PutUint64(lenBuf[:], uint64(len(payload)))
	h.Write(lenBuf[:])
	h.Write([]byte(payload))
	binary.BigEndian.PutUint64(lenBuf[:], uint64(len(sig)))
	h.Write(lenBuf[:])
	h.Write([]byte(sig))
	return hex.EncodeToString(h.Sum(nil))
}

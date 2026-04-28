package models

import "time"

type Signature struct {
	ID        string    `json:"id"`
	Signer    string    `json:"signer"`
	Payload   string    `json:"payload"`
	Algorithm string    `json:"algorithm"`
	Sig       string    `json:"sig"`
	UploadedAt time.Time `json:"uploaded_at"`
	Revoked   bool      `json:"revoked,omitempty"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
}

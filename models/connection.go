package models

import (
	"fmt"
	"time"
)

type Connection struct {
	ID           string     `json:"id"`
	Owner        string     `json:"owner"`
	OtherID      string     `json:"other_id"`
	Public       bool       `json:"public"`
	Trust        bool       `json:"trust"`
	TrustExtends int        `json:"trust_extends"`
	Payload      string     `json:"payload"`
	Algorithm    string     `json:"algorithm"`
	Sig          string     `json:"sig"`
	CreatedAt    time.Time  `json:"created_at"`
	Revoked      bool       `json:"revoked,omitempty"`
	RevokedAt    *time.Time `json:"revoked_at,omitempty"`
}

func (c Connection) String() string {
	return fmt.Sprintf("Connection. Owner: %s, Other: %s", c.Owner, c.OtherID)
}

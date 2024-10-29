package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Signature represents a user's sign-off on a specific package version.
type Signature struct {
	gorm.Model

	ID        string    `json:"id" gorm:"primaryKey"`
	Owner     string    `json:"owner"`
	KeyID     string    `json:"key_id" gorm:"foreignKey: "`
	PackageID int       `json:"package_id"`
	Signature string    `json:"signature"`
	SignedAt  time.Time `json:"signed_at"`
	Revoked   bool      `json:"revoked,omitempty`
	RevokedAt time.Time `json:"revoked_at,omitempty`
}

func (s *Signature) New() (*Signature, error) {

	if id, err := uuid.NewRandom(); err != nil {
		return s, err
	} else {
		s.ID = id.String()
	}

	return s, nil
}

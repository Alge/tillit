package models

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// User represents a user with an ID, username, public key, and creation timestamp.
type Connection struct {
	gorm.Model
	ID           string    `json:"id" gorm:"primaryKey"`
	Owner        string    `json:"owner" gorm:"uniqueIndex:idx_owner_otherid"`
	OtherID      string    `json:"other_id" gorm:"uniqueIndex:idx_owner_otherid"`
	Public       bool      `json:"public"`        // Can others see this connection?
	Trust        bool      `json:"trust"`         // Do you trust this user?
	TrustExtends int       `json:"trust_extends"` // How far out do you trust people this person trusts
	CreatedAt    time.Time `json:"created_at"`
}

func (c Connection) String() string {
	return fmt.Sprintf("Connection. User1 (owner): %s, User2 (Other): %s", c.Owner, c.OtherID)
}

func NewConnection() (*Connection, error) {
	c := &Connection{}

	if id, err := uuid.NewRandom(); err != nil {
		return c, err
	} else {
		c.ID = id.String()
	}

	return c, nil
}

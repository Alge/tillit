package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// User represents a user with an ID, username, public key, and creation timestamp.
type User struct {
	gorm.Model
	ID        string    `json:"id" gorm:"primaryKey`
	Username  string    `json:"username"`
	PubKeys   []PubKey  `json:"pubkeys" gorm:"foreignKey:Owner;references:ID"`
	CreatedAt time.Time `json:"created_at"`
}

func (u *User) New() (*User, error) {

	if id, err := uuid.NewRandom(); err != nil {
		return u, err
	} else {
		u.ID = id.String()
	}

	u.CreatedAt = time.Now().UTC()
	return u, nil
}

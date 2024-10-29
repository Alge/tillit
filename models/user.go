package models

import (
	"fmt"
	//"github.com/google/uuid"
	"gorm.io/gorm"
)

// User represents a user with an ID, username, public key, and creation timestamp.

type User struct {
  gorm.Model
	ID          string       `json:"id" gorm:"primaryKey"`
	Username    string       `json:"username"`
	PubKeys     []PubKey     `json:"pubkeys" gorm:"foreignKey:Owner;references:ID"`
	Connections []Connection `json:"connections" gorm:"foreignKey:Owner;references:ID"`
}

/*
type User struct {
	ID          string       `json:"id" gorm:"type:uuid;primaryKey;default:uuid_generate_v4().String()"`
	Username    string       `json:"username"`
	PubKeys     []PubKey     `json:"pubkeys" gorm:"foreignKey:Owner;references:ID"`
	Connections []Connection `json:"connections" gorm:"foreignKey:Owner;references:ID"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
}
*/

func NewUser() (*User, error) {

	u := &User{}

  /*
	if id, err := uuid.NewRandom(); err != nil {
		return u, err
	} else {
		u.ID = id.String()
	}
  */
	return u, nil
}

func (u User) String() string {
  return fmt.Sprintf("User Object. ID: %s, Username: %s, Connections: %d", u.ID, u.Username, len(u.Connections))
}

func (u *User) Connect(other *User, public bool, trust bool, trustExtends int) (c *Connection, err error) {
	c, err = NewConnection()
	if err != nil {
		return
	}

	c.Owner = u.ID
	c.OtherID = other.ID
	c.Public = public
	c.Trust = trust
	c.TrustExtends = trustExtends

	return
}

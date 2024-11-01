package models

import (
	"fmt"

	"github.com/google/uuid"
)

// User represents a user with an ID, username, public key, and creation timestamp.

type User struct {
	ID          string       `json:"id"`
	Username    string       `json:"username"`
	PubKeys     []PubKey     `json:"pubkeys"`
	Connections []Connection `json:"connections"`
}

func NewUser(username string) (*User, error) {

	u := &User{}
	u.Username = username

	if id, err := uuid.NewRandom(); err != nil {
		return u, err
	} else {
		u.ID = id.String()
	}

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

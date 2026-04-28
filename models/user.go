package models

import "fmt"

type User struct {
	ID          string       `json:"id"`
	Username    string       `json:"username"`
	PubKey      string       `json:"public_key"`
	Connections []Connection `json:"connections,omitempty"`
	IsAdmin     bool         `json:"is_admin,omitempty"`
}

func (u User) String() string {
	return fmt.Sprintf("User Object. ID: %s, Username: %s, Connections: %d", u.ID, u.Username, len(u.Connections))
}

func (u *User) Connect(other *User, public bool, trust bool, delegate bool, trustExtends int) *Connection {
	return &Connection{
		Owner:        u.ID,
		OtherID:      other.ID,
		Public:       public,
		Trust:        trust,
		Delegate:     delegate,
		TrustExtends: trustExtends,
	}
}

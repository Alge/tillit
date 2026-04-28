package models

import "fmt"

type Connection struct {
	ID           string `json:"id"`
	Owner        string `json:"owner"`
	OtherID      string `json:"other_id"`
	Public       bool   `json:"public"`
	Trust        bool   `json:"trust"`
	TrustExtends int    `json:"trust_extends"`
}

func (c Connection) String() string {
	return fmt.Sprintf("Connection. Owner: %s, Other: %s", c.Owner, c.OtherID)
}

package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PubKey struct {
	gorm.Model
	ID      string    `json:"id" gorm:"primaryKey`
	Owner   string    `json:"owner"`
	Created time.Time `json:"created"`
	Key     string    `json:"key"`
}

func NewPubKey() (*PubKey, error) {

	p := &PubKey{}

	if id, err := uuid.NewRandom(); err != nil {
		return p, err
	} else {
		p.ID = id.String()
	}

	return p, nil
}

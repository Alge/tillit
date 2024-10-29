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

func (p *PubKey) New() (*PubKey, error) {

	if id, err := uuid.NewRandom(); err != nil {
		return p, err
	} else {
		p.ID = id.String()
	}

	p.CreatedAt = time.Now().UTC()
	return p, nil
}

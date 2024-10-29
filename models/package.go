// models/package.go
package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Package represents a software package with a name, version, and creation timestamp.
type Package struct {
	gorm.Model
	ID        string    `json:"id"`         // Unique identifier for the package
	Name      string    `json:"name"`       // Name of the software package
	Version   string    `json:"version"`    // Version of the software package
	Hash      string    `json:"hash"`       // Version of the software package
	CreatedAt time.Time `json:"created_at"` // Timestamp of when the package was created
}

func NewPackage(name string, version string, hash string) (*Package, error) {
	p := &Package{}

	p.Name = name
	p.Version = version
	p.Hash = hash

	if id, err := uuid.NewRandom(); err != nil {
		return p, err
	} else {
		p.ID = id.String()
	}

	return p, nil
}

func (pgk Package) GetHashAlgorithm() string {
	return "sha256"
}

func (pkg Package) getVersionList() []string {
	parts := []string{""}
	return parts
}

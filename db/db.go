package db

import (
	_ "fmt"

	"errors"

	"gorm.io/gorm"

	"github.com/Alge/tillit/models"
)

var DB *gorm.DB
var initialized bool

func Init(dialector gorm.Dialector, opts ...gorm.Option) error {

	database, err := gorm.Open(dialector, opts...)
	if err != nil {
		return err
	}

	// Automigrate all model schemas. TODO: Place this under a flag in init

	if err = database.AutoMigrate(&models.User{}); err != nil {
		return err
	}
	if err = database.AutoMigrate(&models.PubKey{}); err != nil {
		return err
	}
	if err = database.AutoMigrate(&models.Signature{}); err != nil {
		return err
	}
	if err = database.AutoMigrate(&models.Package{}); err != nil {
		return err
	}
	if err = database.AutoMigrate(&models.Connection{}); err != nil {
		return err
	}

	initialized = true

	DB = database

	return nil
}

func GetDB() (*gorm.DB, error) {
	if !initialized {
		return DB, errors.New("Database not initialized yet")
	}

	return DB, nil
}

package db

import (
	_ "fmt"

	"errors"

	"gorm.io/gorm"

	"github.com/Alge/tillit/models"
)

var db *gorm.DB
var initialized bool

func Init(dialector gorm.Dialector, opts ...gorm.Option) (*gorm.DB, error) {

	database, err := gorm.Open(dialector, opts...)
	if err != nil {
		return database, err
	}

	// Automigrate all model schemas. TODO: Place this under a flag in init

	if err = database.AutoMigrate(&models.User{}); err != nil {
		return database, err
	}
	if err = database.AutoMigrate(&models.PubKey{}); err != nil {
		return database, err
	}
	if err = database.AutoMigrate(&models.Signature{}); err != nil {
		return database, err
	}
	if err = database.AutoMigrate(&models.Package{}); err != nil {
		return database, err
	}

	initialized = true

	return database, nil
}

func GetDB() (*gorm.DB, error) {
	if !initialized {
		return db, errors.New("Database not initialized yet")
	}

	return db, nil
}

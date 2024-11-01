package db

import (
	"errors"
	"fmt"

	"github.com/Alge/tillit/db/sqliteconnector"
	"github.com/Alge/tillit/models"
	"github.com/Alge/tillit/responsetypes"
)

type DatabaseConnector interface {
	Close() error

	GetUser(id string) (*models.User, error)
	CreateUser(u *models.User) error
	DeleteUser(u *models.User) error
	GetUserList(page int, size int) (*responsetypes.PaginatedResponse[*models.User], error)

	GetConnection(id string) (*models.Connection, error)
	GetUserConnections(userID string) ([]*models.Connection, error)
	CreateConnection(u *models.Connection) error
	DeleteConnection(u *models.Connection) error
}

func Init(connector string, dsn string) (db DatabaseConnector, err error) {

	switch connector {
	case "sqlite3":
		db, e := sqliteconnector.Init(dsn)
		return db, e

	default:
		return db, errors.New(fmt.Sprintf("Don't know how to initialize a '%s' database", connector))
	}
}

package db

import (
  "fmt"
	"errors"
  
  "database/sql"
  _ "github.com/mattn/go-sqlite3"

	"github.com/Alge/tillit/models"
)

var DB *sql.DB
var initialized bool

type DatabaseConnector interface{
  GetUser(id string) (*models.User, error)
  CreateUser(u *models.User) error
  DeleteUser(u *models.User) error

  GetConnection(id string) (*models.Connection, error)
  GetUserConnections(userID string) ([]*models.Connection, error)
  CreateConnection(u *models.Connection) error
  DeleteConnection(u *models.Connection) error

  GetPubKey(id string) (*models.PubKey, error)
  GetUserPubKeys(userID string) ([]*models.PubKey, error)
  CreatePubKey(u *models.PubKey) error
  DeletePubKey(u *models.PubKey) error
}

func Init(connector string, dsn string) db *sql.DB, err error {

  switch connector{
  case "sqlite3":
    database, err := sql.Open(connector, dsn)
    if err != nil {
      return 
    }
    DB = database

  default:
    return errors.New(fmt.Sprintf("Don't know how to initialize a '%s' database", connector))
    
  }

	initialized = true

	return nil
}

func GetDB() (*sql.DB, error) {
	if !initialized {
		return DB, errors.New("Database not initialized yet")
	}

	return DB, nil
}

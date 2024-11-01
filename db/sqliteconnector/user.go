package sqliteconnector

import (
	"database/sql"
	"errors"
	_ "fmt"

	"github.com/Alge/tillit/db/dberrors"
	"github.com/Alge/tillit/models"
)

func (c *SqliteConnector) GetUser(id string) (u *models.User, err error) {

	stmt, err := c.DB.Prepare("SELECT (id, username) FROM users WHERE id = ?")
	if err != nil {
		return
	}
	defer stmt.Close()

	row := stmt.QueryRow(id)

	err = row.Scan(&u.ID, &u.Username)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return u, dberrors.NewObjectNotFoundError("No such user")
		default:
			return u, err
		}
	}
	return
}

func (c *SqliteConnector) CreateUser(u *models.User) error {
	return nil
}
func (c *SqliteConnector) DeleteUser(u *models.User) error {
	return nil
}

func CreateUserTable() error {
	return nil
}

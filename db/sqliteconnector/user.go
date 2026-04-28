package sqliteconnector

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/Alge/tillit/db/dberrors"
	"github.com/Alge/tillit/models"
)


func (c *SqliteConnector) GetUser(id string) (*models.User, error) {
	stmt, err := c.Database.Prepare("SELECT id, username, pubkey, algorithm FROM users WHERE id = ?")
	if err != nil {
		return nil, fmt.Errorf("failed preparing statement: %w", err)
	}
	defer stmt.Close()

	u := &models.User{}
	err = stmt.QueryRow(id).Scan(&u.ID, &u.Username, &u.PubKey, &u.Algorithm)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, dberrors.NewObjectNotFoundError("No such user")
		}
		return nil, err
	}
	return u, nil
}

func (c *SqliteConnector) CreateUser(u *models.User) error {
	stmt, err := c.Database.Prepare(`INSERT INTO users (id, username, pubkey, algorithm, is_admin) VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(u.ID, u.Username, u.PubKey, u.Algorithm, u.IsAdmin)
	return err
}
func (c *SqliteConnector) DeleteUser(u *models.User) error {
	return nil
}

func (c *SqliteConnector) CreateUserTable() error {
	stmt, err := c.Database.Prepare(`
		CREATE TABLE IF NOT EXISTS users (
			id        TEXT PRIMARY KEY,
			username  TEXT NOT NULL UNIQUE,
			pubkey    TEXT NOT NULL UNIQUE,
			algorithm TEXT NOT NULL DEFAULT '',
			is_admin  INTEGER DEFAULT 0
		);`)

	if err != nil {
		return err
	}

	_, err = stmt.Exec()

	return err
}

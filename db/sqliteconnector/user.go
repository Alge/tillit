package sqliteconnector

import (
	"database/sql"
	"errors"
	"fmt"
	_ "fmt"
	"log"

	"github.com/Alge/tillit/db/dberrors"
	"github.com/Alge/tillit/models"
	"github.com/Alge/tillit/responsetypes"
)

func (c *SqliteConnector) GetUser(id string) (u *models.User, err error) {

	stmt, err := c.Database.Prepare("SELECT id, username, pubkey FROM users WHERE id = ?")
	if err != nil {
		log.Printf("Failed creating statmemt: %s", err)
		return
	}
	defer stmt.Close()
	log.Printf("Statement created")

	row := stmt.QueryRow(id)

	log.Print("Run query")

	u = &models.User{}

	err = row.Scan(&u.ID, &u.Username, &u.PubKey)

	log.Print("Read results")
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):

			log.Print("No results")
			return u, dberrors.NewObjectNotFoundError("No such user")
		default:
			log.Printf("Other error: %s", err)
			return u, err

		}
	}
	return
}

func (c *SqliteConnector) GetUserList(page int, size int) (res *responsetypes.PaginatedResponse[*models.User], err error) {

	stmt, err := c.Database.Prepare("SELECT id, username, pubkey FROM users LIMIT ? OFFSET ?")
	if err != nil {
		log.Printf("Failed creating statmemt: %s", err)
		return
	}
	defer stmt.Close()

	res = &responsetypes.PaginatedResponse[*models.User]{
		Page: page,
	}

	res.Data = []*models.User{}

	rows, err := stmt.Query(size, (page-1)*size)
	if err != nil {
		return nil, fmt.Errorf("Failed fetching users from database: %w", err)
	}

	for rows.Next() {
		u := &models.User{}
		err = rows.Scan(&u.ID, &u.Username, &u.PubKey)
		if err != nil {
			return nil, fmt.Errorf("Failed parsing user object from database: %w", err)
		}
		res.Data = append(res.Data, u)
	}

	res.Size = len(res.Data)

	return res, nil
}

func (c *SqliteConnector) CreateUser(u *models.User) error {

	stmt, err := c.Database.Prepare(`
        INSERT INTO users (id, username, pubkey, is_admin)
        VALUES (?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}

	// TODO: Actually include the pubkey
	_, err = stmt.Exec(u.ID, u.Username, u.PubKey, u.IsAdmin)
	if err != nil {
		log.Printf("Sqlite: Failed inserting new user: %s", err)
	}
	return err
}
func (c *SqliteConnector) DeleteUser(u *models.User) error {
	return nil
}

func (c *SqliteConnector) CreateUserTable() error {
	stmt, err := c.Database.Prepare(`		
		CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			username TEXT NOT NULL UNIQUE,
			pubkey TEX NOT NULL UNIQUE,
			is_admin INTEGER DEFAULT 0
		);
	`)

	if err != nil {
		return err
	}

	_, err = stmt.Exec()

	return err
}

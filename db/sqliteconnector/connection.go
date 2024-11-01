package sqliteconnector

import "github.com/Alge/tillit/models"

func (c *SqliteConnector) GetConnection(id string) (*models.Connection, error) {
	return nil, nil
}
func (c *SqliteConnector) GetUserConnections(userID string) ([]*models.Connection, error) {
	return nil, nil
}
func (c *SqliteConnector) CreateConnection(u *models.Connection) error {
	return nil
}

func (c *SqliteConnector) DeleteConnection(u *models.Connection) error {
	return nil
}

func (c *SqliteConnector) CreateConnectionTable() error {
	stmt, err := c.Database.Prepare(`		
		CREATE TABLE IF NOT EXISTS connections (
			id TEXT PRIMARY KEY,
			owner TEXT NOT NULL,
			other TEXT NOT NULL
		);
	`)

	if err != nil {
		return err
	}

	_, err = stmt.Exec()

	return err
}

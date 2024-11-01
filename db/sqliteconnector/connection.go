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

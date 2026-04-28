package sqliteconnector

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/Alge/tillit/db/dberrors"
	"github.com/Alge/tillit/models"
)

func (c *SqliteConnector) GetConnection(id string) (*models.Connection, error) {
	stmt, err := c.Database.Prepare(`
		SELECT id, owner, other_id, public, trust, trust_extends, delegate
		FROM connections WHERE id = ?`)
	if err != nil {
		return nil, fmt.Errorf("failed preparing statement: %w", err)
	}
	defer stmt.Close()

	conn := &models.Connection{}
	err = stmt.QueryRow(id).Scan(
		&conn.ID, &conn.Owner, &conn.OtherID,
		&conn.Public, &conn.Trust, &conn.TrustExtends, &conn.Delegate,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, dberrors.NewObjectNotFoundError("no such connection")
		}
		return nil, err
	}
	return conn, nil
}

func (c *SqliteConnector) GetUserConnections(userID string) ([]*models.Connection, error) {
	stmt, err := c.Database.Prepare(`
		SELECT id, owner, other_id, public, trust, trust_extends, delegate
		FROM connections WHERE owner = ?`)
	if err != nil {
		return nil, fmt.Errorf("failed preparing statement: %w", err)
	}
	defer stmt.Close()

	rows, err := stmt.Query(userID)
	if err != nil {
		return nil, fmt.Errorf("failed querying connections: %w", err)
	}
	defer rows.Close()

	var conns []*models.Connection
	for rows.Next() {
		conn := &models.Connection{}
		if err := rows.Scan(
			&conn.ID, &conn.Owner, &conn.OtherID,
			&conn.Public, &conn.Trust, &conn.TrustExtends, &conn.Delegate,
		); err != nil {
			return nil, fmt.Errorf("failed scanning connection row: %w", err)
		}
		conns = append(conns, conn)
	}
	return conns, nil
}

func (c *SqliteConnector) CreateConnection(conn *models.Connection) error {
	stmt, err := c.Database.Prepare(`
		INSERT INTO connections (id, owner, other_id, public, trust, trust_extends, delegate)
		VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("failed preparing statement: %w", err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(
		conn.ID, conn.Owner, conn.OtherID,
		conn.Public, conn.Trust, conn.TrustExtends, conn.Delegate,
	)
	return err
}

func (c *SqliteConnector) DeleteConnection(conn *models.Connection) error {
	stmt, err := c.Database.Prepare(`DELETE FROM connections WHERE id = ?`)
	if err != nil {
		return fmt.Errorf("failed preparing statement: %w", err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(conn.ID)
	return err
}

func (c *SqliteConnector) CreateConnectionTable() error {
	stmt, err := c.Database.Prepare(`
		CREATE TABLE IF NOT EXISTS connections (
			id           TEXT PRIMARY KEY,
			owner        TEXT NOT NULL,
			other_id     TEXT NOT NULL,
			public       INTEGER NOT NULL DEFAULT 0,
			trust        INTEGER NOT NULL DEFAULT 0,
			trust_extends INTEGER NOT NULL DEFAULT 0,
			delegate     INTEGER NOT NULL DEFAULT 0,
			UNIQUE(owner, other_id)
		);`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec()
	return err
}

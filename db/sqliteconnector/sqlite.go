package sqliteconnector

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

type SqliteConnector struct {
	Database *sql.DB
}

func Init(dsn string) (*SqliteConnector, error) {
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}

	conn := &SqliteConnector{
		Database: db,
	}

	err = conn.CreateUserTable()
	if err != nil {
		return nil, fmt.Errorf("failed creating user table: %w", err)
	}

	err = conn.CreateConnectionTable()
	if err != nil {
		return nil, fmt.Errorf("failed creating connection table: %w", err)
	}

	return conn, err
}

func (c *SqliteConnector) Close() error {
	return c.Database.Close()
}

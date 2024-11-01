package sqliteconnector

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

type SqliteConnector struct {
	DB *sql.DB
}

func Init(dsn string) (c *SqliteConnector, err error) {
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return
	}

	c.DB = db

	return
}

func (c *SqliteConnector) Close() error {
	return c.DB.Close()
}

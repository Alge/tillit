package sqliteconnector

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Alge/tillit/db/dberrors"
	"github.com/Alge/tillit/models"
)

const connectionColumns = `id, owner, other_id, public, trust, trust_extends,
	payload, algorithm, sig, created_at, revoked, revoked_at`

func (c *SqliteConnector) GetConnection(id string) (*models.Connection, error) {
	row := c.Database.QueryRow(
		`SELECT `+connectionColumns+` FROM connections WHERE id = ?`, id)
	conn, err := scanConnection(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, dberrors.NewObjectNotFoundError("no such connection")
		}
		return nil, err
	}
	return conn, nil
}

func (c *SqliteConnector) GetUserConnections(userID string) ([]*models.Connection, error) {
	return c.queryConnections(`WHERE owner = ?`, userID)
}

// GetUserPublicConnections returns connections owned by userID. By
// default only the public, non-revoked rows are returned (the public
// peer-discovery view); when includePrivate is true the result also
// includes private rows — only the authenticated owner should ask for
// that. Revoked rows are always returned (they need to propagate to
// peers' caches so they can mark theirs revoked too).
func (c *SqliteConnector) GetUserPublicConnections(userID string, since *time.Time, includePrivate bool) ([]*models.Connection, error) {
	where := `owner = ?`
	args := []any{userID}
	if !includePrivate {
		where += ` AND public = 1 AND revoked = 0`
	}
	if since != nil {
		where += ` AND created_at > ?`
		args = append(args, since.UTC().Format(time.RFC3339))
	}
	return c.queryConnections(`WHERE `+where+` ORDER BY created_at ASC`, args...)
}

func (c *SqliteConnector) queryConnections(where string, args ...any) ([]*models.Connection, error) {
	rows, err := c.Database.Query(`SELECT `+connectionColumns+` FROM connections `+where, args...)
	if err != nil {
		return nil, fmt.Errorf("failed querying connections: %w", err)
	}
	defer rows.Close()

	var conns []*models.Connection
	for rows.Next() {
		conn, err := scanConnection(rows)
		if err != nil {
			return nil, fmt.Errorf("failed scanning connection row: %w", err)
		}
		conns = append(conns, conn)
	}
	return conns, nil
}

func (c *SqliteConnector) CreateConnection(conn *models.Connection) error {
	if conn.CreatedAt.IsZero() {
		conn.CreatedAt = time.Now().UTC()
	}
	_, err := c.Database.Exec(
		`INSERT INTO connections (`+connectionColumns+`)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, NULL)`,
		conn.ID, conn.Owner, conn.OtherID,
		conn.Public, conn.Trust, conn.TrustExtends,
		conn.Payload, conn.Algorithm, conn.Sig,
		conn.CreatedAt.UTC().Format(time.RFC3339),
	)
	return err
}

func (c *SqliteConnector) DeleteConnection(conn *models.Connection) error {
	_, err := c.Database.Exec(`DELETE FROM connections WHERE id = ?`, conn.ID)
	return err
}

// RevokeConnection marks a connection as revoked at the given time.
func (c *SqliteConnector) RevokeConnection(id string, at time.Time) error {
	res, err := c.Database.Exec(
		`UPDATE connections SET revoked = 1, revoked_at = ? WHERE id = ?`,
		at.UTC().Format(time.RFC3339), id,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return dberrors.NewObjectNotFoundError("no such connection")
	}
	return nil
}

func (c *SqliteConnector) CreateConnectionTable() error {
	_, err := c.Database.Exec(`
		CREATE TABLE IF NOT EXISTS connections (
			id            TEXT PRIMARY KEY,
			owner         TEXT NOT NULL,
			other_id      TEXT NOT NULL,
			public        INTEGER NOT NULL DEFAULT 0,
			trust         INTEGER NOT NULL DEFAULT 0,
			trust_extends INTEGER NOT NULL DEFAULT 0,
			payload       TEXT NOT NULL DEFAULT '',
			algorithm     TEXT NOT NULL DEFAULT '',
			sig           TEXT NOT NULL DEFAULT '',
			created_at    TEXT NOT NULL DEFAULT '',
			revoked       INTEGER NOT NULL DEFAULT 0,
			revoked_at    TEXT
		);`)
	return err
}

func scanConnection(row interface {
	Scan(dest ...any) error
}) (*models.Connection, error) {
	conn := &models.Connection{}
	var createdAtStr string
	var revokedAtStr sql.NullString
	if err := row.Scan(
		&conn.ID, &conn.Owner, &conn.OtherID,
		&conn.Public, &conn.Trust, &conn.TrustExtends,
		&conn.Payload, &conn.Algorithm, &conn.Sig,
		&createdAtStr, &conn.Revoked, &revokedAtStr,
	); err != nil {
		return nil, err
	}
	if createdAtStr != "" {
		t, err := time.Parse(time.RFC3339, createdAtStr)
		if err != nil {
			return nil, fmt.Errorf("failed parsing created_at: %w", err)
		}
		conn.CreatedAt = t
	}
	if revokedAtStr.Valid {
		t, err := time.Parse(time.RFC3339, revokedAtStr.String)
		if err != nil {
			return nil, fmt.Errorf("failed parsing revoked_at: %w", err)
		}
		conn.RevokedAt = &t
	}
	return conn, nil
}

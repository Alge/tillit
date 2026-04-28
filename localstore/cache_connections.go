package localstore

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type CachedConnection struct {
	ID        string
	Signer    string
	OtherID   string
	Payload   string
	Algorithm string
	Sig       string
	CreatedAt time.Time
	Revoked   bool
	RevokedAt *time.Time
	FetchedAt time.Time
}

func (s *Store) migrateCachedConnections() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS cached_connections (
			id          TEXT PRIMARY KEY,
			signer      TEXT NOT NULL,
			other_id    TEXT NOT NULL DEFAULT '',
			payload     TEXT NOT NULL,
			algorithm   TEXT NOT NULL,
			sig         TEXT NOT NULL,
			created_at  TEXT NOT NULL,
			revoked     INTEGER NOT NULL DEFAULT 0,
			revoked_at  TEXT,
			fetched_at  TEXT NOT NULL
		);`)
	return err
}

func (s *Store) SaveCachedConnection(c *CachedConnection) error {
	revokedAt := (*string)(nil)
	if c.RevokedAt != nil {
		v := c.RevokedAt.UTC().Format(time.RFC3339)
		revokedAt = &v
	}
	_, err := s.db.Exec(`
		INSERT INTO cached_connections
			(id, signer, other_id, payload, algorithm, sig, created_at, revoked, revoked_at, fetched_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			payload=excluded.payload, revoked=excluded.revoked,
			revoked_at=excluded.revoked_at, fetched_at=excluded.fetched_at`,
		c.ID, c.Signer, c.OtherID, c.Payload, c.Algorithm, c.Sig,
		c.CreatedAt.UTC().Format(time.RFC3339),
		c.Revoked, revokedAt,
		c.FetchedAt.UTC().Format(time.RFC3339),
	)
	return err
}

func (s *Store) GetCachedConnection(id string) (*CachedConnection, error) {
	return scanCachedConnection(s.db.QueryRow(
		`SELECT id, signer, other_id, payload, algorithm, sig, created_at, revoked, revoked_at, fetched_at
		 FROM cached_connections WHERE id = ?`, id))
}

// GetActiveConnection returns the most recent non-revoked connection from
// signer to other, or (nil, nil) if none exists.
func (s *Store) GetActiveConnection(signer, other string) (*CachedConnection, error) {
	row := s.db.QueryRow(
		`SELECT id, signer, other_id, payload, algorithm, sig, created_at, revoked, revoked_at, fetched_at
		 FROM cached_connections
		 WHERE signer = ? AND other_id = ? AND revoked = 0
		 ORDER BY created_at DESC LIMIT 1`, signer, other)
	c, err := scanCachedConnection(row)
	if err != nil {
		if err.Error() == "cached connection not found" {
			return nil, nil
		}
		return nil, err
	}
	return c, nil
}

func (s *Store) GetCachedConnectionsBySigner(signerID string) ([]*CachedConnection, error) {
	rows, err := s.db.Query(
		`SELECT id, signer, other_id, payload, algorithm, sig, created_at, revoked, revoked_at, fetched_at
		 FROM cached_connections WHERE signer = ? ORDER BY created_at ASC`, signerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var conns []*CachedConnection
	for rows.Next() {
		c, err := scanCachedConnection(rows)
		if err != nil {
			return nil, err
		}
		conns = append(conns, c)
	}
	return conns, nil
}

func scanCachedConnection(row scanner) (*CachedConnection, error) {
	var createdAtStr, fetchedAtStr string
	var revokedAtStr *string
	c := &CachedConnection{}

	err := row.Scan(
		&c.ID, &c.Signer, &c.OtherID, &c.Payload, &c.Algorithm, &c.Sig,
		&createdAtStr, &c.Revoked, &revokedAtStr, &fetchedAtStr,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("cached connection not found")
		}
		return nil, err
	}

	c.CreatedAt, err = time.Parse(time.RFC3339, createdAtStr)
	if err != nil {
		return nil, fmt.Errorf("failed parsing created_at: %w", err)
	}
	c.FetchedAt, err = time.Parse(time.RFC3339, fetchedAtStr)
	if err != nil {
		return nil, fmt.Errorf("failed parsing fetched_at: %w", err)
	}
	if revokedAtStr != nil {
		t, err := time.Parse(time.RFC3339, *revokedAtStr)
		if err != nil {
			return nil, fmt.Errorf("failed parsing revoked_at: %w", err)
		}
		c.RevokedAt = &t
	}
	return c, nil
}

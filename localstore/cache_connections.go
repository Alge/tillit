package localstore

import (
	"database/sql"
	"encoding/json"
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

// SaveCachedConnection inserts a connection row. ON CONFLICT(id) the
// existing row wins — same write-once rationale as SaveCachedSignature.
// Revocation is derived via IsCachedConnectionRevoked.
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
		ON CONFLICT(id) DO NOTHING`,
		c.ID, c.Signer, c.OtherID, c.Payload, c.Algorithm, c.Sig,
		c.CreatedAt.UTC().Format(time.RFC3339),
		c.Revoked, revokedAt,
		c.FetchedAt.UTC().Format(time.RFC3339),
	)
	return err
}

// IsCachedConnectionRevoked reports whether a connection_revocation
// signature targeting id, signed by the same signer, exists in the
// local cache. Mirrors IsCachedSignatureRevoked but on the connections
// table — connection revocations live as cached_connections rows
// whose payload type is "connection_revocation".
func (s *Store) IsCachedConnectionRevoked(id string) (bool, *time.Time, error) {
	target, err := s.GetCachedConnection(id)
	if err != nil {
		return false, nil, err
	}
	conns, err := s.GetCachedConnectionsBySigner(target.Signer)
	if err != nil {
		return false, nil, err
	}
	for _, candidate := range conns {
		if candidate.ID == id {
			continue
		}
		if isConnectionRevocationFor(candidate.Payload, id) {
			t := candidate.CreatedAt
			return true, &t, nil
		}
	}
	return false, nil, nil
}

func isConnectionRevocationFor(payload, targetID string) bool {
	var p struct {
		Type     string `json:"type"`
		TargetID string `json:"target_id"`
	}
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		return false
	}
	return p.Type == "connection_revocation" && p.TargetID == targetID
}

// DeleteCachedConnection removes the row with the given ID. Returns
// an error if no row matched.
func (s *Store) DeleteCachedConnection(id string) error {
	res, err := s.db.Exec(`DELETE FROM cached_connections WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("cached connection %q not found", id)
	}
	return nil
}

func (s *Store) GetCachedConnection(id string) (*CachedConnection, error) {
	return scanCachedConnection(s.db.QueryRow(
		`SELECT id, signer, other_id, payload, algorithm, sig, created_at, revoked, revoked_at, fetched_at
		 FROM cached_connections WHERE id = ?`, id))
}

// GetActiveConnection returns the most recent connection from signer
// to other that has not been revoked, or (nil, nil) if none exists.
// Revocation is derived from the existence of a
// connection_revocation row in the same signer's set — the cache
// row's mutable revoked column is ignored.
func (s *Store) GetActiveConnection(signer, other string) (*CachedConnection, error) {
	conns, err := s.GetCachedConnectionsBySigner(signer)
	if err != nil {
		return nil, err
	}
	revoked := map[string]bool{}
	for _, c := range conns {
		var p struct {
			Type     string `json:"type"`
			TargetID string `json:"target_id"`
		}
		if err := json.Unmarshal([]byte(c.Payload), &p); err != nil {
			continue
		}
		if p.Type == "connection_revocation" && p.TargetID != "" {
			revoked[p.TargetID] = true
		}
	}
	var best *CachedConnection
	for _, c := range conns {
		if c.OtherID != other {
			continue
		}
		if revoked[c.ID] {
			continue
		}
		var p struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal([]byte(c.Payload), &p); err != nil {
			continue
		}
		if p.Type != "connection" {
			continue
		}
		if best == nil || c.CreatedAt.After(best.CreatedAt) {
			best = c
		}
	}
	return best, nil
}

// ListAllCachedConnections returns every cached connection
// regardless of signer. Used by the export command.
func (s *Store) ListAllCachedConnections() ([]*CachedConnection, error) {
	rows, err := s.db.Query(
		`SELECT id, signer, other_id, payload, algorithm, sig, created_at, revoked, revoked_at, fetched_at
		 FROM cached_connections ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*CachedConnection
	for rows.Next() {
		c, err := scanCachedConnection(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
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

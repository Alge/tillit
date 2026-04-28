package localstore

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type Server struct {
	URL          string
	Alias        string
	UserID       string // our user ID on this server
	LastSyncedAt *time.Time
}

type Peer struct {
	ID           string // peer's user ID (SHA-256 pubkey hash)
	ServerURL    string
	TrustDepth   int
	Public       bool
	Distrusted   bool
	VetoOnly     bool // only inherit rejected decisions; ignore vetted/allowed
	LastSyncedAt *time.Time
}

func (s *Store) migratePeers() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS servers (
			url             TEXT PRIMARY KEY,
			alias           TEXT NOT NULL DEFAULT '',
			user_id         TEXT NOT NULL DEFAULT '',
			last_synced_at  TEXT
		);
		CREATE TABLE IF NOT EXISTS peers (
			id              TEXT PRIMARY KEY,
			server_url      TEXT NOT NULL,
			trust_depth     INTEGER NOT NULL DEFAULT 1,
			public          INTEGER NOT NULL DEFAULT 0,
			distrusted      INTEGER NOT NULL DEFAULT 0,
			veto_only       INTEGER NOT NULL DEFAULT 0,
			last_synced_at  TEXT
		);`)
	return err
}

func (s *Store) SaveServer(srv *Server) error {
	_, err := s.db.Exec(
		`INSERT INTO servers (url, alias, user_id) VALUES (?, ?, ?)
		 ON CONFLICT(url) DO UPDATE SET alias=excluded.alias, user_id=excluded.user_id`,
		srv.URL, srv.Alias, srv.UserID,
	)
	return err
}

func (s *Store) SetServerLastSyncedAt(url string, t time.Time) error {
	res, err := s.db.Exec(
		`UPDATE servers SET last_synced_at = ? WHERE url = ?`,
		t.UTC().Format(time.RFC3339), url,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("server %q not found", url)
	}
	return nil
}

func (s *Store) GetServer(url string) (*Server, error) {
	srv := &Server{}
	var syncedAt sql.NullString
	err := s.db.QueryRow(
		`SELECT url, alias, user_id, last_synced_at FROM servers WHERE url = ?`, url,
	).Scan(&srv.URL, &srv.Alias, &srv.UserID, &syncedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("server %q not found", url)
		}
		return nil, err
	}
	if syncedAt.Valid {
		t, err := time.Parse(time.RFC3339, syncedAt.String)
		if err != nil {
			return nil, fmt.Errorf("failed parsing last_synced_at: %w", err)
		}
		srv.LastSyncedAt = &t
	}
	return srv, nil
}

func (s *Store) ListServers() ([]*Server, error) {
	rows, err := s.db.Query(`SELECT url, alias, user_id, last_synced_at FROM servers ORDER BY url`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var servers []*Server
	for rows.Next() {
		srv := &Server{}
		var syncedAt sql.NullString
		if err := rows.Scan(&srv.URL, &srv.Alias, &srv.UserID, &syncedAt); err != nil {
			return nil, err
		}
		if syncedAt.Valid {
			t, err := time.Parse(time.RFC3339, syncedAt.String)
			if err != nil {
				return nil, fmt.Errorf("failed parsing last_synced_at: %w", err)
			}
			srv.LastSyncedAt = &t
		}
		servers = append(servers, srv)
	}
	return servers, nil
}

func (s *Store) SavePeer(p *Peer) error {
	_, err := s.db.Exec(
		`INSERT INTO peers (id, server_url, trust_depth, public, distrusted, veto_only)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET server_url=excluded.server_url,
		 trust_depth=excluded.trust_depth, public=excluded.public,
		 distrusted=excluded.distrusted, veto_only=excluded.veto_only`,
		p.ID, p.ServerURL, p.TrustDepth, p.Public, p.Distrusted, p.VetoOnly,
	)
	return err
}

func (s *Store) SetPeerLastSyncedAt(id string, t time.Time) error {
	res, err := s.db.Exec(
		`UPDATE peers SET last_synced_at = ? WHERE id = ?`,
		t.UTC().Format(time.RFC3339), id,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("peer %q not found", id)
	}
	return nil
}

func (s *Store) GetPeer(id string) (*Peer, error) {
	return scanPeer(s.db.QueryRow(
		`SELECT id, server_url, trust_depth, public, distrusted, veto_only, last_synced_at
		 FROM peers WHERE id = ?`, id))
}

func (s *Store) ListPeers() ([]*Peer, error) {
	rows, err := s.db.Query(
		`SELECT id, server_url, trust_depth, public, distrusted, veto_only, last_synced_at
		 FROM peers ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var peers []*Peer
	for rows.Next() {
		p, err := scanPeer(rows)
		if err != nil {
			return nil, err
		}
		peers = append(peers, p)
	}
	return peers, nil
}

func (s *Store) RemovePeer(id string) error {
	_, err := s.db.Exec(`DELETE FROM peers WHERE id = ?`, id)
	return err
}

func scanPeer(row scanner) (*Peer, error) {
	p := &Peer{}
	var syncedAt sql.NullString
	err := row.Scan(&p.ID, &p.ServerURL, &p.TrustDepth, &p.Public, &p.Distrusted, &p.VetoOnly, &syncedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("peer not found")
		}
		return nil, err
	}
	if syncedAt.Valid {
		t, err := time.Parse(time.RFC3339, syncedAt.String)
		if err != nil {
			return nil, fmt.Errorf("failed parsing last_synced_at: %w", err)
		}
		p.LastSyncedAt = &t
	}
	return p, nil
}

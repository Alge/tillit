package localstore

import (
	"database/sql"
	"errors"
	"fmt"
)

type Server struct {
	URL    string
	Alias  string
	UserID string // our user ID on this server
}

type Peer struct {
	ID         string // peer's user ID (SHA-256 pubkey hash)
	ServerURL  string
	TrustDepth int
	Public   bool
	Distrusted bool
}

func (s *Store) migratePeers() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS servers (
			url     TEXT PRIMARY KEY,
			alias   TEXT NOT NULL DEFAULT '',
			user_id TEXT NOT NULL DEFAULT ''
		);
		CREATE TABLE IF NOT EXISTS peers (
			id          TEXT PRIMARY KEY,
			server_url  TEXT NOT NULL,
			trust_depth INTEGER NOT NULL DEFAULT 1,
			public    INTEGER NOT NULL DEFAULT 0,
			distrusted  INTEGER NOT NULL DEFAULT 0
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

func (s *Store) GetServer(url string) (*Server, error) {
	srv := &Server{}
	err := s.db.QueryRow(
		`SELECT url, alias, user_id FROM servers WHERE url = ?`, url,
	).Scan(&srv.URL, &srv.Alias, &srv.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("server %q not found", url)
		}
		return nil, err
	}
	return srv, nil
}

func (s *Store) ListServers() ([]*Server, error) {
	rows, err := s.db.Query(`SELECT url, alias, user_id FROM servers ORDER BY url`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var servers []*Server
	for rows.Next() {
		srv := &Server{}
		if err := rows.Scan(&srv.URL, &srv.Alias, &srv.UserID); err != nil {
			return nil, err
		}
		servers = append(servers, srv)
	}
	return servers, nil
}

func (s *Store) SavePeer(p *Peer) error {
	_, err := s.db.Exec(
		`INSERT INTO peers (id, server_url, trust_depth, public, distrusted) VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET server_url=excluded.server_url,
		 trust_depth=excluded.trust_depth, public=excluded.public,
		 distrusted=excluded.distrusted`,
		p.ID, p.ServerURL, p.TrustDepth, p.Public, p.Distrusted,
	)
	return err
}

func (s *Store) GetPeer(id string) (*Peer, error) {
	p := &Peer{}
	err := s.db.QueryRow(
		`SELECT id, server_url, trust_depth, public, distrusted FROM peers WHERE id = ?`, id,
	).Scan(&p.ID, &p.ServerURL, &p.TrustDepth, &p.Public, &p.Distrusted)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("peer %q not found", id)
		}
		return nil, err
	}
	return p, nil
}

func (s *Store) ListPeers() ([]*Peer, error) {
	rows, err := s.db.Query(
		`SELECT id, server_url, trust_depth, public, distrusted FROM peers ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var peers []*Peer
	for rows.Next() {
		p := &Peer{}
		if err := rows.Scan(&p.ID, &p.ServerURL, &p.TrustDepth, &p.Public, &p.Distrusted); err != nil {
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

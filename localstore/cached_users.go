package localstore

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type CachedUser struct {
	ID        string
	Username  string
	PubKey    string
	Algorithm string
	FetchedAt time.Time
}

func (s *Store) migrateCachedUsers() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS cached_users (
			id         TEXT PRIMARY KEY,
			username   TEXT NOT NULL DEFAULT '',
			pubkey     TEXT NOT NULL,
			algorithm  TEXT NOT NULL,
			fetched_at TEXT NOT NULL
		);`)
	return err
}

func (s *Store) SaveCachedUser(u *CachedUser) error {
	_, err := s.db.Exec(
		`INSERT INTO cached_users (id, username, pubkey, algorithm, fetched_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET username=excluded.username,
		 pubkey=excluded.pubkey, algorithm=excluded.algorithm, fetched_at=excluded.fetched_at`,
		u.ID, u.Username, u.PubKey, u.Algorithm,
		u.FetchedAt.UTC().Format(time.RFC3339),
	)
	return err
}

func (s *Store) GetCachedUser(id string) (*CachedUser, error) {
	u := &CachedUser{}
	var fetchedAtStr string
	err := s.db.QueryRow(
		`SELECT id, username, pubkey, algorithm, fetched_at FROM cached_users WHERE id = ?`, id,
	).Scan(&u.ID, &u.Username, &u.PubKey, &u.Algorithm, &fetchedAtStr)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("cached user %q not found", id)
		}
		return nil, err
	}
	t, err := time.Parse(time.RFC3339, fetchedAtStr)
	if err != nil {
		return nil, fmt.Errorf("failed parsing fetched_at: %w", err)
	}
	u.FetchedAt = t
	return u, nil
}

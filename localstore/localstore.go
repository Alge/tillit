package localstore

import (
	"database/sql"
	"errors"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

type Store struct {
	db *sql.DB
}

type Key struct {
	Name      string
	Algorithm string
	PubKey    string
	PrivKey   string
}

func Init(dsn string) (*Store, error) {
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migration failed: %w", err)
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS keys (
			name      TEXT PRIMARY KEY,
			algorithm TEXT NOT NULL,
			pubkey    TEXT NOT NULL,
			privkey   TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS config (
			key   TEXT PRIMARY KEY,
			value TEXT NOT NULL
		);`)
	if err != nil {
		return err
	}
	if err := s.migratePeers(); err != nil {
		return err
	}
	if err := s.migrateCache(); err != nil {
		return err
	}
	return s.migrateCachedConnections()
}

func (s *Store) SaveKey(k *Key) error {
	_, err := s.db.Exec(
		`INSERT INTO keys (name, algorithm, pubkey, privkey) VALUES (?, ?, ?, ?)
		 ON CONFLICT(name) DO UPDATE SET algorithm=excluded.algorithm,
		 pubkey=excluded.pubkey, privkey=excluded.privkey`,
		k.Name, k.Algorithm, k.PubKey, k.PrivKey,
	)
	return err
}

func (s *Store) GetKey(name string) (*Key, error) {
	k := &Key{}
	err := s.db.QueryRow(
		`SELECT name, algorithm, pubkey, privkey FROM keys WHERE name = ?`, name,
	).Scan(&k.Name, &k.Algorithm, &k.PubKey, &k.PrivKey)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("key %q not found", name)
		}
		return nil, err
	}
	return k, nil
}

func (s *Store) ListKeys() ([]*Key, error) {
	rows, err := s.db.Query(`SELECT name, algorithm, pubkey, privkey FROM keys ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []*Key
	for rows.Next() {
		k := &Key{}
		if err := rows.Scan(&k.Name, &k.Algorithm, &k.PubKey, &k.PrivKey); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, nil
}

func (s *Store) SetActiveKey(name string) error {
	_, err := s.db.Exec(
		`INSERT INTO config (key, value) VALUES ('active_key', ?)
		 ON CONFLICT(key) DO UPDATE SET value=excluded.value`, name,
	)
	return err
}

func (s *Store) GetActiveKey() (string, error) {
	var name string
	err := s.db.QueryRow(`SELECT value FROM config WHERE key = 'active_key'`).Scan(&name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("no active key set")
		}
		return "", err
	}
	return name, nil
}

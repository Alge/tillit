package localstore

import "time"

type ItemType string

const (
	ItemConnection ItemType = "connection"
	ItemSignature  ItemType = "signature"
)

func (s *Store) migratePushState() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS push_state (
			item_id    TEXT NOT NULL,
			item_type  TEXT NOT NULL,
			server_url TEXT NOT NULL,
			pushed_at  TEXT NOT NULL,
			PRIMARY KEY (item_id, item_type, server_url)
		);`)
	return err
}

func (s *Store) RecordPush(itemID string, itemType ItemType, serverURL string, at time.Time) error {
	_, err := s.db.Exec(
		`INSERT INTO push_state (item_id, item_type, server_url, pushed_at) VALUES (?, ?, ?, ?)
		 ON CONFLICT(item_id, item_type, server_url) DO UPDATE SET pushed_at=excluded.pushed_at`,
		itemID, string(itemType), serverURL, at.UTC().Format(time.RFC3339),
	)
	return err
}

func (s *Store) IsPushed(itemID string, itemType ItemType, serverURL string) (bool, error) {
	var n int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM push_state
		 WHERE item_id = ? AND item_type = ? AND server_url = ?`,
		itemID, string(itemType), serverURL,
	).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

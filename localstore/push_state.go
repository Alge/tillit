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

// HasBeenPushed reports whether the item has been pushed to ANY
// server. Used by 'tillit delete' to decide whether outright deletion
// is safe (no peer has fetched it) or whether the user needs to revoke
// instead.
func (s *Store) HasBeenPushed(itemID string, itemType ItemType) (bool, error) {
	var n int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM push_state WHERE item_id = ? AND item_type = ?`,
		itemID, string(itemType),
	).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// PushStateRow is one entry from push_state. Exported only for
// snapshot purposes (export/import).
type PushStateRow struct {
	ItemID    string
	ItemType  ItemType
	ServerURL string
	PushedAt  time.Time
}

// ListAllPushState returns every row in push_state. Used by the
// export command to snapshot the local state.
func (s *Store) ListAllPushState() ([]*PushStateRow, error) {
	rows, err := s.db.Query(`SELECT item_id, item_type, server_url, pushed_at FROM push_state`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*PushStateRow
	for rows.Next() {
		r := &PushStateRow{}
		var pushedAtStr string
		var typeStr string
		if err := rows.Scan(&r.ItemID, &typeStr, &r.ServerURL, &pushedAtStr); err != nil {
			return nil, err
		}
		r.ItemType = ItemType(typeStr)
		t, err := time.Parse(time.RFC3339, pushedAtStr)
		if err != nil {
			return nil, err
		}
		r.PushedAt = t
		out = append(out, r)
	}
	return out, nil
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

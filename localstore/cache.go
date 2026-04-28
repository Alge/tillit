package localstore

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type CachedSignature struct {
	ID         string
	Signer     string
	Payload    string
	Algorithm  string
	Sig        string
	UploadedAt time.Time
	Revoked    bool
	RevokedAt  *time.Time
	FetchedAt  time.Time
}

func (s *Store) migrateCache() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS cached_signatures (
			id          TEXT PRIMARY KEY,
			signer      TEXT NOT NULL,
			payload     TEXT NOT NULL,
			algorithm   TEXT NOT NULL,
			sig         TEXT NOT NULL,
			uploaded_at TEXT NOT NULL,
			revoked     INTEGER NOT NULL DEFAULT 0,
			revoked_at  TEXT,
			fetched_at  TEXT NOT NULL
		);`)
	return err
}

func (s *Store) SaveCachedSignature(sig *CachedSignature) error {
	revokedAt := (*string)(nil)
	if sig.RevokedAt != nil {
		v := sig.RevokedAt.UTC().Format(time.RFC3339)
		revokedAt = &v
	}
	_, err := s.db.Exec(`
		INSERT INTO cached_signatures
			(id, signer, payload, algorithm, sig, uploaded_at, revoked, revoked_at, fetched_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			payload=excluded.payload, revoked=excluded.revoked,
			revoked_at=excluded.revoked_at, fetched_at=excluded.fetched_at`,
		sig.ID, sig.Signer, sig.Payload, sig.Algorithm, sig.Sig,
		sig.UploadedAt.UTC().Format(time.RFC3339),
		sig.Revoked, revokedAt,
		sig.FetchedAt.UTC().Format(time.RFC3339),
	)
	return err
}

func (s *Store) GetCachedSignature(id string) (*CachedSignature, error) {
	return scanCachedSignature(s.db.QueryRow(
		`SELECT id, signer, payload, algorithm, sig, uploaded_at, revoked, revoked_at, fetched_at
		 FROM cached_signatures WHERE id = ?`, id))
}

func (s *Store) GetCachedSignaturesBySigner(signerID string) ([]*CachedSignature, error) {
	rows, err := s.db.Query(
		`SELECT id, signer, payload, algorithm, sig, uploaded_at, revoked, revoked_at, fetched_at
		 FROM cached_signatures WHERE signer = ? ORDER BY uploaded_at ASC`, signerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sigs []*CachedSignature
	for rows.Next() {
		sig, err := scanCachedSignature(rows)
		if err != nil {
			return nil, err
		}
		sigs = append(sigs, sig)
	}
	return sigs, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanCachedSignature(row scanner) (*CachedSignature, error) {
	var uploadedAtStr, fetchedAtStr string
	var revokedAtStr *string
	sig := &CachedSignature{}

	err := row.Scan(
		&sig.ID, &sig.Signer, &sig.Payload, &sig.Algorithm, &sig.Sig,
		&uploadedAtStr, &sig.Revoked, &revokedAtStr, &fetchedAtStr,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("cached signature not found")
		}
		return nil, err
	}

	sig.UploadedAt, err = time.Parse(time.RFC3339, uploadedAtStr)
	if err != nil {
		return nil, fmt.Errorf("failed parsing uploaded_at: %w", err)
	}
	sig.FetchedAt, err = time.Parse(time.RFC3339, fetchedAtStr)
	if err != nil {
		return nil, fmt.Errorf("failed parsing fetched_at: %w", err)
	}
	if revokedAtStr != nil {
		t, err := time.Parse(time.RFC3339, *revokedAtStr)
		if err != nil {
			return nil, fmt.Errorf("failed parsing revoked_at: %w", err)
		}
		sig.RevokedAt = &t
	}
	return sig, nil
}

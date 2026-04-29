package localstore

import (
	"database/sql"
	"encoding/json"
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

// SaveCachedSignature inserts a signature row. ON CONFLICT(id) the
// existing row wins — cached rows are immutable signed artifacts,
// keyed by content hash, so any second write necessarily has the
// same payload+sig anyway. Never overwriting also rules out a class
// of bugs where a fresh fetch from a server (or peer) that doesn't
// know about a revocation could clobber locally-known state.
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
		ON CONFLICT(id) DO NOTHING`,
		sig.ID, sig.Signer, sig.Payload, sig.Algorithm, sig.Sig,
		sig.UploadedAt.UTC().Format(time.RFC3339),
		sig.Revoked, revokedAt,
		sig.FetchedAt.UTC().Format(time.RFC3339),
	)
	return err
}

// IsCachedSignatureRevoked reports whether a revocation signature
// targeting id, signed by the same signer, exists in the local cache.
// Revocation status is purely derived — there is no mutable revoked
// flag the cache trusts. Returns the revocation's upload time as the
// effective revoked_at when one exists.
func (s *Store) IsCachedSignatureRevoked(id string) (bool, *time.Time, error) {
	target, err := s.GetCachedSignature(id)
	if err != nil {
		return false, nil, err
	}
	sigs, err := s.GetCachedSignaturesBySigner(target.Signer)
	if err != nil {
		return false, nil, err
	}
	for _, candidate := range sigs {
		if candidate.ID == id {
			continue
		}
		// Lightweight check on the payload — the resolver and inspect
		// already use the same json.Unmarshal pattern, so this is the
		// cheapest place to centralise the rule.
		if isRevocationFor(candidate.Payload, id) {
			t := candidate.UploadedAt
			return true, &t, nil
		}
	}
	return false, nil, nil
}

// isRevocationFor returns true when the cached payload JSON declares
// type=revocation with the given target_id. Defined here (rather than
// pulling in models package) to keep localstore self-contained.
func isRevocationFor(payload, targetID string) bool {
	var p struct {
		Type     string `json:"type"`
		TargetID string `json:"target_id"`
	}
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		return false
	}
	return p.Type == "revocation" && p.TargetID == targetID
}

func (s *Store) GetCachedSignature(id string) (*CachedSignature, error) {
	return scanCachedSignature(s.db.QueryRow(
		`SELECT id, signer, payload, algorithm, sig, uploaded_at, revoked, revoked_at, fetched_at
		 FROM cached_signatures WHERE id = ?`, id))
}

// LookupCachedSignature returns the cached signature whose ID exactly
// matches q, or whose ID has q as a prefix when no exact match exists.
// Returns an error if zero or more than one signature matches the prefix.
func (s *Store) LookupCachedSignature(q string) (*CachedSignature, error) {
	if q == "" {
		return nil, fmt.Errorf("signature id is empty")
	}
	if sig, err := s.GetCachedSignature(q); err == nil {
		return sig, nil
	}
	rows, err := s.db.Query(
		`SELECT id, signer, payload, algorithm, sig, uploaded_at, revoked, revoked_at, fetched_at
		 FROM cached_signatures WHERE id LIKE ? LIMIT 2`, q+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var matches []*CachedSignature
	for rows.Next() {
		sig, err := scanCachedSignature(rows)
		if err != nil {
			return nil, err
		}
		matches = append(matches, sig)
	}
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("no signature matches %q", q)
	case 1:
		return matches[0], nil
	default:
		return nil, fmt.Errorf("signature prefix %q is ambiguous (matches %s, %s, ...)",
			q, matches[0].ID, matches[1].ID)
	}
}

// ListAllCachedSignatures returns every cached signature regardless
// of signer. Used by the export command.
func (s *Store) ListAllCachedSignatures() ([]*CachedSignature, error) {
	rows, err := s.db.Query(
		`SELECT id, signer, payload, algorithm, sig, uploaded_at, revoked, revoked_at, fetched_at
		 FROM cached_signatures ORDER BY uploaded_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*CachedSignature
	for rows.Next() {
		sig, err := scanCachedSignature(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sig)
	}
	return out, nil
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

// DeleteCachedSignature removes the row with the given ID. Returns an
// error if no row matched, so callers can distinguish "deleted" from
// "didn't exist". Used by 'tillit delete' on signatures that have not
// yet been pushed.
func (s *Store) DeleteCachedSignature(id string) error {
	res, err := s.db.Exec(`DELETE FROM cached_signatures WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("cached signature %q not found", id)
	}
	return nil
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

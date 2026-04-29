package sqliteconnector

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Alge/tillit/db/dberrors"
	"github.com/Alge/tillit/models"
)

const signatureColumns = `id, signer, payload, algorithm, sig, uploaded_at, revoked, revoked_at, public`

func (c *SqliteConnector) GetSignature(id string) (*models.Signature, error) {
	row := c.Database.QueryRow(`SELECT `+signatureColumns+` FROM signatures WHERE id = ?`, id)
	s, err := scanSignature(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, dberrors.NewObjectNotFoundError("no such signature")
		}
		return nil, err
	}
	return s, nil
}

func scanSignature(row interface {
	Scan(...any) error
}) (*models.Signature, error) {
	s := &models.Signature{}
	var uploadedAtStr string
	var revokedAtStr *string
	if err := row.Scan(
		&s.ID, &s.Signer, &s.Payload, &s.Algorithm, &s.Sig,
		&uploadedAtStr, &s.Revoked, &revokedAtStr, &s.Public,
	); err != nil {
		return nil, err
	}
	t, err := time.Parse(time.RFC3339, uploadedAtStr)
	if err != nil {
		return nil, fmt.Errorf("failed parsing uploaded_at: %w", err)
	}
	s.UploadedAt = t
	if revokedAtStr != nil {
		rt, err := time.Parse(time.RFC3339, *revokedAtStr)
		if err != nil {
			return nil, fmt.Errorf("failed parsing revoked_at: %w", err)
		}
		s.RevokedAt = &rt
	}
	return s, nil
}

// GetUserSignatures returns the public signatures for signerID. When
// includePrivate is true the result also includes private rows — only
// the authenticated owner should ask for that.
func (c *SqliteConnector) GetUserSignatures(signerID string, since *time.Time, includePrivate bool) ([]*models.Signature, error) {
	where := `signer = ?`
	args := []any{signerID}
	if !includePrivate {
		where += ` AND public = 1`
	}
	if since != nil {
		where += ` AND uploaded_at >= ?`
		args = append(args, since.UTC().Format(time.RFC3339))
	}
	rows, err := c.Database.Query(
		`SELECT `+signatureColumns+` FROM signatures WHERE `+where+` ORDER BY uploaded_at ASC`,
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("failed querying signatures: %w", err)
	}
	defer rows.Close()

	var sigs []*models.Signature
	for rows.Next() {
		s, err := scanSignature(rows)
		if err != nil {
			return nil, fmt.Errorf("failed scanning signature row: %w", err)
		}
		sigs = append(sigs, s)
	}
	return sigs, nil
}

// CreateSignature inserts a signature. ON CONFLICT(id) the existing
// row is left untouched — re-uploading the same signature (e.g.
// re-running 'tillit mirror push' or syncing the same row from
// multiple devices) is a silent no-op rather than an error.
func (c *SqliteConnector) CreateSignature(s *models.Signature) error {
	_, err := c.Database.Exec(
		`INSERT INTO signatures (id, signer, payload, algorithm, sig, uploaded_at, public)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO NOTHING`,
		s.ID, s.Signer, s.Payload, s.Algorithm, s.Sig,
		s.UploadedAt.UTC().Format(time.RFC3339), s.Public,
	)
	return err
}

func (c *SqliteConnector) RevokeSignature(id string, at time.Time) error {
	stmt, err := c.Database.Prepare(`
		UPDATE signatures SET revoked = 1, revoked_at = ? WHERE id = ?`)
	if err != nil {
		return fmt.Errorf("failed preparing statement: %w", err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(at.UTC().Format(time.RFC3339), id)
	return err
}

func (c *SqliteConnector) CreateSignatureTable() error {
	if _, err := c.Database.Exec(`
		CREATE TABLE IF NOT EXISTS signatures (
			id          TEXT PRIMARY KEY,
			signer      TEXT NOT NULL,
			payload     TEXT NOT NULL,
			algorithm   TEXT NOT NULL,
			sig         TEXT NOT NULL,
			uploaded_at TEXT NOT NULL,
			revoked     INTEGER NOT NULL DEFAULT 0,
			revoked_at  TEXT,
			public      INTEGER NOT NULL DEFAULT 1
		);`); err != nil {
		return err
	}
	// Older databases predate the public column. ALTER TABLE … ADD COLUMN
	// fails if the column already exists; we treat that as success.
	if _, err := c.Database.Exec(
		`ALTER TABLE signatures ADD COLUMN public INTEGER NOT NULL DEFAULT 1`,
	); err != nil && !strings.Contains(err.Error(), "duplicate column") {
		return err
	}
	return nil
}

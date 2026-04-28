package sqliteconnector

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Alge/tillit/db/dberrors"
	"github.com/Alge/tillit/models"
)

func (c *SqliteConnector) GetSignature(id string) (*models.Signature, error) {
	stmt, err := c.Database.Prepare(`
		SELECT id, signer, payload, algorithm, sig, uploaded_at, revoked, revoked_at
		FROM signatures WHERE id = ?`)
	if err != nil {
		return nil, fmt.Errorf("failed preparing statement: %w", err)
	}
	defer stmt.Close()

	var uploadedAtStr string
	var revokedAtStr *string
	s := &models.Signature{}

	err = stmt.QueryRow(id).Scan(
		&s.ID, &s.Signer, &s.Payload, &s.Algorithm, &s.Sig,
		&uploadedAtStr, &s.Revoked, &revokedAtStr,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, dberrors.NewObjectNotFoundError("no such signature")
		}
		return nil, err
	}

	s.UploadedAt, err = time.Parse(time.RFC3339, uploadedAtStr)
	if err != nil {
		return nil, fmt.Errorf("failed parsing uploaded_at: %w", err)
	}
	if revokedAtStr != nil {
		t, err := time.Parse(time.RFC3339, *revokedAtStr)
		if err != nil {
			return nil, fmt.Errorf("failed parsing revoked_at: %w", err)
		}
		s.RevokedAt = &t
	}
	return s, nil
}

func (c *SqliteConnector) GetUserSignatures(signerID string, since *time.Time) ([]*models.Signature, error) {
	var (
		stmt *sql.Stmt
		err  error
	)
	if since != nil {
		stmt, err = c.Database.Prepare(`
			SELECT id, signer, payload, algorithm, sig, uploaded_at, revoked, revoked_at
			FROM signatures WHERE signer = ? AND uploaded_at >= ?
			ORDER BY uploaded_at ASC`)
	} else {
		stmt, err = c.Database.Prepare(`
			SELECT id, signer, payload, algorithm, sig, uploaded_at, revoked, revoked_at
			FROM signatures WHERE signer = ?
			ORDER BY uploaded_at ASC`)
	}
	if err != nil {
		return nil, fmt.Errorf("failed preparing statement: %w", err)
	}
	defer stmt.Close()

	var rows *sql.Rows
	if since != nil {
		rows, err = stmt.Query(signerID, since.UTC().Format(time.RFC3339))
	} else {
		rows, err = stmt.Query(signerID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed querying signatures: %w", err)
	}
	defer rows.Close()

	var sigs []*models.Signature
	for rows.Next() {
		var uploadedAtStr string
		var revokedAtStr *string
		s := &models.Signature{}
		if err := rows.Scan(
			&s.ID, &s.Signer, &s.Payload, &s.Algorithm, &s.Sig,
			&uploadedAtStr, &s.Revoked, &revokedAtStr,
		); err != nil {
			return nil, fmt.Errorf("failed scanning signature row: %w", err)
		}
		s.UploadedAt, err = time.Parse(time.RFC3339, uploadedAtStr)
		if err != nil {
			return nil, fmt.Errorf("failed parsing uploaded_at: %w", err)
		}
		if revokedAtStr != nil {
			t, err := time.Parse(time.RFC3339, *revokedAtStr)
			if err != nil {
				return nil, fmt.Errorf("failed parsing revoked_at: %w", err)
			}
			s.RevokedAt = &t
		}
		sigs = append(sigs, s)
	}
	return sigs, nil
}

func (c *SqliteConnector) CreateSignature(s *models.Signature) error {
	stmt, err := c.Database.Prepare(`
		INSERT INTO signatures (id, signer, payload, algorithm, sig, uploaded_at)
		VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("failed preparing statement: %w", err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(
		s.ID, s.Signer, s.Payload, s.Algorithm, s.Sig,
		s.UploadedAt.UTC().Format(time.RFC3339),
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
	stmt, err := c.Database.Prepare(`
		CREATE TABLE IF NOT EXISTS signatures (
			id          TEXT PRIMARY KEY,
			signer      TEXT NOT NULL,
			payload     TEXT NOT NULL,
			algorithm   TEXT NOT NULL,
			sig         TEXT NOT NULL,
			uploaded_at TEXT NOT NULL,
			revoked     INTEGER NOT NULL DEFAULT 0,
			revoked_at  TEXT
		);`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec()
	return err
}

package commands

import (
	"fmt"
	"time"

	"github.com/Alge/tillit/localstore"
	"github.com/Alge/tillit/models"
)

func Sign(args []string) error {
	// usage: tillit sign <ecosystem> <package_id> <version> --level <allowed|vetted|rejected> [--reason "..."]
	if len(args) < 3 {
		return fmt.Errorf("usage: tillit sign <ecosystem> <package_id> <version> --level <allowed|vetted|rejected> [--reason \"...\"]")
	}
	ecosystem := args[0]
	packageID := args[1]
	version := args[2]

	level := ""
	reason := ""
	for i := 3; i < len(args); i++ {
		switch args[i] {
		case "--level":
			if i+1 >= len(args) {
				return fmt.Errorf("--level requires a value")
			}
			i++
			level = args[i]
		case "--reason":
			if i+1 >= len(args) {
				return fmt.Errorf("--reason requires a value")
			}
			i++
			reason = args[i]
		default:
			return fmt.Errorf("unknown flag: %s", args[i])
		}
	}
	if level == "" {
		return fmt.Errorf("--level is required (allowed, vetted, or rejected)")
	}

	s, err := openStore()
	if err != nil {
		return err
	}
	defer s.Close()

	signer, userID, err := activeSignerAndID(s)
	if err != nil {
		return err
	}

	payload := &models.Payload{
		Type:      models.PayloadTypeDecision,
		Signer:    userID,
		Ecosystem: ecosystem,
		PackageID: packageID,
		Version:   version,
		Level:     models.DecisionLevel(level),
		Reason:    reason,
	}
	signed, err := signPayload(signer, payload)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	if err := s.SaveCachedSignature(&localstore.CachedSignature{
		ID:         signed.ID,
		Signer:     userID,
		Payload:    signed.Payload,
		Algorithm:  signed.Algorithm,
		Sig:        signed.Sig,
		UploadedAt: now,
		FetchedAt:  now,
	}); err != nil {
		return fmt.Errorf("failed saving signature: %w", err)
	}

	fmt.Printf("Signed %s/%s@%s as %s (id: %s)\n", ecosystem, packageID, version, level, signed.ID)
	fmt.Println("Run 'tillit publish' to push it to your registered servers.")
	return nil
}

func Revoke(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: tillit revoke <signature_id>")
	}
	targetID := args[0]

	s, err := openStore()
	if err != nil {
		return err
	}
	defer s.Close()

	signer, userID, err := activeSignerAndID(s)
	if err != nil {
		return err
	}

	payload := &models.Payload{
		Type:     models.PayloadTypeRevocation,
		Signer:   userID,
		TargetID: targetID,
	}
	signed, err := signPayload(signer, payload)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	if err := s.SaveCachedSignature(&localstore.CachedSignature{
		ID:         signed.ID,
		Signer:     userID,
		Payload:    signed.Payload,
		Algorithm:  signed.Algorithm,
		Sig:        signed.Sig,
		UploadedAt: now,
		FetchedAt:  now,
	}); err != nil {
		return fmt.Errorf("failed saving revocation: %w", err)
	}

	// Mark the locally-cached target as revoked too, so check works offline.
	if existing, err := s.GetCachedSignature(targetID); err == nil {
		existing.Revoked = true
		existing.RevokedAt = &now
		if err := s.SaveCachedSignature(existing); err != nil {
			fmt.Printf("warning: failed marking target %s revoked locally: %v\n", targetID, err)
		}
	}

	fmt.Printf("Revoked %s (id: %s)\n", targetID, signed.ID)
	fmt.Println("Run 'tillit publish' to push it to your registered servers.")
	return nil
}

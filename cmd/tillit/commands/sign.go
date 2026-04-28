package commands

import (
	"fmt"
	"time"

	tillit_crypto "github.com/Alge/tillit/crypto"
	"github.com/Alge/tillit/ecosystems"
	"github.com/Alge/tillit/localstore"
	"github.com/Alge/tillit/models"
)

// Sign dispatches to a subcommand: "version" for exact-version vettings
// or "delta" for change-between-versions reviews.
func Sign(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: tillit sign <version|delta> ...")
	}
	sub := args[0]
	rest := args[1:]
	switch sub {
	case "version":
		return signVersion(rest)
	case "delta":
		return signDelta(rest)
	default:
		return fmt.Errorf("unknown sign subcommand %q (expected version or delta)", sub)
	}
}

// signVersion creates a vetting decision for an exact version.
//
// usage: tillit sign version <ecosystem> <package> <version> --level <l> [--reason "..."]
func signVersion(args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("usage: tillit sign version <ecosystem> <package> <version> --level <allowed|vetted|rejected> [--reason \"...\"]")
	}
	ecosystem, packageID, version := args[0], args[1], args[2]

	level, reason, err := parseLevelReason(args[3:])
	if err != nil {
		return err
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
		Level:     level,
		Reason:    reason,
	}
	id, err := signAndCache(s, signer, userID, payload)
	if err != nil {
		return err
	}
	fmt.Printf("Signed %s/%s@%s as %s (id: %s)\n", ecosystem, packageID, version, level, id)
	fmt.Println("Run 'tillit publish' to push it to your registered servers.")
	return nil
}

// signDelta creates a delta decision attesting to review of the changes
// between two versions.
//
// usage: tillit sign delta <ecosystem> <package> <from> <to> --level <l> [--reason "..."]
func signDelta(args []string) error {
	if len(args) < 4 {
		return fmt.Errorf("usage: tillit sign delta <ecosystem> <package> <from-version> <to-version> --level <allowed|vetted|rejected> [--reason \"...\"]")
	}
	ecosystem, packageID, from, to := args[0], args[1], args[2], args[3]

	level, reason, err := parseLevelReason(args[4:])
	if err != nil {
		return err
	}

	if a, ok := adapterForEcosystem(ecosystem); ok {
		if a.CompareVersions(from, to) >= 0 {
			return fmt.Errorf("from version %s must precede to version %s in %s ordering",
				from, to, ecosystem)
		}
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
		Type:        models.PayloadTypeDeltaDecision,
		Signer:      userID,
		Ecosystem:   ecosystem,
		PackageID:   packageID,
		FromVersion: from,
		ToVersion:   to,
		Level:       level,
		Reason:      reason,
	}
	id, err := signAndCache(s, signer, userID, payload)
	if err != nil {
		return err
	}
	fmt.Printf("Signed delta %s/%s %s → %s as %s (id: %s)\n", ecosystem, packageID, from, to, level, id)
	fmt.Println("Run 'tillit publish' to push it to your registered servers.")
	return nil
}

// parseLevelReason extracts --level (required) and --reason (optional)
// from the trailing args of a sign subcommand.
func parseLevelReason(args []string) (models.DecisionLevel, string, error) {
	level := ""
	reason := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--level":
			if i+1 >= len(args) {
				return "", "", fmt.Errorf("--level requires a value")
			}
			i++
			level = args[i]
		case "--reason":
			if i+1 >= len(args) {
				return "", "", fmt.Errorf("--reason requires a value")
			}
			i++
			reason = args[i]
		default:
			return "", "", fmt.Errorf("unknown flag: %s", args[i])
		}
	}
	if level == "" {
		return "", "", fmt.Errorf("--level is required (allowed, vetted, or rejected)")
	}
	return models.DecisionLevel(level), reason, nil
}

// signAndCache signs payload and writes the resulting CachedSignature.
// Returns the new signature ID.
func signAndCache(s *localstore.Store, signer tillit_crypto.Signer, userID string, payload *models.Payload) (string, error) {
	signed, err := signPayload(signer, payload)
	if err != nil {
		return "", err
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
		return "", fmt.Errorf("failed saving signature: %w", err)
	}
	return signed.ID, nil
}

// adapterForEcosystem returns the first registered adapter whose
// Ecosystem() matches name. The "ok" return is false if no adapter
// claims that ecosystem (sign still proceeds; ordering won't be checked).
func adapterForEcosystem(name string) (ecosystems.Adapter, bool) {
	for _, a := range adapters {
		if a.Ecosystem() == name {
			return a, true
		}
	}
	return nil, false
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
	id, err := signAndCache(s, signer, userID, payload)
	if err != nil {
		return fmt.Errorf("failed saving revocation: %w", err)
	}

	now := time.Now().UTC()
	if existing, err := s.GetCachedSignature(targetID); err == nil {
		existing.Revoked = true
		existing.RevokedAt = &now
		if err := s.SaveCachedSignature(existing); err != nil {
			fmt.Printf("warning: failed marking target %s revoked locally: %v\n", targetID, err)
		}
	}

	fmt.Printf("Revoked %s (id: %s)\n", targetID, id)
	fmt.Println("Run 'tillit publish' to push it to your registered servers.")
	return nil
}

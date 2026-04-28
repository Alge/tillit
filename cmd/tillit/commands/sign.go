package commands

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

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
	if err := payload.Validate(); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	sigBytes, err := signer.Sign(payloadBytes)
	if err != nil {
		return fmt.Errorf("signing failed: %w", err)
	}

	req := sigUploadRequest{
		Payload:   string(payloadBytes),
		Algorithm: signer.Algorithm(),
		Sig:       base64.RawURLEncoding.EncodeToString(sigBytes),
	}

	servers, err := s.ListServers()
	if err != nil {
		return fmt.Errorf("failed listing servers: %w", err)
	}
	if len(servers) == 0 {
		return fmt.Errorf("no servers registered — run 'tillit register <server_url>' first")
	}

	for _, srv := range servers {
		result, err := uploadSignature(srv.URL, userID, req)
		if err != nil {
			fmt.Printf("  [%s] failed: %v\n", srv.URL, err)
			continue
		}
		fmt.Printf("Published to %s (id: %s)\n", srv.URL, result.ID)
	}
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

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	sigBytes, err := signer.Sign(payloadBytes)
	if err != nil {
		return fmt.Errorf("signing failed: %w", err)
	}

	req := sigUploadRequest{
		Payload:   string(payloadBytes),
		Algorithm: signer.Algorithm(),
		Sig:       base64.RawURLEncoding.EncodeToString(sigBytes),
	}

	servers, err := s.ListServers()
	if err != nil {
		return fmt.Errorf("failed listing servers: %w", err)
	}
	if len(servers) == 0 {
		return fmt.Errorf("no servers registered — run 'tillit register <server_url>' first")
	}

	for _, srv := range servers {
		result, err := uploadSignature(srv.URL, userID, req)
		if err != nil {
			fmt.Printf("  [%s] failed: %v\n", srv.URL, err)
			continue
		}
		fmt.Printf("Revocation published to %s (id: %s)\n", srv.URL, result.ID)
	}
	return nil
}

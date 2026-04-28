package models_test

import (
	"encoding/json"
	"testing"

	"github.com/Alge/tillit/models"
)

func TestParsePayload_Decision(t *testing.T) {
	raw := `{
		"type": "decision",
		"signer": "abc123",
		"ecosystem": "go",
		"package_id": "github.com/foo/bar",
		"version": "v1.2.3",
		"level": "vetted",
		"reason": "code looks good"
	}`

	p, err := models.ParsePayload([]byte(raw))
	if err != nil {
		t.Fatalf("ParsePayload failed: %v", err)
	}
	if p.Type != models.PayloadTypeDecision {
		t.Errorf("Type = %q, want %q", p.Type, models.PayloadTypeDecision)
	}
	if p.Level != models.DecisionVetted {
		t.Errorf("Level = %q, want %q", p.Level, models.DecisionVetted)
	}
	if p.PackageID != "github.com/foo/bar" {
		t.Errorf("PackageID = %q", p.PackageID)
	}
}

func TestParsePayload_Revocation(t *testing.T) {
	raw := `{"type":"revocation","signer":"abc123","target_id":"sig-uuid-1"}`
	p, err := models.ParsePayload([]byte(raw))
	if err != nil {
		t.Fatalf("ParsePayload failed: %v", err)
	}
	if !p.IsRevocation() {
		t.Error("expected IsRevocation() = true")
	}
	if p.TargetID != "sig-uuid-1" {
		t.Errorf("TargetID = %q, want %q", p.TargetID, "sig-uuid-1")
	}
}

func TestParsePayload_InvalidJSON(t *testing.T) {
	_, err := models.ParsePayload([]byte("not json"))
	if err == nil {
		t.Error("expected error on invalid JSON")
	}
}

func TestPayload_Validate_Decision(t *testing.T) {
	p := &models.Payload{
		Type:      models.PayloadTypeDecision,
		Signer:    "abc123",
		Ecosystem: "go",
		PackageID: "github.com/foo/bar",
		Version:   "v1.2.3",
		Level:     models.DecisionVetted,
	}
	if err := p.Validate(); err != nil {
		t.Errorf("Validate() returned unexpected error: %v", err)
	}
}

func TestPayload_Validate_MissingFields(t *testing.T) {
	cases := []struct {
		name string
		p    models.Payload
	}{
		{"missing signer", models.Payload{Type: models.PayloadTypeDecision, Ecosystem: "go", PackageID: "x", Version: "v1", Level: models.DecisionAllowed}},
		{"missing ecosystem", models.Payload{Type: models.PayloadTypeDecision, Signer: "s", PackageID: "x", Version: "v1", Level: models.DecisionAllowed}},
		{"missing package_id", models.Payload{Type: models.PayloadTypeDecision, Signer: "s", Ecosystem: "go", Version: "v1", Level: models.DecisionAllowed}},
		{"missing version", models.Payload{Type: models.PayloadTypeDecision, Signer: "s", Ecosystem: "go", PackageID: "x", Level: models.DecisionAllowed}},
		{"missing level", models.Payload{Type: models.PayloadTypeDecision, Signer: "s", Ecosystem: "go", PackageID: "x", Version: "v1"}},
		{"invalid level", models.Payload{Type: models.PayloadTypeDecision, Signer: "s", Ecosystem: "go", PackageID: "x", Version: "v1", Level: "supervetted"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.p.Validate(); err == nil {
				t.Error("expected Validate() to return an error")
			}
		})
	}
}

func TestPayload_Validate_Revocation(t *testing.T) {
	p := &models.Payload{
		Type:     models.PayloadTypeRevocation,
		Signer:   "abc123",
		TargetID: "sig-uuid-1",
	}
	if err := p.Validate(); err != nil {
		t.Errorf("Validate() returned unexpected error: %v", err)
	}
}

func TestPayload_Validate_RevocationMissingTarget(t *testing.T) {
	p := &models.Payload{
		Type:   models.PayloadTypeRevocation,
		Signer: "abc123",
	}
	if err := p.Validate(); err == nil {
		t.Error("expected Validate() to return error for missing target_id")
	}
}

func TestPayload_RoundTrip(t *testing.T) {
	p := &models.Payload{
		Type:      models.PayloadTypeDecision,
		Signer:    "abc123",
		Ecosystem: "npm",
		PackageID: "lodash",
		Version:   "4.17.21",
		Level:     models.DecisionAllowed,
		Reason:    "pinned and reviewed",
	}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	got, err := models.ParsePayload(b)
	if err != nil {
		t.Fatalf("ParsePayload failed: %v", err)
	}
	if got.Reason != p.Reason || got.Ecosystem != p.Ecosystem {
		t.Errorf("round-trip mismatch: got %+v", got)
	}
}

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

func TestParsePayload_Connection(t *testing.T) {
	raw := `{
		"type": "connection",
		"signer": "abc123",
		"other_id": "def456",
		"public": true,
		"trust": true,
		"trust_extends": 2
	}`
	p, err := models.ParsePayload([]byte(raw))
	if err != nil {
		t.Fatalf("ParsePayload failed: %v", err)
	}
	if p.Type != models.PayloadTypeConnection {
		t.Errorf("Type = %q, want %q", p.Type, models.PayloadTypeConnection)
	}
	if p.OtherID != "def456" {
		t.Errorf("OtherID = %q", p.OtherID)
	}
	if !p.Public || !p.Trust {
		t.Error("expected Public and Trust to be true")
	}
	if p.TrustExtends != 2 {
		t.Errorf("TrustExtends = %d, want 2", p.TrustExtends)
	}
}

func TestPayload_Validate_Connection(t *testing.T) {
	p := &models.Payload{
		Type:    models.PayloadTypeConnection,
		Signer:  "abc123",
		OtherID: "def456",
		Trust:   true,
	}
	if err := p.Validate(); err != nil {
		t.Errorf("Validate() unexpected error: %v", err)
	}
}

func TestPayload_Validate_ConnectionMissingOther(t *testing.T) {
	p := &models.Payload{Type: models.PayloadTypeConnection, Signer: "abc123"}
	if err := p.Validate(); err == nil {
		t.Error("expected error for connection payload missing other_id")
	}
}

func TestPayload_Validate_ConnectionRevocation(t *testing.T) {
	p := &models.Payload{
		Type:     models.PayloadTypeConnectionRevocation,
		Signer:   "abc123",
		TargetID: "conn-uuid-1",
	}
	if err := p.Validate(); err != nil {
		t.Errorf("Validate() unexpected error: %v", err)
	}
}

func TestPayload_Validate_ConnectionRevocationMissingTarget(t *testing.T) {
	p := &models.Payload{Type: models.PayloadTypeConnectionRevocation, Signer: "abc123"}
	if err := p.Validate(); err == nil {
		t.Error("expected error for connection_revocation missing target_id")
	}
}

func TestPayload_Validate_DeltaDecision(t *testing.T) {
	p := &models.Payload{
		Type:        models.PayloadTypeDeltaDecision,
		Signer:      "abc",
		Ecosystem:   "go",
		PackageID:   "github.com/foo/bar",
		FromVersion: "v1.0.0",
		ToVersion:   "v1.1.0",
		Level:       models.DecisionVetted,
	}
	if err := p.Validate(); err != nil {
		t.Errorf("Validate() unexpected error: %v", err)
	}
}

func TestPayload_Validate_DeltaSelfReferential(t *testing.T) {
	p := &models.Payload{
		Type:        models.PayloadTypeDeltaDecision,
		Signer:      "abc",
		Ecosystem:   "go",
		PackageID:   "github.com/foo/bar",
		FromVersion: "v1.0.0",
		ToVersion:   "v1.0.0",
		Level:       models.DecisionVetted,
	}
	if err := p.Validate(); err == nil {
		t.Error("expected error for diff with from_version == to_version")
	}
}

func TestPayload_Validate_DeltaMissingFields(t *testing.T) {
	cases := []models.Payload{
		{Type: models.PayloadTypeDeltaDecision, Signer: "s", Ecosystem: "go", PackageID: "p", ToVersion: "v1.1", Level: models.DecisionVetted},                  // no from
		{Type: models.PayloadTypeDeltaDecision, Signer: "s", Ecosystem: "go", PackageID: "p", FromVersion: "v1.0", Level: models.DecisionVetted},                // no to
		{Type: models.PayloadTypeDeltaDecision, Signer: "s", PackageID: "p", FromVersion: "v1.0", ToVersion: "v1.1", Level: models.DecisionVetted},              // no ecosystem
		{Type: models.PayloadTypeDeltaDecision, Signer: "s", Ecosystem: "go", FromVersion: "v1.0", ToVersion: "v1.1", Level: models.DecisionVetted},             // no package_id
		{Type: models.PayloadTypeDeltaDecision, Signer: "s", Ecosystem: "go", PackageID: "p", FromVersion: "v1.0", ToVersion: "v1.1", Level: "supervetted"},     // bad level
		{Type: models.PayloadTypeDeltaDecision, Signer: "s", Ecosystem: "go", PackageID: "p", FromVersion: "v1.0", ToVersion: "v1.1"},                           // no level
	}
	for i, p := range cases {
		if err := p.Validate(); err == nil {
			t.Errorf("case %d: expected error, got none for %+v", i, p)
		}
	}
}

func TestParsePayload_DeltaDecisionRoundTrip(t *testing.T) {
	raw := `{
		"type": "delta_decision",
		"signer": "abc",
		"ecosystem": "go",
		"package_id": "p",
		"from_version": "v1.0.0",
		"to_version": "v1.1.0",
		"level": "vetted"
	}`
	p, err := models.ParsePayload([]byte(raw))
	if err != nil {
		t.Fatalf("ParsePayload failed: %v", err)
	}
	if p.FromVersion != "v1.0.0" || p.ToVersion != "v1.1.0" || p.Type != models.PayloadTypeDeltaDecision {
		t.Errorf("unexpected diff payload: %+v", p)
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

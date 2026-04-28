package models

import (
	"encoding/json"
	"fmt"
)

type PayloadType string

const (
	PayloadTypeDecision             PayloadType = "decision"
	PayloadTypeRevocation           PayloadType = "revocation"
	PayloadTypeConnection           PayloadType = "connection"
	PayloadTypeConnectionRevocation PayloadType = "connection_revocation"
)

type DecisionLevel string

const (
	DecisionAllowed  DecisionLevel = "allowed"
	DecisionVetted   DecisionLevel = "vetted"
	DecisionRejected DecisionLevel = "rejected"
)

var validLevels = map[DecisionLevel]bool{
	DecisionAllowed:  true,
	DecisionVetted:   true,
	DecisionRejected: true,
}

type Payload struct {
	Type      PayloadType   `json:"type"`
	Signer    string        `json:"signer"`
	Ecosystem string        `json:"ecosystem,omitempty"`
	PackageID string        `json:"package_id,omitempty"`
	Version   string        `json:"version,omitempty"`
	Level     DecisionLevel `json:"level,omitempty"`
	Reason    string        `json:"reason,omitempty"`
	TargetID  string        `json:"target_id,omitempty"`

	// connection fields
	OtherID      string `json:"other_id,omitempty"`
	Public       bool   `json:"public,omitempty"`
	Trust        bool   `json:"trust,omitempty"`
	TrustExtends int    `json:"trust_extends,omitempty"`
}

func ParsePayload(data []byte) (*Payload, error) {
	var p Payload
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("invalid payload JSON: %w", err)
	}
	return &p, nil
}

func (p *Payload) IsRevocation() bool {
	return p.Type == PayloadTypeRevocation
}

func (p *Payload) Validate() error {
	if p.Signer == "" {
		return fmt.Errorf("signer is required")
	}
	switch p.Type {
	case PayloadTypeDecision:
		if p.Ecosystem == "" {
			return fmt.Errorf("ecosystem is required for decision payloads")
		}
		if p.PackageID == "" {
			return fmt.Errorf("package_id is required for decision payloads")
		}
		if p.Version == "" {
			return fmt.Errorf("version is required for decision payloads")
		}
		if !validLevels[p.Level] {
			return fmt.Errorf("level must be one of: allowed, vetted, rejected; got %q", p.Level)
		}
	case PayloadTypeRevocation:
		if p.TargetID == "" {
			return fmt.Errorf("target_id is required for revocation payloads")
		}
	case PayloadTypeConnection:
		if p.OtherID == "" {
			return fmt.Errorf("other_id is required for connection payloads")
		}
	case PayloadTypeConnectionRevocation:
		if p.TargetID == "" {
			return fmt.Errorf("target_id is required for connection_revocation payloads")
		}
	default:
		return fmt.Errorf("unknown payload type %q", p.Type)
	}
	return nil
}

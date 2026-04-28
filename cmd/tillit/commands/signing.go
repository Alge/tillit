package commands

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	tillit_crypto "github.com/Alge/tillit/crypto"
	"github.com/Alge/tillit/models"
	"github.com/google/uuid"
)

// signedItem is the result of signing a payload: the marshalled payload,
// algorithm, base64url signature, and a freshly-minted UUID. Used to populate
// localstore cache rows.
type signedItem struct {
	ID        string
	Payload   string
	Algorithm string
	Sig       string
}

func signPayload(signer tillit_crypto.Signer, p *models.Payload) (*signedItem, error) {
	if err := p.Validate(); err != nil {
		return nil, fmt.Errorf("invalid payload: %w", err)
	}
	payloadBytes, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	sigBytes, err := signer.Sign(payloadBytes)
	if err != nil {
		return nil, fmt.Errorf("signing failed: %w", err)
	}
	return &signedItem{
		ID:        uuid.NewString(),
		Payload:   string(payloadBytes),
		Algorithm: signer.Algorithm(),
		Sig:       base64.RawURLEncoding.EncodeToString(sigBytes),
	}, nil
}

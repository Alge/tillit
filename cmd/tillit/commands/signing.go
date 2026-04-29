package commands

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	tillit_crypto "github.com/Alge/tillit/crypto"
	"github.com/Alge/tillit/models"
)

// signedItem is the result of signing a payload: the marshalled payload,
// algorithm, base64url signature, and the content-addressed signature ID.
// Used to populate localstore cache rows.
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
	payload := string(payloadBytes)
	sig := base64.RawURLEncoding.EncodeToString(sigBytes)
	return &signedItem{
		ID:        models.SignatureID(payload, sig),
		Payload:   payload,
		Algorithm: signer.Algorithm(),
		Sig:       sig,
	}, nil
}

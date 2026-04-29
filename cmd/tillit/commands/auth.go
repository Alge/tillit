package commands

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	tillit_crypto "github.com/Alge/tillit/crypto"
	"github.com/Alge/tillit/models"
)

// mintAuthHeader returns a value suitable for the Authorization
// header on an authenticated request to serverURL. The token is
// freshly signed each call — its lifetime is bounded by
// MaxAuthTokenLifetime on the server. serverURL must match exactly
// what the server has configured as its PublicURL (no trailing slash
// is added or stripped).
func mintAuthHeader(signer tillit_crypto.Signer, userID, serverURL string) (string, error) {
	now := time.Now().UTC()
	tok := models.AuthToken{
		Type:   models.AuthTokenType,
		Signer: userID,
		Server: serverURL,
		IAT:    now.Format(time.RFC3339),
		// Half of the server's max — leaves room for a retry.
		EXP: now.Add(models.MaxAuthTokenLifetime / 2).Format(time.RFC3339),
	}
	payloadBytes, err := json.Marshal(tok)
	if err != nil {
		return "", err
	}
	sigBytes, err := signer.Sign(payloadBytes)
	if err != nil {
		return "", fmt.Errorf("sign auth token: %w", err)
	}
	envelope := struct {
		Payload   string `json:"payload"`
		Algorithm string `json:"algorithm"`
		Sig       string `json:"sig"`
	}{
		Payload:   string(payloadBytes),
		Algorithm: signer.Algorithm(),
		Sig:       base64.RawURLEncoding.EncodeToString(sigBytes),
	}
	envBytes, err := json.Marshal(envelope)
	if err != nil {
		return "", err
	}
	return "Tillit " + base64.RawURLEncoding.EncodeToString(envBytes), nil
}

// withAuth attaches a freshly-minted auth header to req. The serverURL
// is derived from the request's scheme+host so the token's `server`
// field matches what the server checks against.
func withAuth(req *http.Request, signer tillit_crypto.Signer, userID string) error {
	serverURL := strings.TrimRight(req.URL.Scheme+"://"+req.URL.Host, "/")
	header, err := mintAuthHeader(signer, userID, serverURL)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", header)
	return nil
}

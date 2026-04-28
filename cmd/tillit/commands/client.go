package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Alge/tillit/models"
)

type sigUploadRequest struct {
	Payload   string `json:"payload"`
	Algorithm string `json:"algorithm"`
	Sig       string `json:"sig"`
}

func uploadSignature(serverURL, userID string, req sigUploadRequest) (*models.Signature, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	resp, err := http.Post(
		fmt.Sprintf("%s/v1/users/%s/signatures", serverURL, userID),
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("server returned %s", resp.Status)
	}
	var sig models.Signature
	if err := json.NewDecoder(resp.Body).Decode(&sig); err != nil {
		return nil, fmt.Errorf("failed decoding response: %w", err)
	}
	return &sig, nil
}

func fetchUserSignatures(serverURL, userID string, since *time.Time) ([]*models.Signature, error) {
	url := fmt.Sprintf("%s/v1/users/%s/signatures", serverURL, userID)
	if since != nil {
		url += "?since=" + since.UTC().Format(time.RFC3339)
	}
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned %s", resp.Status)
	}
	var sigs []*models.Signature
	if err := json.NewDecoder(resp.Body).Decode(&sigs); err != nil {
		return nil, fmt.Errorf("failed decoding response: %w", err)
	}
	return sigs, nil
}

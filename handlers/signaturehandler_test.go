package handlers_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Alge/tillit/crypto"
	"github.com/Alge/tillit/db/sqliteconnector"
	"github.com/Alge/tillit/handlers"
	"github.com/Alge/tillit/models"
)

func newTestDB(t *testing.T) *sqliteconnector.SqliteConnector {
	t.Helper()
	c, err := sqliteconnector.Init(":memory:")
	if err != nil {
		t.Fatalf("failed creating test db: %v", err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func createTestUser(t *testing.T, db *sqliteconnector.SqliteConnector) (*models.User, crypto.Signer) {
	t.Helper()
	signer, err := crypto.NewEd25519Signer()
	if err != nil {
		t.Fatalf("failed creating signer: %v", err)
	}
	u, err := models.NewUserFromSigner("alice", signer)
	if err != nil {
		t.Fatalf("failed creating user: %v", err)
	}
	if err := db.CreateUser(u); err != nil {
		t.Fatalf("failed storing user: %v", err)
	}
	return u, signer
}

type sigRequest struct {
	Payload   string `json:"payload"`
	Algorithm string `json:"algorithm"`
	Sig       string `json:"sig"`
}

func signPayload(t *testing.T, signer crypto.Signer, payload string) sigRequest {
	t.Helper()
	sigBytes, err := signer.Sign([]byte(payload))
	if err != nil {
		t.Fatalf("failed signing: %v", err)
	}
	return sigRequest{
		Payload:   payload,
		Algorithm: signer.Algorithm(),
		Sig:       base64.RawURLEncoding.EncodeToString(sigBytes),
	}
}

func TestCreateSignatureHandler_Success(t *testing.T) {
	db := newTestDB(t)
	u, signer := createTestUser(t, db)

	payload := `{"type":"vetted","package":"example@1.0.0"}`
	body, _ := json.Marshal(signPayload(t, signer, payload))

	req := httptest.NewRequest(http.MethodPost, "/v1/users/"+u.ID+"/signatures", bytes.NewReader(body))
	req.SetPathValue("id", u.ID)
	w := httptest.NewRecorder()

	handlers.CreateSignatureHandler(db)(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var got models.Signature
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("failed decoding response: %v", err)
	}
	if got.ID == "" {
		t.Error("expected non-empty ID in response")
	}
	if got.Signer != u.ID {
		t.Errorf("signer = %q, want %q", got.Signer, u.ID)
	}
	if got.UploadedAt.IsZero() {
		t.Error("expected non-zero UploadedAt")
	}
}

func TestCreateSignatureHandler_UserNotFound(t *testing.T) {
	db := newTestDB(t)
	signer, _ := crypto.NewEd25519Signer()

	payload := `{"type":"vetted"}`
	body, _ := json.Marshal(signPayload(t, signer, payload))

	req := httptest.NewRequest(http.MethodPost, "/v1/users/nonexistent/signatures", bytes.NewReader(body))
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()

	handlers.CreateSignatureHandler(db)(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestCreateSignatureHandler_BadSignature(t *testing.T) {
	db := newTestDB(t)
	u, _ := createTestUser(t, db)

	// Sign with a different key — should fail verification
	otherSigner, _ := crypto.NewEd25519Signer()
	payload := `{"type":"vetted"}`
	body, _ := json.Marshal(signPayload(t, otherSigner, payload))

	req := httptest.NewRequest(http.MethodPost, "/v1/users/"+u.ID+"/signatures", bytes.NewReader(body))
	req.SetPathValue("id", u.ID)
	w := httptest.NewRecorder()

	handlers.CreateSignatureHandler(db)(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestGetUserSignaturesHandler_Success(t *testing.T) {
	db := newTestDB(t)
	u, signer := createTestUser(t, db)

	payload := `{"type":"vetted","package":"foo@1.0.0"}`
	body, _ := json.Marshal(signPayload(t, signer, payload))

	createReq := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	createReq.SetPathValue("id", u.ID)
	handlers.CreateSignatureHandler(db)(httptest.NewRecorder(), createReq)

	req := httptest.NewRequest(http.MethodGet, "/v1/users/"+u.ID+"/signatures", nil)
	req.SetPathValue("id", u.ID)
	w := httptest.NewRecorder()

	handlers.GetUserSignaturesHandler(db)(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var sigs []*models.Signature
	if err := json.NewDecoder(w.Body).Decode(&sigs); err != nil {
		t.Fatalf("failed decoding response: %v", err)
	}
	if len(sigs) != 1 {
		t.Errorf("expected 1 signature, got %d", len(sigs))
	}
}

func TestGetUserSignaturesHandler_Since(t *testing.T) {
	db := newTestDB(t)
	u, signer := createTestUser(t, db)

	// Upload an old signature directly into DB
	oldSig := &models.Signature{
		ID:         "old-sig",
		Signer:     u.ID,
		Payload:    `{"type":"vetted"}`,
		Algorithm:  "ed25519",
		Sig:        "fakesig",
		UploadedAt: time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Second),
	}
	if err := db.CreateSignature(oldSig); err != nil {
		t.Fatalf("CreateSignature failed: %v", err)
	}

	// Upload a recent one via the handler
	payload := `{"type":"vetted","package":"bar@2.0.0"}`
	body, _ := json.Marshal(signPayload(t, signer, payload))
	createReq := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	createReq.SetPathValue("id", u.ID)
	handlers.CreateSignatureHandler(db)(httptest.NewRecorder(), createReq)

	cutoff := time.Now().UTC().Add(-1 * time.Hour).Format(time.RFC3339)
	req := httptest.NewRequest(http.MethodGet, "/v1/users/"+u.ID+"/signatures?since="+cutoff, nil)
	req.SetPathValue("id", u.ID)
	w := httptest.NewRecorder()

	handlers.GetUserSignaturesHandler(db)(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var sigs []*models.Signature
	json.NewDecoder(w.Body).Decode(&sigs)
	if len(sigs) != 1 {
		t.Errorf("expected 1 signature after cutoff, got %d", len(sigs))
	}
}

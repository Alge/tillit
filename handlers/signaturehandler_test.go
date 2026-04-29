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
	ID        string `json:"id"`
	Payload   string `json:"payload"`
	Algorithm string `json:"algorithm"`
	Sig       string `json:"sig"`
}

// decisionPayload builds a minimal valid decision payload for tests so
// the server-side payload.Validate() check is satisfied.
func decisionPayload(signerID, pkg, version string) string {
	return `{"type":"decision","signer":"` + signerID + `","ecosystem":"go","package_id":"` + pkg + `","version":"` + version + `","level":"vetted"}`
}

func signPayload(t *testing.T, signer crypto.Signer, payload string) sigRequest {
	t.Helper()
	sigBytes, err := signer.Sign([]byte(payload))
	if err != nil {
		t.Fatalf("failed signing: %v", err)
	}
	sig := base64.RawURLEncoding.EncodeToString(sigBytes)
	return sigRequest{
		ID:        models.SignatureID(payload, sig),
		Payload:   payload,
		Algorithm: signer.Algorithm(),
		Sig:       sig,
	}
}

func TestCreateSignatureHandler_Success(t *testing.T) {
	db := newTestDB(t)
	u, signer := createTestUser(t, db)

	payload := decisionPayload(u.ID, "example", "v1.0.0")
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

func TestCreateSignatureHandler_RejectsInvalidPayload(t *testing.T) {
	db := newTestDB(t)
	u, signer := createTestUser(t, db)

	// Decision payload missing required version + ecosystem fields.
	bogus := `{"type":"decision","signer":"` + u.ID + `","level":"vetted"}`
	body, _ := json.Marshal(signPayload(t, signer, bogus))

	req := httptest.NewRequest(http.MethodPost, "/v1/users/"+u.ID+"/signatures", bytes.NewReader(body))
	req.SetPathValue("id", u.ID)
	w := httptest.NewRecorder()
	handlers.CreateSignatureHandler(db)(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for structurally-invalid payload, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateSignatureHandler_RejectsMismatchedID(t *testing.T) {
	db := newTestDB(t)
	u, signer := createTestUser(t, db)

	payload := `{"type":"vetted"}`
	req := signPayload(t, signer, payload)
	req.ID = "deadbeef" // tampered, doesn't match hash
	body, _ := json.Marshal(req)

	httpReq := httptest.NewRequest(http.MethodPost, "/v1/users/"+u.ID+"/signatures", bytes.NewReader(body))
	httpReq.SetPathValue("id", u.ID)
	w := httptest.NewRecorder()
	handlers.CreateSignatureHandler(db)(w, httpReq)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for mismatched ID, got %d: %s", w.Code, w.Body.String())
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

	payload := decisionPayload(u.ID, "foo", "v1.0.0")
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
	payload := decisionPayload(u.ID, "bar", "v2.0.0")
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

func uploadDecision(t *testing.T, db *sqliteconnector.SqliteConnector, u *models.User, signer crypto.Signer, pkg string) string {
	t.Helper()
	payload := `{"type":"decision","signer":"` + u.ID + `","ecosystem":"go","package_id":"` + pkg + `","version":"v1.0.0","level":"vetted"}`
	body, _ := json.Marshal(signPayload(t, signer, payload))
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.SetPathValue("id", u.ID)
	w := httptest.NewRecorder()
	handlers.CreateSignatureHandler(db)(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("uploadDecision: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var sig models.Signature
	json.NewDecoder(w.Body).Decode(&sig)
	return sig.ID
}

func createSecondTestUser(t *testing.T, db *sqliteconnector.SqliteConnector, name string) (*models.User, crypto.Signer) {
	t.Helper()
	signer, err := crypto.NewEd25519Signer()
	if err != nil {
		t.Fatalf("failed creating signer: %v", err)
	}
	u, err := models.NewUserFromSigner(name, signer)
	if err != nil {
		t.Fatalf("failed creating user: %v", err)
	}
	if err := db.CreateUser(u); err != nil {
		t.Fatalf("failed storing user: %v", err)
	}
	return u, signer
}

func TestCreateSignatureHandler_RevocationRejectsForeignTarget(t *testing.T) {
	db := newTestDB(t)
	alice, aliceSigner := createTestUser(t, db)
	bob, bobSigner := createSecondTestUser(t, db, "bob")

	// Alice signs an exact decision.
	aliceSigID := uploadDecision(t, db, alice, aliceSigner, "github.com/foo/bar")

	// Bob tries to revoke Alice's signature.
	revokePayload := `{"type":"revocation","signer":"` + bob.ID + `","target_id":"` + aliceSigID + `"}`
	body, _ := json.Marshal(signPayload(t, bobSigner, revokePayload))
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.SetPathValue("id", bob.ID)
	w := httptest.NewRecorder()
	handlers.CreateSignatureHandler(db)(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}

	// Alice's signature must NOT be revoked.
	got, err := db.GetSignature(aliceSigID)
	if err != nil {
		t.Fatalf("GetSignature failed: %v", err)
	}
	if got.Revoked {
		t.Error("Alice's signature was revoked by Bob — ownership check failed")
	}
}

func TestCreateSignatureHandler_RevocationTargetNotFound(t *testing.T) {
	db := newTestDB(t)
	u, signer := createTestUser(t, db)

	revokePayload := `{"type":"revocation","signer":"` + u.ID + `","target_id":"does-not-exist"}`
	body, _ := json.Marshal(signPayload(t, signer, revokePayload))
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.SetPathValue("id", u.ID)
	w := httptest.NewRecorder()
	handlers.CreateSignatureHandler(db)(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateSignatureHandler_Revocation(t *testing.T) {
	db := newTestDB(t)
	u, signer := createTestUser(t, db)

	sigID := uploadDecision(t, db, u, signer, "github.com/foo/bar")

	// Upload revocation
	revokePayload := `{"type":"revocation","signer":"` + u.ID + `","target_id":"` + sigID + `"}`
	body, _ := json.Marshal(signPayload(t, signer, revokePayload))
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.SetPathValue("id", u.ID)
	w := httptest.NewRecorder()
	handlers.CreateSignatureHandler(db)(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("revocation upload: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	// Original signature must now be marked revoked
	got, err := db.GetSignature(sigID)
	if err != nil {
		t.Fatalf("GetSignature failed: %v", err)
	}
	if !got.Revoked {
		t.Error("expected original signature to be marked revoked")
	}
	if got.RevokedAt == nil {
		t.Error("expected RevokedAt to be set")
	}
}

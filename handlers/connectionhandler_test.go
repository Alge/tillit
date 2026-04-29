package handlers_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Alge/tillit/crypto"
	"github.com/Alge/tillit/handlers"
	"github.com/Alge/tillit/models"
)

type connRequest struct {
	ID        string `json:"id"`
	Payload   string `json:"payload"`
	Algorithm string `json:"algorithm"`
	Sig       string `json:"sig"`
}

// signConnPayload builds a connRequest with a content-addressed ID
// (sha256 of payload+sig). Server-side validation requires this to
// match exactly, so callers shouldn't set their own ID — anything
// that wants a tampered ID overrides the field on the returned struct.
func signConnPayload(t *testing.T, signer crypto.Signer, payload string) connRequest {
	t.Helper()
	sigBytes, err := signer.Sign([]byte(payload))
	if err != nil {
		t.Fatalf("failed signing: %v", err)
	}
	sig := base64.RawURLEncoding.EncodeToString(sigBytes)
	return connRequest{
		ID:        models.SignatureID(payload, sig),
		Payload:   payload,
		Algorithm: signer.Algorithm(),
		Sig:       sig,
	}
}

func TestCreateConnectionHandler_Success(t *testing.T) {
	db := newTestDB(t)
	u, signer := createTestUser(t, db)

	payload := `{"type":"connection","signer":"` + u.ID + `","other_id":"bob","public":true,"trust":true,"trust_extends":2}`
	conn := signConnPayload(t, signer, payload)
	body, _ := json.Marshal(conn)

	req := httptest.NewRequest(http.MethodPost, "/v1/users/"+u.ID+"/connections", bytes.NewReader(body))
	req.SetPathValue("id", u.ID)
	w := httptest.NewRecorder()

	handlers.CreateConnectionHandler(db)(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var got models.Connection
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if got.ID != conn.ID || got.OtherID != "bob" || !got.Public || !got.Trust || got.TrustExtends != 2 {
		t.Errorf("unexpected connection: %+v", got)
	}
}

func TestCreateConnectionHandler_RejectsMismatchedID(t *testing.T) {
	db := newTestDB(t)
	u, signer := createTestUser(t, db)

	payload := `{"type":"connection","signer":"` + u.ID + `","other_id":"bob","trust":true}`
	conn := signConnPayload(t, signer, payload)
	conn.ID = "deadbeef-not-the-hash" // tamper
	body, _ := json.Marshal(conn)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.SetPathValue("id", u.ID)
	w := httptest.NewRecorder()
	handlers.CreateConnectionHandler(db)(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for mismatched connection id, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateConnectionHandler_BadSignature(t *testing.T) {
	db := newTestDB(t)
	u, _ := createTestUser(t, db)

	other, _ := crypto.NewEd25519Signer()
	payload := `{"type":"connection","signer":"` + u.ID + `","other_id":"bob","trust":true}`
	body, _ := json.Marshal(signConnPayload(t, other, payload))

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.SetPathValue("id", u.ID)
	w := httptest.NewRecorder()
	handlers.CreateConnectionHandler(db)(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestCreateConnectionHandler_Revocation(t *testing.T) {
	db := newTestDB(t)
	u, signer := createTestUser(t, db)

	// Create
	payload := `{"type":"connection","signer":"` + u.ID + `","other_id":"bob","public":true,"trust":true}`
	conn := signConnPayload(t, signer, payload)
	body, _ := json.Marshal(conn)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.SetPathValue("id", u.ID)
	w := httptest.NewRecorder()
	handlers.CreateConnectionHandler(db)(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	// Revoke
	revPayload := `{"type":"connection_revocation","signer":"` + u.ID + `","target_id":"` + conn.ID + `"}`
	body, _ = json.Marshal(signConnPayload(t, signer, revPayload))
	req = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.SetPathValue("id", u.ID)
	w = httptest.NewRecorder()
	handlers.CreateConnectionHandler(db)(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("revoke: expected 204, got %d: %s", w.Code, w.Body.String())
	}

	got, err := db.GetConnection(conn.ID)
	if err != nil {
		t.Fatalf("GetConnection failed: %v", err)
	}
	if !got.Revoked {
		t.Error("expected connection to be marked revoked")
	}
}

func TestCreateConnectionHandler_RevocationRejectsForeignTarget(t *testing.T) {
	db := newTestDB(t)
	alice, aliceSigner := createTestUser(t, db)
	bob, bobSigner := createSecondTestUser(t, db, "bob")

	// Alice creates a connection to "carol".
	payload := `{"type":"connection","signer":"` + alice.ID + `","other_id":"carol","public":true,"trust":true}`
	conn := signConnPayload(t, aliceSigner, payload)
	body, _ := json.Marshal(conn)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.SetPathValue("id", alice.ID)
	w := httptest.NewRecorder()
	handlers.CreateConnectionHandler(db)(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("alice create: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	// Bob tries to revoke Alice's connection.
	revPayload := `{"type":"connection_revocation","signer":"` + bob.ID + `","target_id":"` + conn.ID + `"}`
	body, _ = json.Marshal(signConnPayload(t, bobSigner, revPayload))
	req = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.SetPathValue("id", bob.ID)
	w = httptest.NewRecorder()
	handlers.CreateConnectionHandler(db)(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}

	got, err := db.GetConnection(conn.ID)
	if err != nil {
		t.Fatalf("GetConnection failed: %v", err)
	}
	if got.Revoked {
		t.Error("Alice's connection was revoked by Bob — ownership check failed")
	}
}

func TestCreateConnectionHandler_RevocationTargetNotFound(t *testing.T) {
	db := newTestDB(t)
	u, signer := createTestUser(t, db)

	revPayload := `{"type":"connection_revocation","signer":"` + u.ID + `","target_id":"does-not-exist"}`
	body, _ := json.Marshal(signConnPayload(t, signer, revPayload))
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.SetPathValue("id", u.ID)
	w := httptest.NewRecorder()
	handlers.CreateConnectionHandler(db)(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetUserConnectionsHandler_PublicOnly(t *testing.T) {
	db := newTestDB(t)
	u, signer := createTestUser(t, db)

	for i, public := range []bool{true, false} {
		other := "peer-" + string(rune('a'+i))
		publicJSON := "false"
		if public {
			publicJSON = "true"
		}
		payload := `{"type":"connection","signer":"` + u.ID + `","other_id":"` + other + `","public":` + publicJSON + `,"trust":true}`
		body, _ := json.Marshal(signConnPayload(t, signer, payload))
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
		req.SetPathValue("id", u.ID)
		w := httptest.NewRecorder()
		handlers.CreateConnectionHandler(db)(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("create: expected 201, got %d: %s", w.Code, w.Body.String())
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/users/"+u.ID+"/connections", nil)
	req.SetPathValue("id", u.ID)
	w := httptest.NewRecorder()
	handlers.GetUserConnectionsHandler(db)(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var conns []*models.Connection
	json.NewDecoder(w.Body).Decode(&conns)
	if len(conns) != 1 {
		t.Errorf("expected 1 public connection, got %d", len(conns))
	}
}

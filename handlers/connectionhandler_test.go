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
	"github.com/google/uuid"
)

type connRequest struct {
	ID        string `json:"id"`
	Payload   string `json:"payload"`
	Algorithm string `json:"algorithm"`
	Sig       string `json:"sig"`
}

func signConnPayload(t *testing.T, signer crypto.Signer, id, payload string) connRequest {
	t.Helper()
	sigBytes, err := signer.Sign([]byte(payload))
	if err != nil {
		t.Fatalf("failed signing: %v", err)
	}
	return connRequest{
		ID:        id,
		Payload:   payload,
		Algorithm: signer.Algorithm(),
		Sig:       base64.RawURLEncoding.EncodeToString(sigBytes),
	}
}

func TestCreateConnectionHandler_Success(t *testing.T) {
	db := newTestDB(t)
	u, signer := createTestUser(t, db)

	connID := uuid.NewString()
	payload := `{"type":"connection","signer":"` + u.ID + `","other_id":"bob","public":true,"trust":true,"trust_extends":2}`
	body, _ := json.Marshal(signConnPayload(t, signer, connID, payload))

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
	if got.ID != connID || got.OtherID != "bob" || !got.Public || !got.Trust || got.TrustExtends != 2 {
		t.Errorf("unexpected connection: %+v", got)
	}
}

func TestCreateConnectionHandler_BadSignature(t *testing.T) {
	db := newTestDB(t)
	u, _ := createTestUser(t, db)

	other, _ := crypto.NewEd25519Signer()
	payload := `{"type":"connection","signer":"` + u.ID + `","other_id":"bob","trust":true}`
	body, _ := json.Marshal(signConnPayload(t, other, "c1", payload))

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
	connID := uuid.NewString()
	payload := `{"type":"connection","signer":"` + u.ID + `","other_id":"bob","public":true,"trust":true}`
	body, _ := json.Marshal(signConnPayload(t, signer, connID, payload))
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.SetPathValue("id", u.ID)
	w := httptest.NewRecorder()
	handlers.CreateConnectionHandler(db)(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	// Revoke
	revID := uuid.NewString()
	revPayload := `{"type":"connection_revocation","signer":"` + u.ID + `","target_id":"` + connID + `"}`
	body, _ = json.Marshal(signConnPayload(t, signer, revID, revPayload))
	req = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.SetPathValue("id", u.ID)
	w = httptest.NewRecorder()
	handlers.CreateConnectionHandler(db)(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("revoke: expected 204, got %d: %s", w.Code, w.Body.String())
	}

	got, err := db.GetConnection(connID)
	if err != nil {
		t.Fatalf("GetConnection failed: %v", err)
	}
	if !got.Revoked {
		t.Error("expected connection to be marked revoked")
	}
}

func TestGetUserConnectionsHandler_PublicOnly(t *testing.T) {
	db := newTestDB(t)
	u, signer := createTestUser(t, db)

	for i, public := range []bool{true, false} {
		connID := uuid.NewString()
		other := "peer-" + string(rune('a'+i))
		publicJSON := "false"
		if public {
			publicJSON = "true"
		}
		payload := `{"type":"connection","signer":"` + u.ID + `","other_id":"` + other + `","public":` + publicJSON + `,"trust":true}`
		body, _ := json.Marshal(signConnPayload(t, signer, connID, payload))
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

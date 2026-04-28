package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Alge/tillit/handlers"
	"github.com/Alge/tillit/models"
)

func TestCreateUserHandler_Success(t *testing.T) {
	db := newTestDB(t)

	body, _ := json.Marshal(map[string]string{
		"id":         "user-1",
		"username":   "alice",
		"public_key": "somepubkey",
		"algorithm":  "ed25519",
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/users", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handlers.CreateUserHandler(db)(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var got models.User
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("failed decoding response: %v", err)
	}
	if got.Algorithm != "ed25519" {
		t.Errorf("Algorithm = %q, want %q", got.Algorithm, "ed25519")
	}
}

func TestGetUserIDHandler_Success(t *testing.T) {
	db := newTestDB(t)
	u, _ := createTestUser(t, db)

	req := httptest.NewRequest(http.MethodGet, "/v1/users/"+u.ID, nil)
	req.SetPathValue("id", u.ID)
	w := httptest.NewRecorder()

	handlers.GetUserIDHandler(db)(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var got models.User
	json.NewDecoder(w.Body).Decode(&got)
	if got.ID != u.ID {
		t.Errorf("ID = %q, want %q", got.ID, u.ID)
	}
}

func TestGetUserIDHandler_NotFound(t *testing.T) {
	db := newTestDB(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/users/nonexistent", nil)
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()

	handlers.GetUserIDHandler(db)(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

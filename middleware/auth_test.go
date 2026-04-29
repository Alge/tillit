package middleware

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Alge/tillit/crypto"
	"github.com/Alge/tillit/db/sqliteconnector"
	"github.com/Alge/tillit/models"
	"github.com/Alge/tillit/requestdata"
)

func newTestDB(t *testing.T) *sqliteconnector.SqliteConnector {
	t.Helper()
	c, err := sqliteconnector.Init(":memory:")
	if err != nil {
		t.Fatalf("init test db: %v", err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func mkSignedTokenHeader(t *testing.T, signer crypto.Signer, tok models.AuthToken) string {
	t.Helper()
	payloadBytes, err := json.Marshal(tok)
	if err != nil {
		t.Fatalf("marshal token: %v", err)
	}
	sigBytes, err := signer.Sign(payloadBytes)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	env := signedAuthEnvelope{
		Payload:   string(payloadBytes),
		Algorithm: signer.Algorithm(),
		Sig:       base64.RawURLEncoding.EncodeToString(sigBytes),
	}
	envBytes, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal env: %v", err)
	}
	return authScheme + base64.RawURLEncoding.EncodeToString(envBytes)
}

func TestAuthenticate_Success(t *testing.T) {
	db := newTestDB(t)
	signer, _ := crypto.NewEd25519Signer()
	user, _ := models.NewUserFromSigner("alice", signer)
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("create user: %v", err)
	}

	now := time.Now().UTC()
	tok := models.AuthToken{
		Type:   models.AuthTokenType,
		Signer: user.ID,
		Server: "https://srv",
		IAT:    now.Format(time.RFC3339),
		EXP:    now.Add(2 * time.Minute).Format(time.RFC3339),
	}
	header := mkSignedTokenHeader(t, signer, tok)

	var seenUser *models.User
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, _ := requestdata.GetUser(r)
		seenUser = u
		w.WriteHeader(http.StatusOK)
	})
	h := Authenticate(next, db, "https://srv")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", header)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if seenUser == nil || seenUser.ID != user.ID {
		t.Errorf("expected user %s in context, got %+v", user.ID, seenUser)
	}
}

func TestAuthenticate_MissingHeader(t *testing.T) {
	db := newTestDB(t)
	h := Authenticate(okHandler(), db, "https://srv")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAuthenticate_BadSignature(t *testing.T) {
	db := newTestDB(t)
	signer, _ := crypto.NewEd25519Signer()
	user, _ := models.NewUserFromSigner("alice", signer)
	db.CreateUser(user)

	// Sign with a DIFFERENT key — won't verify against user's pubkey.
	otherSigner, _ := crypto.NewEd25519Signer()

	now := time.Now().UTC()
	tok := models.AuthToken{
		Type: models.AuthTokenType, Signer: user.ID, Server: "https://srv",
		IAT: now.Format(time.RFC3339), EXP: now.Add(2 * time.Minute).Format(time.RFC3339),
	}
	header := mkSignedTokenHeader(t, otherSigner, tok)

	h := Authenticate(okHandler(), db, "https://srv")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", header)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for bad sig, got %d", w.Code)
	}
}

func TestAuthenticate_Expired(t *testing.T) {
	db := newTestDB(t)
	signer, _ := crypto.NewEd25519Signer()
	user, _ := models.NewUserFromSigner("alice", signer)
	db.CreateUser(user)

	now := time.Now().UTC()
	tok := models.AuthToken{
		Type: models.AuthTokenType, Signer: user.ID, Server: "https://srv",
		IAT: now.Add(-10 * time.Minute).Format(time.RFC3339),
		EXP: now.Add(-5 * time.Minute).Format(time.RFC3339),
	}
	header := mkSignedTokenHeader(t, signer, tok)

	h := Authenticate(okHandler(), db, "https://srv")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", header)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for expired token, got %d", w.Code)
	}
}

func TestAuthenticate_UnknownSigner(t *testing.T) {
	db := newTestDB(t)
	signer, _ := crypto.NewEd25519Signer()
	user, _ := models.NewUserFromSigner("alice", signer)
	// NOT inserted into the db.

	now := time.Now().UTC()
	tok := models.AuthToken{
		Type: models.AuthTokenType, Signer: user.ID, Server: "https://srv",
		IAT: now.Format(time.RFC3339), EXP: now.Add(2 * time.Minute).Format(time.RFC3339),
	}
	header := mkSignedTokenHeader(t, signer, tok)

	h := Authenticate(okHandler(), db, "https://srv")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", header)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for unknown signer, got %d", w.Code)
	}
}

func TestAuthenticate_ServerMismatch(t *testing.T) {
	db := newTestDB(t)
	signer, _ := crypto.NewEd25519Signer()
	user, _ := models.NewUserFromSigner("alice", signer)
	db.CreateUser(user)

	now := time.Now().UTC()
	tok := models.AuthToken{
		Type: models.AuthTokenType, Signer: user.ID, Server: "https://other-server",
		IAT: now.Format(time.RFC3339), EXP: now.Add(2 * time.Minute).Format(time.RFC3339),
	}
	header := mkSignedTokenHeader(t, signer, tok)

	h := Authenticate(okHandler(), db, "https://srv")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", header)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for server mismatch, got %d", w.Code)
	}
}

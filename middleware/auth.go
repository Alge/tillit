package middleware

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/Alge/tillit/db"
	"github.com/Alge/tillit/db/dberrors"
	"github.com/Alge/tillit/models"
	"github.com/Alge/tillit/requestdata"
)

// authScheme is the prefix used in the Authorization header. The
// token following it is a base64url-encoded JSON envelope of
// signedAuthEnvelope.
const authScheme = "Tillit "

// signedAuthEnvelope is the wire shape of a tillit auth token. The
// Payload field is the JSON-encoded AuthToken; Sig is base64url of
// the raw signature bytes; Algorithm names the signature scheme so
// the verifier can be constructed without inspecting the user record.
type signedAuthEnvelope struct {
	Payload   string `json:"payload"`
	Algorithm string `json:"algorithm"`
	Sig       string `json:"sig"`
}

// Authenticate is a middleware that verifies a tillit-signed auth
// token in the Authorization header and, on success, attaches the
// authenticated user to the request context. It does NOT enforce
// authorization — callers (or downstream middleware/handlers) decide
// whether the authenticated user is allowed to touch the URL's
// resource.
//
// Failures (missing header, malformed envelope, bad signature,
// expired token, server mismatch) all map to 401 with a generic body
// to avoid leaking which check failed. Server-side logs see the
// detail for debugging.
func Authenticate(next http.Handler, database db.DatabaseConnector, expectedServer string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, err := authenticateRequest(r, database, expectedServer, time.Now().UTC())
		if err != nil {
			writeUnauthed(w)
			return
		}
		next.ServeHTTP(w, requestdata.WithUser(r, user))
	})
}

// authenticateRequest does the bulk of the work as a pure-ish
// function so it's testable without spinning up a real handler chain.
// Returns the authenticated user on success.
func authenticateRequest(r *http.Request, database db.DatabaseConnector, expectedServer string, now time.Time) (*models.User, error) {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, authScheme) {
		return nil, errors.New("missing or malformed Authorization header")
	}
	encoded := strings.TrimPrefix(header, authScheme)

	envelopeBytes, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	var env signedAuthEnvelope
	if err := json.Unmarshal(envelopeBytes, &env); err != nil {
		return nil, err
	}

	var token models.AuthToken
	if err := json.Unmarshal([]byte(env.Payload), &token); err != nil {
		return nil, err
	}
	if err := token.Validate(now, expectedServer); err != nil {
		return nil, err
	}

	user, err := database.GetUser(token.Signer)
	if err != nil {
		var notFound *dberrors.ObjectNotFoundError
		if errors.As(err, &notFound) {
			return nil, errors.New("auth token signer not registered")
		}
		return nil, err
	}

	verifier, err := user.Verifier()
	if err != nil {
		return nil, err
	}
	sigBytes, err := base64.RawURLEncoding.DecodeString(env.Sig)
	if err != nil {
		return nil, err
	}
	if !verifier.Verify([]byte(env.Payload), sigBytes) {
		return nil, errors.New("auth token signature did not verify")
	}
	return user, nil
}

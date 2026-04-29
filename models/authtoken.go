package models

import (
	"fmt"
	"time"
)

// AuthTokenType is the constant value of the Type field for an
// authentication payload, distinct from the on-trust payload types
// signed for the trust graph.
const AuthTokenType = "auth"

// AuthTokenClockSkew is how far in the future an iat may be before
// we reject the token. Covers normal NTP drift.
const AuthTokenClockSkew = 60 * time.Second

// MaxAuthTokenLifetime caps how long a single auth token may be valid
// (exp - iat). 5 minutes is enough for clock skew + a network retry
// while keeping the blast radius of a leaked token tight. Replay
// within this window is bounded but not separately tracked — clients
// should still send tokens over TLS.
const MaxAuthTokenLifetime = 5 * time.Minute

// AuthToken authenticates a request to a private endpoint as the
// signer. The client signs this struct (canonical JSON) with their
// private key; the server verifies the signature against the
// signer's pubkey and runs Validate.
//
// The fields mirror JWT (RFC 7519) names where the meaning matches
// — iat, exp — but the wire format is our own signed-JSON envelope,
// not a JWT.
type AuthToken struct {
	Type   string `json:"type"`   // always AuthTokenType
	Signer string `json:"signer"` // expected to match the URL's user id
	Server string `json:"server"` // base URL of the server we're calling
	IAT    string `json:"iat"`    // issued-at, RFC3339
	EXP    string `json:"exp"`    // expires-at, RFC3339
}

// Validate enforces all the structural and time-bound rules. It does
// NOT verify the cryptographic signature — that's the caller's job
// (the AuthToken comes packaged with a sig the caller checks against
// the user's pubkey before/after this).
func (a *AuthToken) Validate(now time.Time, expectedServer string) error {
	if a.Type != AuthTokenType {
		return fmt.Errorf("auth token has wrong type %q, expected %q", a.Type, AuthTokenType)
	}
	if a.Signer == "" {
		return fmt.Errorf("auth token signer is empty")
	}
	if a.Server != expectedServer {
		return fmt.Errorf("auth token server %q does not match request target %q", a.Server, expectedServer)
	}
	iat, err := time.Parse(time.RFC3339, a.IAT)
	if err != nil {
		return fmt.Errorf("auth token iat invalid: %w", err)
	}
	exp, err := time.Parse(time.RFC3339, a.EXP)
	if err != nil {
		return fmt.Errorf("auth token exp invalid: %w", err)
	}
	if now.After(exp) {
		return fmt.Errorf("auth token expired at %s (now %s)", exp.Format(time.RFC3339), now.Format(time.RFC3339))
	}
	if iat.After(now.Add(AuthTokenClockSkew)) {
		return fmt.Errorf("auth token iat %s is too far in the future", iat.Format(time.RFC3339))
	}
	if exp.Sub(iat) > MaxAuthTokenLifetime {
		return fmt.Errorf("auth token lifetime %s exceeds maximum %s", exp.Sub(iat), MaxAuthTokenLifetime)
	}
	if !exp.After(iat) {
		return fmt.Errorf("auth token exp %s is not after iat %s", a.EXP, a.IAT)
	}
	return nil
}

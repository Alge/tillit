package handlers

import (
	"encoding/base64"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/Alge/tillit/db"
	"github.com/Alge/tillit/db/dberrors"
	"github.com/Alge/tillit/models"
	"github.com/Alge/tillit/requestdata"
)

func processRevocation(database db.DatabaseConnector, payload *models.Payload, uploadedAt time.Time) {
	if err := database.RevokeSignature(payload.TargetID, uploadedAt); err != nil {
		log.Printf("RevokeSignature(%s) failed: %v", payload.TargetID, err)
	}
}

// authorizeSignatureRevocation enforces that a signer may only revoke
// their own signatures. Returns (notFound, forbidden, internalErr). The
// caller writes the appropriate HTTP status; non-nil internalErr means
// log and 500.
func authorizeSignatureRevocation(database db.DatabaseConnector, targetID, revokerID string) (notFound bool, forbidden bool, internalErr error) {
	target, err := database.GetSignature(targetID)
	if err != nil {
		var nf *dberrors.ObjectNotFoundError
		if errors.As(err, &nf) {
			return true, false, nil
		}
		return false, false, err
	}
	if target.Signer != revokerID {
		return false, true, nil
	}
	return false, false, nil
}

func CreateSignatureHandler(database db.DatabaseConnector) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.PathValue("id")

		user, err := database.GetUser(userID)
		if err != nil {
			var notFound *dberrors.ObjectNotFoundError
			if errors.As(err, &notFound) {
				http.NotFound(w, r)
				return
			}
			log.Printf("GetUser failed: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		type sigInput struct {
			ID        string `json:"id"`
			Payload   string `json:"payload"`
			Algorithm string `json:"algorithm"`
			Sig       string `json:"sig"`
			Public    *bool  `json:"public,omitempty"` // default true (legacy clients)
		}
		input, err := decode[sigInput](r)
		if err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if input.ID == "" {
			http.Error(w, "Missing signature id", http.StatusBadRequest)
			return
		}
		if expected := models.SignatureID(input.Payload, input.Sig); input.ID != expected {
			http.Error(w, "Signature id does not match payload+sig hash", http.StatusBadRequest)
			return
		}

		sigBytes, err := base64.RawURLEncoding.DecodeString(input.Sig)
		if err != nil {
			http.Error(w, "Invalid signature encoding", http.StatusBadRequest)
			return
		}

		verifier, err := user.Verifier()
		if err != nil {
			log.Printf("Failed creating verifier for user %s: %v", userID, err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		if !verifier.Verify([]byte(input.Payload), sigBytes) {
			http.Error(w, "Signature verification failed", http.StatusUnauthorized)
			return
		}

		payload, err := models.ParsePayload([]byte(input.Payload))
		if err != nil {
			http.Error(w, "Invalid payload JSON", http.StatusBadRequest)
			return
		}
		if err := payload.Validate(); err != nil {
			http.Error(w, "Invalid payload: "+err.Error(), http.StatusBadRequest)
			return
		}

		// Revocations must be authorised before the signature is stored:
		// the signer can only revoke their own signatures.
		if payload.IsRevocation() {
			notFound, forbidden, internalErr := authorizeSignatureRevocation(database, payload.TargetID, userID)
			if internalErr != nil {
				log.Printf("authorizeSignatureRevocation failed: %v", internalErr)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			if notFound {
				http.Error(w, "Revocation target not found", http.StatusNotFound)
				return
			}
			if forbidden {
				http.Error(w, "Cannot revoke another signer's signature", http.StatusForbidden)
				return
			}
		}

		// Default visibility is public; private uploads (the cross-device
		// mirror flow) require the request to be authenticated as the
		// signer.
		isPublic := true
		if input.Public != nil {
			isPublic = *input.Public
		}
		if !isPublic {
			authedUser, ok := requestdata.GetUser(r)
			if !ok || authedUser.ID != userID {
				http.Error(w, "Private uploads require authentication as the signer", http.StatusUnauthorized)
				return
			}
		}

		uploadedAt := time.Now().UTC()
		sig := &models.Signature{
			ID:         input.ID,
			Signer:     userID,
			Payload:    input.Payload,
			Algorithm:  input.Algorithm,
			Sig:        input.Sig,
			UploadedAt: uploadedAt,
			Public:     isPublic,
		}
		if err := database.CreateSignature(sig); err != nil {
			log.Printf("CreateSignature failed: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		if payload.IsRevocation() {
			processRevocation(database, payload, uploadedAt)
		}

		encode(w, r, http.StatusCreated, sig)
	}
}

func GetUserSignaturesHandler(database db.DatabaseConnector) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.PathValue("id")

		var since *time.Time
		if sinceStr := r.URL.Query().Get("since"); sinceStr != "" {
			t, err := time.Parse(time.RFC3339, sinceStr)
			if err != nil {
				http.Error(w, "Invalid 'since' parameter, expected RFC3339", http.StatusBadRequest)
				return
			}
			since = &t
		}

		// Authenticated owners see their full set (public + private);
		// everyone else gets only the public rows.
		includePrivate := false
		if u, ok := requestdata.GetUser(r); ok && u.ID == userID {
			includePrivate = true
		}
		sigs, err := database.GetUserSignatures(userID, since, includePrivate)
		if err != nil {
			log.Printf("GetUserSignatures failed: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		if sigs == nil {
			sigs = []*models.Signature{}
		}
		encode(w, r, http.StatusOK, sigs)
	}
}

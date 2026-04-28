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
	"github.com/google/uuid"
)

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
			Payload   string `json:"payload"`
			Algorithm string `json:"algorithm"`
			Sig       string `json:"sig"`
		}
		input, err := decode[sigInput](r)
		if err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
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

		sig := &models.Signature{
			ID:         uuid.NewString(),
			Signer:     userID,
			Payload:    input.Payload,
			Algorithm:  input.Algorithm,
			Sig:        input.Sig,
			UploadedAt: time.Now().UTC(),
		}
		if err := database.CreateSignature(sig); err != nil {
			log.Printf("CreateSignature failed: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
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

		sigs, err := database.GetUserSignatures(userID, since)
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

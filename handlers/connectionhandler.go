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
)

type connInput struct {
	ID        string `json:"id"`
	Payload   string `json:"payload"`
	Algorithm string `json:"algorithm"`
	Sig       string `json:"sig"`
}

func CreateConnectionHandler(database db.DatabaseConnector) http.HandlerFunc {
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

		input, err := decode[connInput](r)
		if err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if input.ID == "" {
			http.Error(w, "Missing connection id", http.StatusBadRequest)
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
		if payload.Signer != userID {
			http.Error(w, "Payload signer does not match URL", http.StatusBadRequest)
			return
		}

		now := time.Now().UTC()
		switch payload.Type {
		case models.PayloadTypeConnection:
			conn := &models.Connection{
				ID:           input.ID,
				Owner:        userID,
				OtherID:      payload.OtherID,
				Public:       payload.Public,
				Trust:        payload.Trust,
				TrustExtends: payload.TrustExtends,
				Payload:      input.Payload,
				Algorithm:    input.Algorithm,
				Sig:          input.Sig,
				CreatedAt:    now,
			}
			if err := database.CreateConnection(conn); err != nil {
				log.Printf("CreateConnection failed: %v", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			encode(w, r, http.StatusCreated, conn)

		case models.PayloadTypeConnectionRevocation:
			target, err := database.GetConnection(payload.TargetID)
			if err != nil {
				var notFound *dberrors.ObjectNotFoundError
				if errors.As(err, &notFound) {
					http.NotFound(w, r)
					return
				}
				log.Printf("GetConnection failed: %v", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			if target.Owner != userID {
				http.Error(w, "Cannot revoke another user's connection", http.StatusForbidden)
				return
			}
			if err := database.RevokeConnection(payload.TargetID, now); err != nil {
				var notFound *dberrors.ObjectNotFoundError
				if errors.As(err, &notFound) {
					http.NotFound(w, r)
					return
				}
				log.Printf("RevokeConnection failed: %v", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusNoContent)

		default:
			http.Error(w, "Payload type must be connection or connection_revocation", http.StatusBadRequest)
		}
	}
}

func GetUserConnectionsHandler(database db.DatabaseConnector) http.HandlerFunc {
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

		conns, err := database.GetUserPublicConnections(userID, since)
		if err != nil {
			log.Printf("GetUserPublicConnections failed: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if conns == nil {
			conns = []*models.Connection{}
		}
		encode(w, r, http.StatusOK, conns)
	}
}

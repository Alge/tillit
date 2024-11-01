package requestdata

import (
	"context"
	"net/http"

	"github.com/Alge/tillit/models"
	"github.com/google/uuid"
)

type contextKey int

const (
	userKey      contextKey = iota
	requestIDKey contextKey = iota
)

func WithRequestID(r *http.Request, id uuid.UUID) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), requestIDKey, id))
}

func GetRequestID(r *http.Request) (id uuid.UUID, ok bool) {
	id, ok = r.Context().Value(requestIDKey).(uuid.UUID)
	return
}

func WithUser(r *http.Request, user *models.User) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), userKey, user))
}

func GetUser(r *http.Request) (user *models.User, ok bool) {
	user, ok = r.Context().Value(userKey).(*models.User)
	return
}

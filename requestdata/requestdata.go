package requestdata

import (
	"context"

	"github.com/Alge/tillit/db"
	"github.com/Alge/tillit/models"
	"github.com/google/uuid"
)

type RequestData struct {
	User      *models.User
	DB        *db.DatabaseConnector
	RequestID *uuid.UUID
}

func NewRequestData() (rd *RequestData, err error) {
	id, err := uuid.NewRandom()

	if err != nil {
		return
	}

	rd.RequestID = &id

	return
}

func WithRequestID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, "requestID", id)
}

func GetRequestID(ctx context.Context) (id *uuid.UUID, ok bool) {
	id, ok = ctx.Value("requestID").(*uuid.UUID)
	return
}

func WithUser(ctx context.Context, user *models.User) context.Context {
	return context.WithValue(ctx, "user", user)
}

func GetUser(ctx context.Context) (user *models.User, ok bool) {
	user, ok = ctx.Value("user").(*models.User)
	return
}

func WithDatabase(ctx context.Context, db *db.DatabaseConnector) context.Context {
	return context.WithValue(ctx, "database", db)
}

func GetDatabase(ctx context.Context) (user *db.DatabaseConnector, ok bool) {
	user, ok = ctx.Value("database").(*db.DatabaseConnector)
	return
}

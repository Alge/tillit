package api

import (
	"net/http"

	"github.com/Alge/tillit/config"
	"github.com/Alge/tillit/db"
	"github.com/Alge/tillit/handlers"
	"github.com/Alge/tillit/middleware"
)

func addRoutes(
	mux *http.ServeMux,
	cfg config.Config,
	database db.DatabaseConnector,
) {
	mux.HandleFunc("POST /v1/users", handlers.CreateUserHandler(database))
	mux.HandleFunc("GET /v1/users/{id}", handlers.GetUserIDHandler(database))

	// Signature and connection endpoints accept optional authentication.
	// Unauthenticated requests get the public view; an authenticated
	// owner additionally sees their private rows and is allowed to
	// upload private rows.
	auth := func(h http.HandlerFunc) http.Handler {
		return middleware.Authenticate(h, database, cfg.Server.PublicURL)
	}
	mux.Handle("POST /v1/users/{id}/signatures", auth(handlers.CreateSignatureHandler(database)))
	mux.Handle("GET /v1/users/{id}/signatures", auth(handlers.GetUserSignaturesHandler(database)))
	mux.Handle("POST /v1/users/{id}/connections", auth(handlers.CreateConnectionHandler(database)))
	mux.Handle("GET /v1/users/{id}/connections", auth(handlers.GetUserConnectionsHandler(database)))

	mux.HandleFunc("/health", handlers.HandleHealth())
	mux.Handle("/", http.NotFoundHandler())
}

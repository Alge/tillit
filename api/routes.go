package api

import (
	"net/http"

	"github.com/Alge/tillit/config"
	"github.com/Alge/tillit/db"
	"github.com/Alge/tillit/handlers"
)

func addRoutes(
	mux *http.ServeMux,
	cfg config.Config,
	database db.DatabaseConnector,
) {
	mux.HandleFunc("POST /v1/users", handlers.CreateUserHandler(database))
	mux.HandleFunc("GET /v1/users/{id}", handlers.GetUserIDHandler(database))
	mux.HandleFunc("POST /v1/users/{id}/signatures", handlers.CreateSignatureHandler(database))
	mux.HandleFunc("GET /v1/users/{id}/signatures", handlers.GetUserSignaturesHandler(database))
	mux.HandleFunc("POST /v1/users/{id}/connections", handlers.CreateConnectionHandler(database))
	mux.HandleFunc("GET /v1/users/{id}/connections", handlers.GetUserConnectionsHandler(database))

	mux.HandleFunc("/health", handlers.HandleHealth())
	mux.Handle("/", http.NotFoundHandler())
}

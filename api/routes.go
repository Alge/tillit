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
	mux.HandleFunc("POST /api/v1/users", handlers.CreateUserHandler(database))
	mux.HandleFunc("GET /api/v1/users/", handlers.GetUserListHandler(database))
	mux.HandleFunc("GET /api/v1/users/{id}", handlers.GetUserIDHandler(database))

	mux.HandleFunc("/healthz", handlers.HandleHealthzPlease())
	mux.Handle("/", http.NotFoundHandler())
}

package api

import (
	"net/http"
	"time"

	"github.com/Alge/tillit/config"
	"github.com/Alge/tillit/db"
	"github.com/Alge/tillit/middleware"
	"github.com/go-chi/httprate"
)

func NewServer(
	cfg *config.Config,
	database db.DatabaseConnector,
) (h http.Handler) {

	mux := http.NewServeMux()

	// Register routes
	addRoutes(
		mux,
		*cfg,
		database,
	)

	// Add these middleware functions to all requests. Reverse execution order
	var handler http.Handler = mux
	handler = middleware.Auth(handler, database) // Load the user (if present) into the context
	handler = httprate.LimitByRealIP(            // Rate limiter
		cfg.Ratelimit.RequestLimit,
		time.Duration(cfg.Ratelimit.WindowLength)*time.Second,
	)(handler)
	handler = middleware.Logging(handler)      // Request logger
	handler = middleware.AddRequestID(handler) // Add a unique request ID to the context
	return handler

}

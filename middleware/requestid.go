package middleware

import (
	"log"
	"net/http"

	"github.com/Alge/tillit/requestdata"
	"github.com/google/uuid"
)

func AddRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		rID, err := uuid.NewRandom()
		if err != nil {
			log.Printf("Failed generating UUID: %w", err)
		}
		r = requestdata.WithRequestID(r, rID)

		next.ServeHTTP(w, r)
	})
}

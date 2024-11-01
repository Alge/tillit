package handlers

import (
	"log"
	"net/http"
)

func HandleHealthzPlease() func(w http.ResponseWriter, r *http.Request) {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			type healthResponse struct {
				Status string `json:"status"`
			}
			s := healthResponse{
				Status: "OK",
			}

			err := encode(w, r, 200, s)
			if err != nil {
				log.Printf("Failed encoding and returning healthResponse %w", err)
			}
		},
	)
}

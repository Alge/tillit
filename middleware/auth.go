package middleware

import (
	"net/http"

	"github.com/Alge/tillit/db"
)

func Auth(next http.Handler, database db.DatabaseConnector) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		/*

			authorization := r.Header.Get("Authorization")

			// Check that the header begins with a prefix of Bearer
			if !strings.HasPrefix(authorization, "Bearer ") {
				log.Printf("Invalid auth header: '%s'", authorization)
				writeUnauthed(w)
				return
			}

			// Pull out the token
			encodedToken := strings.TrimPrefix(authorization, "Bearer ")

			// Decode the token from base 64
			token, err := base64.StdEncoding.DecodeString(encodedToken)
			if err != nil {
				log.Println("Failed decoding base64 header")
				writeUnauthed(w)
				return
			}

			// We're just assuming a valid base64 token is a valid user id.
			userID := string(token)
			log.Println("userID:", userID)

		*/

		next.ServeHTTP(w, r)
	})
}

package middleware

import (
	"net/http"

	"github.com/Alge/tillit/db"
	"github.com/Alge/tillit/requestdata"
)

func IsLoggedIn(next http.Handler, database db.DatabaseConnector) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		_, ok := requestdata.GetUser(r)

		if !ok {
			writeUnauthed(w)
			return
		}

		next.ServeHTTP(w, r)
	})
}

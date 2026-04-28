package middleware

import (
	"net/http"

	"github.com/Alge/tillit/db"
	"github.com/Alge/tillit/requestdata"
)

func IsAdmin(next http.Handler, database db.DatabaseConnector) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := requestdata.GetUser(r)
		if !ok || !user.IsAdmin {
			writeUnauthed(w)
			return
		}
		next.ServeHTTP(w, r)
	})
}

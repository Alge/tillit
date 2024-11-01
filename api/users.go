package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Alge/tillit/db"
	"github.com/Alge/tillit/models"
	"github.com/Alge/tillit/requestdata"
)

func CreateUser(rd *requestdata.RequestData) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("New user will be created, hopefully"))
		u, err := models.NewUser("Alge")
		if err != nil {
			fmt.Fprintf(w, "Failed creating user")
		}
		rd.DB.CreateUser(u)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(u)
	}

}

func GetUserID(DB db.DatabaseConnector) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		w.Write([]byte("Hello there: " + id))
	}
}

package handlers

import (
	"log"
	"net/http"

	"github.com/Alge/tillit/db"
	"github.com/Alge/tillit/models"
)

func CreateUserHandler(database db.DatabaseConnector) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		type NewUserInput struct {
			ID        string `json:"id"`
			Username  string `json:"username"`
			PubKey    string `json:"public_key"`
			Algorithm string `json:"algorithm"`
		}

		userInput, err := decode[NewUserInput](r)
		if err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		u := &models.User{
			ID:        userInput.ID,
			Username:  userInput.Username,
			PubKey:    userInput.PubKey,
			Algorithm: userInput.Algorithm,
		}

		if err := database.CreateUser(u); err != nil {
			log.Printf("Failed inserting user into database: %s", err)
			http.Error(w, "Failed creating user", http.StatusInternalServerError)
			return
		}

		encode(w, r, http.StatusCreated, u)
	}
}

func GetUserIDHandler(database db.DatabaseConnector) func(w http.ResponseWriter, r *http.Request) {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			uID := r.PathValue("id")

			user, err := database.GetUser(uID)
			if err != nil {
				http.NotFound(w, r)
				return
			}

			encode(w, r, http.StatusOK, user)
		},
	)
}


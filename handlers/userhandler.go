package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/Alge/tillit/db"
	"github.com/Alge/tillit/models"
)

func CreateUserHandler(database db.DatabaseConnector) func(w http.ResponseWriter, r *http.Request) {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			type NewUserInput struct {
				ID     string `json:"id"`
				Username  string `json:"username"`
				Publickey string `json:"public_key"`
			}

			userInput, err := decode[NewUserInput](r)
			if err != nil {
				http.Error(w, "Invalid request body", http.StatusBadRequest)
				return
			}

			u := &models.User{
				ID:       userInput.ID,
				Username: userInput.Username,
				PubKey:   userInput.Publickey,
			}

			if err := database.CreateUser(u); err != nil {
				log.Printf("Failed inserting user into database: %s", err)
				http.Error(w, "Failed creating user", http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(u)
		},
	)
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

func GetUserListHandler(database db.DatabaseConnector) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		const (
			defaultPage = 1
			defaultSize = 10
			maxSize     = 100
		)

		query := r.URL.Query()

		page := defaultPage
		if pageStr := query.Get("page"); pageStr != "" {
			p, err := strconv.Atoi(pageStr)
			if err != nil || p < 1 {
				http.Error(w, "Invalid 'page' parameter", http.StatusBadRequest)
				return
			}
			page = p
		}

		size := defaultSize
		if sizeStr := query.Get("size"); sizeStr != "" {
			s, err := strconv.Atoi(sizeStr)
			if err != nil || s < 1 {
				http.Error(w, "Invalid 'size' parameter", http.StatusBadRequest)
				return
			}
			if s > maxSize {
				size = maxSize
			} else {
				size = s
			}
		}

		response, err := database.GetUserList(page, size)
		if err != nil {
			log.Printf("Error fetching users: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		encode(w, r, http.StatusOK, response)
	}
}

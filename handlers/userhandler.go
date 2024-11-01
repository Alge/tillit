package handlers

import (
	"encoding/json"
	"fmt"
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
				Username  string `json:"username"`
				Publickey string `json:"public_key"`
			}

			userInput, err := decode[NewUserInput](r)

			if err != nil {
				log.Println("Invalid data")
			}

			log.Printf("Creating new user: %s, pubkey: %s", userInput.Username, userInput.Publickey)

			u, err := models.NewUser(userInput.Username, userInput.Publickey)
			if err != nil {
				fmt.Fprintf(w, "Failed creating user: %s", err)
			}

			// Validate the public key
			key, err := u.GetPublicKey()
			if err != nil {
				fmt.Fprintf(w, "Invalid pulic key: %s", err)
				return
			}
			log.Printf("Pubkey: %s", key)

			// Store the user in the database
			err = database.CreateUser(u)
			if err != nil {
				log.Printf("Failed inserting user into database: %s", err)
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

			log.Printf("Trying to find user with id: '%s'", uID)

			user, err := database.GetUser(uID)
			if err != nil {
				log.Printf("Could not find user")
				http.NotFound(w, r)
				return
			}

			encode(w, r, 200, user)

		},
	)
}

func GetUserListHandler(database db.DatabaseConnector) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// Default pagination values
		const (
			defaultPage = 1
			defaultSize = 10
			maxSize     = 100
		)

		// Parse query parameters
		query := r.URL.Query()

		// Parse 'page' parameter
		pageStr := query.Get("page")
		page := defaultPage
		if pageStr != "" {
			p, err := strconv.Atoi(pageStr)
			if err != nil || p < 1 {
				http.Error(w, "Invalid 'page' parameter. It must be a positive integer.", http.StatusBadRequest)
				return
			}
			page = p
		}

		// Parse 'size' parameter
		sizeStr := query.Get("size")
		size := defaultSize
		if sizeStr != "" {
			s, err := strconv.Atoi(sizeStr)
			if err != nil || s < 1 {
				http.Error(w, "Invalid 'size' parameter. It must be a positive integer.", http.StatusBadRequest)
				return
			}
			if s > maxSize {
				size = maxSize
			} else {
				size = s
			}
		}

		// Retrieve users and total count from the database
		response, err := database.GetUserList(page, size)
		if err != nil {
			log.Printf("Error fetching users: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Encode the response
		encode(w, r, http.StatusOK, response)
	}
}

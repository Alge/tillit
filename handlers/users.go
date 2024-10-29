package handlers

import (
	"net/http"
)

func CreateUser(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("New user will be created, hopefully"))
}

func GetUserID(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	w.Write([]byte("Hello there: " + id))
}

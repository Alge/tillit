package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/Alge/tillit/config"
	"github.com/Alge/tillit/db"
	"github.com/Alge/tillit/handlers"
	"github.com/Alge/tillit/middleware"
	_ "github.com/Alge/tillit/models"
)

var DB db.DatabaseConnector

func main() {
	log.Println("Starting up")

	log.Println("Loading config")

	conf, err := config.LoadConfig("config.toml")
	if err != nil {
		log.Fatal("Failed loading config: ", err)
	}

	log.Println(conf)

	log.Printf("Initializing %s database: %s", conf.Database.Type, conf.Database.DSN)

	switch conf.Database.Type {
	case "sqlite":
		if db, err := db.Init("sqlite3", conf.Database.DSN); err != nil {
			log.Fatal("Failed initializing database: ", err)
		} else {
			DB = db
		}

	default:
		log.Fatal(fmt.Sprintf("Don't know how to initialize a '%s' database", conf.Database.Type))
	}

	// Close the database when we are done
	defer DB.Close()

	router := http.NewServeMux()

	loggedInRouter := http.NewServeMux()
	loggedInRouter.HandleFunc("GET /users/{userid}", handlers.GetUserID(DB))

	router.Handle("/", loggedInRouter)

	/*
		rateLimiter := httprate.Limit(
			conf.Ratelimit.RequestLimit,
			time.Duration(conf.Ratelimit.WindowLength)*time.Second,
			httprate.WithResponseHeaders(httprate.ResponseHeaders{
				Limit:      "X-RateLimit-Limit",
				Remaining:  "X-RateLimit-Remaining",
				Reset:      "X-RateLimit-Reset",
				RetryAfter: "Retry-After",
				Increment:  "", // omit
			}),
		)
	*/

	middlewareStack, err := middleware.CreateStack(
		middleware.Logging,
		//rateLimiter,
		middleware.Auth,
	)

	server := http.Server{
		Addr:    conf.Server.HostName + ":" + strconv.Itoa(conf.Server.Port),
		Handler: handlers.AdaptHandler(middlewareStack(router.ServeHTTP())),
	}

	log.Printf("Starting up server at '%s:%d'", conf.Server.HostName, conf.Server.Port)
	if err := server.ListenAndServe(); err != nil {
		log.Fatal("Listen server failed with error: ", err)
	}

}

package main

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/httprate"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Alge/tillit/db"
	//"github.com/Alge/tillit/models"
	"github.com/Alge/tillit/config"
	"github.com/Alge/tillit/handlers"
	"github.com/Alge/tillit/middleware"
)

func main() {
	log.Println("Starting up")

	log.Println("Loading config")

	conf, err := config.LoadConfig("config.toml")
	if err != nil {
		log.Fatal("Failed loading config: ", err)
	}

	log.Println(conf)

	/*
		  log.Print("Initializing database")
			//database, err := db.NewSQLiteDatabase(":memory:")
			//database, err := db.NewSQLiteDatabase("./test.db")
			if err != nil {
				log.Fatalf("Failed to initialize database: %v", err)
			}
		  log.Print("Database initialization done")
	*/

	log.Println("Initializing database")

	db.Init(sqlite.Open("test.db"), &gorm.Config{})

	router := http.NewServeMux()

	//router.HandleFunc("POST /register", handlers.RegisterUser)

	loggedInRouter := http.NewServeMux()
	loggedInRouter.HandleFunc("GET /users/{userid}", handlers.GetUserID)
	//loggedInRouter.HandleFunc("POST /keys", handlers.AddPublicKey)

	router.Handle("/", loggedInRouter)
	//router.Handle("/", middleware.IsLoggedIn(loggedInRouter))

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

	middlewareStack := middleware.CreateStack(
		middleware.Logging,
		rateLimiter,
		middleware.Auth,
	)

	server := http.Server{
		Addr:    conf.Server.HostName + ":" + strconv.Itoa(conf.Server.Port),
		Handler: middlewareStack(router),
	}

	log.Printf("Starting up server at '%s:%d'", conf.Server.HostName, conf.Server.Port)
	if err := server.ListenAndServe(); err != nil {
		log.Fatal("Listen server failed with error: ", err)
	}

}

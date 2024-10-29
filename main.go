package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/httprate"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Alge/tillit/config"
	"github.com/Alge/tillit/db"
	"github.com/Alge/tillit/handlers"
	"github.com/Alge/tillit/middleware"
	"github.com/Alge/tillit/models"
)

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
		if err := db.Init(sqlite.Open(conf.Database.DSN), &gorm.Config{}); err != nil {
			log.Fatal("Failed initializing database: ", err)
		}

	default:
		log.Fatal(fmt.Sprintf("Don't know how to initialize a '%s' database", conf.Database.Type))
	}

	// Some tests

	// Create 2 users

	/*
	  u1, _ := models.NewUser()
	  u2, _ := models.NewUser()
	  u3, _ := models.NewUser()
	  u4, _ := models.NewUser()

	  res := db.DB.Create(u1)
	  log.Println(u1.ID, res.RowsAffected)
	  db.DB.Create(u2)
	  db.DB.Create(u3)
	  db.DB.Create(u4)
	*/

	var u1 models.User
	var u2 models.User
	var u3 models.User
	var u4 models.User

	db.DB.First(&u1)
  db.DB.Model(&u1).Association("Connections").Find(&u1.Connections)
  if res := db.DB.Preload("Connections").Preload("PubKeys").First(&u2, "id=?", "a3047569-12e1-4ab9-a6f1-eac460272e4a"); res.Error != nil{
    log.Fatal(res.Error)
  }
  db.DB.First(&u3, "id=?", "979c2bae-a8f3-47fd-82ff-c1dbcb1769fd")
	db.DB.Last(&u4)

	log.Println(u1)
	log.Println(u2)
	log.Println(u3)
	log.Println(u4)

  log.Println("")

  var users []models.User
  db.DB.Preload("Connections").Find(&users)
  for _, user := range users{
    log.Println(user)
  }

  /*	
  // Create some connections
  c1, _ := u2.Connect(&u4, true, true, 10)
  db.DB.Create(c1)
	log.Println(c1)
  */

	log.Fatal("Done")
	// End of the tests

	router := http.NewServeMux()

	loggedInRouter := http.NewServeMux()
	loggedInRouter.HandleFunc("GET /users/{userid}", handlers.GetUserID)

	router.Handle("/", loggedInRouter)

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

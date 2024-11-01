package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"time"

	"github.com/Alge/tillit/api"
	"github.com/Alge/tillit/config"
	"github.com/Alge/tillit/db"
)

var DB db.DatabaseConnector

func run(ctx context.Context) error {

	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt)
	defer cancel()

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
		log.Fatalf("Don't know how to initialize a '%s' database", conf.Database.Type)
	}

	log.Println("Done initializing database")

	// Close the database when we are done
	defer DB.Close()

	srv := api.NewServer(conf, DB)

	httpServer := &http.Server{
		Addr:    net.JoinHostPort(conf.Server.HostName, strconv.Itoa(conf.Server.Port)),
		Handler: srv,
	}

	go func() {
		log.Printf("Starting up server at 'http://%s'", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "error listening and serving: %s\n", err)
		}
	}()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()
		shutdownCtx := context.Background()
		shutdownCtx, cancel := context.WithTimeout(shutdownCtx, 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			fmt.Fprintf(os.Stderr, "error shutting down http server: %s\n", err)
		}
	}()
	wg.Wait()
	return nil
}

func main() {
	ctx := context.Background()
	if err := run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}

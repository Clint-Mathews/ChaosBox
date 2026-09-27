package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Clint-Mathews/chaosbox/apps/store/database"
	restapi "github.com/Clint-Mathews/chaosbox/apps/store/rest-api"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://chaosbox:chaosbox@localhost:5432/chaosbox?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := database.Open(ctx, databaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := db.Prepare(ctx); err != nil {
		return fmt.Errorf("prepare database: %w", err)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	address := ":" + port
	log.Printf("server listening on %s", address)
	server := http.Server{
		Addr:              address,
		Handler:           restapi.NewHandler(db),
		ReadHeaderTimeout: 5 * time.Second,
	}
	if err := server.ListenAndServe(); err != nil {
		return fmt.Errorf("serve HTTP: %w", err)
	}

	return nil
}

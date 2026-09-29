package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Clint-Mathews/chaosbox/apps/store/database"
	restapi "github.com/Clint-Mathews/chaosbox/apps/store/rest-api"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

func main() {
	logger := newLogger()
	if err := run(logger); err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func run(loggers ...*slog.Logger) error {
	logger := newLogger()
	if len(loggers) > 0 && loggers[0] != nil {
		logger = loggers[0]
	}
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
	logger.Info("server listening", "address", address)
	registry := prometheus.NewRegistry()
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		database.NewPoolCollector(db),
	)
	server := http.Server{
		Addr:              address,
		Handler:           restapi.NewInstrumentedHandler(db, logger, registry),
		ReadHeaderTimeout: 5 * time.Second,
	}
	if err := server.ListenAndServe(); err != nil {
		return fmt.Errorf("serve HTTP: %w", err)
	}

	return nil
}

func newLogger() *slog.Logger {
	level := new(slog.LevelVar)
	if err := level.UnmarshalText([]byte(strings.ToLower(os.Getenv("LOG_LEVEL")))); err != nil {
		level.Set(slog.LevelInfo)
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}

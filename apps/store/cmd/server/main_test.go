package main

import (
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestRunRejectsInvalidDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "://invalid")

	err := run()
	if err == nil {
		t.Fatal("expected invalid database URL to fail")
	}
	if !strings.Contains(err.Error(), "create database pool") {
		t.Fatalf("expected pool creation error, got %v", err)
	}
}

func TestNewLoggerLevel(t *testing.T) {
	tests := []struct {
		name         string
		value        string
		debugEnabled bool
		errorEnabled bool
	}{
		{name: "default info", errorEnabled: true},
		{name: "debug", value: "debug", debugEnabled: true, errorEnabled: true},
		{name: "case insensitive", value: "WARN", errorEnabled: true},
		{name: "invalid defaults to info", value: "verbose", errorEnabled: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("LOG_LEVEL", test.value)
			logger := newLogger()
			if got := logger.Enabled(context.Background(), slog.LevelDebug); got != test.debugEnabled {
				t.Fatalf("expected debug enabled=%t, got %t", test.debugEnabled, got)
			}
			if got := logger.Enabled(context.Background(), slog.LevelError); got != test.errorEnabled {
				t.Fatalf("expected error enabled=%t, got %t", test.errorEnabled, got)
			}
		})
	}
}

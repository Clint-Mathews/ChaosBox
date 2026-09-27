package main

import (
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

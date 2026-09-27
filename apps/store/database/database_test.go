package database

import (
	"context"
	"strings"
	"testing"
)

func TestOpenRejectsInvalidDatabaseURL(t *testing.T) {
	_, err := Open(context.Background(), "://invalid")
	if err == nil {
		t.Fatal("expected invalid database URL to fail")
	}
	if !strings.Contains(err.Error(), "create database pool") {
		t.Fatalf("expected pool creation context, got %v", err)
	}
}

func TestOpenHonorsCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := Open(ctx, "postgres://chaosbox:chaosbox@localhost:5432/chaosbox?sslmode=disable")
	if err == nil {
		t.Fatal("expected canceled context to fail")
	}
	if !strings.Contains(err.Error(), "ping database") {
		t.Fatalf("expected ping context, got %v", err)
	}
}

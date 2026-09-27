APP_DIR := apps/store
COMPOSE := docker compose -f $(APP_DIR)/compose.yaml
GO := go -C $(APP_DIR)

.PHONY: help dev run db-up db-down db-status db-logs db-shell fmt test test-race vet check smoke

help:
	@printf '%s\n' \
		'make dev         Start PostgreSQL and run the Go server' \
		'make run         Run the Go server' \
		'make db-up       Start PostgreSQL and wait until healthy' \
		'make db-down     Stop PostgreSQL without deleting data' \
		'make db-status   Show PostgreSQL container status' \
		'make db-logs     Follow PostgreSQL logs' \
		'make db-shell    Open psql in the PostgreSQL container' \
		'make fmt         Format all Go packages' \
		'make test        Run all Go tests' \
		'make test-race   Run all Go tests with the race detector' \
		'make vet         Run go vet' \
		'make check       Run tests, race tests, and vet' \
		'make smoke       Check health and products on a running server'

dev: db-up
	$(GO) run ./cmd/server

run:
	$(GO) run ./cmd/server

db-up:
	$(COMPOSE) up -d --wait postgres

db-down:
	$(COMPOSE) down

db-status:
	$(COMPOSE) ps

db-logs:
	$(COMPOSE) logs -f postgres

db-shell:
	$(COMPOSE) exec postgres psql -U chaosbox -d chaosbox

fmt:
	$(GO) fmt ./...

test:
	$(GO) test ./...

test-race:
	$(GO) test -race ./...

vet:
	$(GO) vet ./...

check: test test-race vet

smoke:
	curl --fail --silent --show-error http://localhost:8080/health
	curl --fail --silent --show-error http://localhost:8080/products

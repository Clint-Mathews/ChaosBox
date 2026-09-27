APP_DIR := apps/store
COMPOSE := docker compose -f $(APP_DIR)/compose.yaml
GO := go -C $(APP_DIR)
STORE_IMAGE ?= chaosbox-store:local

.PHONY: help dev run image-build db-up db-down db-status db-logs db-shell fmt test test-race test-e2e vet check smoke

help:
	@printf '%s\n' \
		'make dev         Start PostgreSQL and run the Go server' \
		'make run         Run the Go server' \
		'make image-build Build the store container image' \
		'make db-up       Start PostgreSQL and wait until healthy' \
		'make db-down     Stop PostgreSQL without deleting data' \
		'make db-status   Show PostgreSQL container status' \
		'make db-logs     Follow PostgreSQL logs' \
		'make db-shell    Open psql in the PostgreSQL container' \
		'make fmt         Format all Go packages' \
		'make test        Run all Go tests' \
		'make test-race   Run all Go tests with the race detector' \
		'make test-e2e    Run the Testcontainers end-to-end flow' \
		'make vet         Run go vet' \
		'make check       Run unit, race, E2E, and vet checks' \
		'make smoke       Check health and products on a running server'

dev: db-up
	$(GO) run ./cmd/server

run:
	$(GO) run ./cmd/server

image-build:
	docker build --file $(APP_DIR)/Dockerfile --tag $(STORE_IMAGE) $(APP_DIR)

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

test-e2e:
	$(GO) test -tags=integration ./e2e -v -count=1

vet:
	$(GO) vet ./...

check: test test-race test-e2e vet

smoke:
	curl --fail --silent --show-error http://localhost:8080/health
	curl --fail --silent --show-error http://localhost:8080/products

APP_DIR := apps/store
COMPOSE := docker compose -f $(APP_DIR)/compose.yaml
GO := go -C $(APP_DIR)
STORE_IMAGE ?= chaosbox-store:local
MINIKUBE_PROFILE ?= chaosbox
KUBE_CONTEXT ?= $(MINIKUBE_PROFILE)
KUBE_NAMESPACE ?= chaosbox
MINIKUBE_DIR := deploy/minikube
MINIKUBE_IMAGE ?= ghcr.io/clint-mathews/chaosbox-store:latest

.PHONY: help dev run image-build db-up db-down db-status db-logs db-shell minikube-start minikube-build minikube-deploy minikube-status minikube-url minikube-logs minikube-clean minikube-stop fmt test test-race test-e2e vet check smoke

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
		'make minikube-start  Start the ChaosBox Minikube cluster' \
		'make minikube-build  Build the store image inside Minikube' \
		'make minikube-deploy Deploy PostgreSQL and the store' \
		'make minikube-status Show Minikube workloads and storage' \
		'make minikube-url    Print the store URL' \
		'make minikube-logs   Follow store logs' \
		'make minikube-clean  Delete ChaosBox workloads and data' \
		'make minikube-stop   Stop the Minikube cluster' \
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

minikube-start:
	minikube start --profile $(MINIKUBE_PROFILE) --driver=docker --alsologtostderr -v=1
	minikube addons enable metrics-server --profile $(MINIKUBE_PROFILE)

minikube-build:
	minikube image build --profile $(MINIKUBE_PROFILE) --tag $(MINIKUBE_IMAGE) --file $(APP_DIR)/Dockerfile $(APP_DIR)

minikube-deploy:
	kubectl --context $(KUBE_CONTEXT) apply -k $(MINIKUBE_DIR)
	kubectl --context $(KUBE_CONTEXT) --namespace $(KUBE_NAMESPACE) rollout status statefulset/postgres --timeout=180s
	kubectl --context $(KUBE_CONTEXT) --namespace $(KUBE_NAMESPACE) rollout status deployment/store --timeout=180s

minikube-status:
	kubectl --context $(KUBE_CONTEXT) --namespace $(KUBE_NAMESPACE) get pods,services,persistentvolumeclaims

minikube-url:
	minikube service store --profile $(MINIKUBE_PROFILE) --namespace $(KUBE_NAMESPACE) --url

minikube-logs:
	kubectl --context $(KUBE_CONTEXT) --namespace $(KUBE_NAMESPACE) logs deployment/store --follow

minikube-clean:
	kubectl --context $(KUBE_CONTEXT) delete -k $(MINIKUBE_DIR) --ignore-not-found=true

minikube-stop:
	minikube stop --profile $(MINIKUBE_PROFILE)

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

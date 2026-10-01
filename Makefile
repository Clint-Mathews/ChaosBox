APP_DIR := apps/store
COMPOSE := docker compose -f $(APP_DIR)/compose.yaml
GO := go -C $(APP_DIR)
PYTHON ?= python3
STORE_IMAGE ?= chaosbox-store:local
MINIKUBE_PROFILE ?= chaosbox
KUBE_CONTEXT ?= $(MINIKUBE_PROFILE)
KUBE_NAMESPACE ?= chaosbox
MINIKUBE_DIR := deploy/minikube
MINIKUBE_IMAGE ?= ghcr.io/clint-mathews/chaosbox-store:latest
HELM ?= helm
MONITORING_DIR := $(MINIKUBE_DIR)/monitoring
MONITORING_NAMESPACE ?= monitoring
MONITORING_RELEASE ?= monitoring
MONITORING_CHART := oci://ghcr.io/prometheus-community/charts/kube-prometheus-stack
MONITORING_CHART_VERSION ?= 91.8.1
LOKI_RELEASE ?= loki
LOKI_CHART := oci://ghcr.io/grafana-community/helm-charts/loki
LOKI_CHART_VERSION ?= 18.13.7
ALLOY_RELEASE ?= alloy
ALLOY_CHART := grafana/alloy
ALLOY_CHART_VERSION ?= 1.13.0
GRAFANA_LOCAL_PORT ?= 3000
PROMETHEUS_LOCAL_PORT ?= 9090
LOKI_LOCAL_PORT ?= 3100
LOAD_BASE_URL ?= http://127.0.0.1/api
GRAFANA_URL ?= http://127.0.0.1/grafana
LOAD_USERS ?= 100
LOAD_RAMP_SECONDS ?= 100
LOAD_HOLD_SECONDS ?= 60
PPROF_LOCAL_PORT ?= 6060
PROFILE_SECONDS ?= 30
PROFILE_DIR ?= artifacts/profiles

.PHONY: help dev run image-build db-up db-down db-status db-logs db-shell minikube-start minikube-build minikube-deploy minikube-status minikube-url minikube-tunnel minikube-forward minikube-logs minikube-db-logs minikube-db-shell minikube-db-clear minikube-previous-logs minikube-events minikube-top minikube-k9s minikube-clean minikube-stop monitoring-install monitoring-status monitoring-grafana-password monitoring-grafana-forward monitoring-prometheus-forward monitoring-loki-forward monitoring-load-dashboard monitoring-clean profile-forward profile-capture profile-cpu profile-heap profile-allocs profile-goroutine load-smoke load-test load-results fmt test test-race test-e2e vet check smoke

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
		'make minikube-url    Print the Nginx gateway URLs' \
		'make minikube-tunnel Make the ingress IP reachable from the host' \
		'make minikube-forward Forward the store to localhost:8081' \
		'make minikube-logs   Follow store logs' \
		'make minikube-db-logs Follow PostgreSQL logs' \
		'make minikube-db-shell Open psql in the Minikube PostgreSQL pod' \
		'make minikube-db-clear Delete orders and reset their identity' \
		'make minikube-previous-logs Show previous store container logs' \
		'make minikube-events Show namespace events in chronological order' \
		'make minikube-top    Continuously show pod and container usage' \
		'make minikube-k9s    Open k9s in the application namespace' \
		'make minikube-clean  Delete ChaosBox workloads and data' \
		'make minikube-stop   Stop the Minikube cluster' \
		'make monitoring-install Install or update Prometheus and Grafana' \
		'make monitoring-status Show monitoring workloads and store discovery' \
		'make monitoring-grafana-password Print the Grafana admin password' \
		'make monitoring-grafana-forward Forward Grafana to localhost:3000' \
		'make monitoring-prometheus-forward Forward Prometheus to localhost:9090' \
		'make monitoring-loki-forward Forward Loki to localhost:3100' \
		'make monitoring-load-dashboard Print the load-test dashboard URL' \
		'make monitoring-clean Remove monitoring resources and Helm release' \
		'make profile-forward Forward the private pprof listener to localhost:6060' \
		'make profile-capture Capture CPU, heap, allocation, and goroutine profiles' \
		'make load-smoke      Run one external Python user journey' \
		'make load-test       Ramp the external Python load to 100 users' \
		'make load-results    Print the load-test artifact location' \
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
	minikube addons enable ingress --profile $(MINIKUBE_PROFILE)

minikube-build:
	minikube image build --profile $(MINIKUBE_PROFILE) --tag $(MINIKUBE_IMAGE) --file Dockerfile $(APP_DIR)

minikube-deploy:
	kubectl --context $(KUBE_CONTEXT) apply -k $(MINIKUBE_DIR)
	kubectl --context $(KUBE_CONTEXT) --namespace $(KUBE_NAMESPACE) rollout status statefulset/postgres --timeout=180s
	kubectl --context $(KUBE_CONTEXT) --namespace $(KUBE_NAMESPACE) rollout status deployment/store --timeout=180s

minikube-status:
	kubectl --context $(KUBE_CONTEXT) --namespace $(KUBE_NAMESPACE) get pods,services,ingresses,persistentvolumeclaims

minikube-url:
	@printf '%s\n' \
		'Store:      http://127.0.0.1/api' \
		'Grafana:    http://127.0.0.1/grafana/' \
		'Prometheus: http://127.0.0.1/prometheus/'

minikube-tunnel:
	minikube tunnel --profile $(MINIKUBE_PROFILE)

minikube-forward:
	kubectl --context $(KUBE_CONTEXT) --namespace $(KUBE_NAMESPACE) port-forward service/store 8081:8080

minikube-logs:
	kubectl --context $(KUBE_CONTEXT) --namespace $(KUBE_NAMESPACE) logs deployment/store --follow

minikube-db-logs:
	kubectl --context $(KUBE_CONTEXT) --namespace $(KUBE_NAMESPACE) logs statefulset/postgres --follow

minikube-db-shell:
	kubectl --context $(KUBE_CONTEXT) --namespace $(KUBE_NAMESPACE) exec --stdin --tty statefulset/postgres -- sh -c 'exec psql -U "$$POSTGRES_USER" -d "$$POSTGRES_DB"'

minikube-db-clear:
	kubectl --context $(KUBE_CONTEXT) --namespace $(KUBE_NAMESPACE) exec statefulset/postgres -- sh -c 'psql -v ON_ERROR_STOP=1 -U "$$POSTGRES_USER" -d "$$POSTGRES_DB" -c "TRUNCATE TABLE order_items, orders RESTART IDENTITY;"'

minikube-previous-logs:
	kubectl --context $(KUBE_CONTEXT) --namespace $(KUBE_NAMESPACE) logs deployment/store --previous

minikube-events:
	kubectl --context $(KUBE_CONTEXT) --namespace $(KUBE_NAMESPACE) get events --sort-by=.metadata.creationTimestamp

minikube-top:
	@while true; do \
		clear; \
		date; \
		kubectl --context $(KUBE_CONTEXT) --namespace $(KUBE_NAMESPACE) top pods --containers; \
		sleep 2; \
	done

minikube-k9s:
	k9s --context $(KUBE_CONTEXT) --namespace $(KUBE_NAMESPACE)

minikube-clean:
	kubectl --context $(KUBE_CONTEXT) delete -k $(MINIKUBE_DIR) --ignore-not-found=true

minikube-stop:
	minikube stop --profile $(MINIKUBE_PROFILE)

monitoring-install:
	@command -v $(HELM) >/dev/null 2>&1 || { printf 'Helm is required. Install it with: brew install helm\n' >&2; exit 1; }
	$(HELM) repo add grafana https://grafana.github.io/helm-charts --force-update
	$(HELM) repo update grafana
	$(HELM) upgrade --install $(LOKI_RELEASE) $(LOKI_CHART) --version $(LOKI_CHART_VERSION) --namespace $(MONITORING_NAMESPACE) --create-namespace --values $(MONITORING_DIR)/loki-values.yaml --wait --timeout 10m
	$(HELM) upgrade --install $(ALLOY_RELEASE) $(ALLOY_CHART) --version $(ALLOY_CHART_VERSION) --namespace $(MONITORING_NAMESPACE) --values $(MONITORING_DIR)/alloy-values.yaml --wait --timeout 10m
	$(HELM) upgrade --install $(MONITORING_RELEASE) $(MONITORING_CHART) --version $(MONITORING_CHART_VERSION) --namespace $(MONITORING_NAMESPACE) --create-namespace --values $(MONITORING_DIR)/kube-prometheus-stack-values.yaml --wait --timeout 10m
	kubectl --context $(KUBE_CONTEXT) --namespace $(MONITORING_NAMESPACE) rollout status deployment/$(MONITORING_RELEASE)-kube-prometheus-operator --timeout=300s
	kubectl --context $(KUBE_CONTEXT) --namespace $(MONITORING_NAMESPACE) rollout status deployment/$(MONITORING_RELEASE)-grafana --timeout=300s
	kubectl --context $(KUBE_CONTEXT) apply -k $(MONITORING_DIR)

monitoring-status:
	$(HELM) list --namespace $(MONITORING_NAMESPACE)
	kubectl --context $(KUBE_CONTEXT) --namespace $(MONITORING_NAMESPACE) get pods,services
	kubectl --context $(KUBE_CONTEXT) --namespace $(MONITORING_NAMESPACE) get persistentvolumeclaims
	kubectl --context $(KUBE_CONTEXT) --namespace $(KUBE_NAMESPACE) get servicemonitor store

monitoring-grafana-password:
	@kubectl --context $(KUBE_CONTEXT) --namespace $(MONITORING_NAMESPACE) get secret $(MONITORING_RELEASE)-grafana -o jsonpath='{.data.admin-password}' | base64 --decode; printf '\n'

monitoring-grafana-forward:
	kubectl --context $(KUBE_CONTEXT) --namespace $(MONITORING_NAMESPACE) port-forward service/$(MONITORING_RELEASE)-grafana $(GRAFANA_LOCAL_PORT):80

monitoring-prometheus-forward:
	kubectl --context $(KUBE_CONTEXT) --namespace $(MONITORING_NAMESPACE) port-forward service/$(MONITORING_RELEASE)-kube-prometheus-prometheus $(PROMETHEUS_LOCAL_PORT):9090

monitoring-loki-forward:
	kubectl --context $(KUBE_CONTEXT) --namespace $(MONITORING_NAMESPACE) port-forward service/$(LOKI_RELEASE) $(LOKI_LOCAL_PORT):3100

monitoring-load-dashboard:
	@printf 'http://127.0.0.1/grafana/d/chaosbox-load-test/chaosbox-load-test\n'

monitoring-clean:
	@if kubectl --context $(KUBE_CONTEXT) get crd servicemonitors.monitoring.coreos.com >/dev/null 2>&1; then \
		kubectl --context $(KUBE_CONTEXT) delete -k $(MONITORING_DIR) --ignore-not-found=true; \
	fi
	@if command -v $(HELM) >/dev/null 2>&1; then \
		$(HELM) uninstall $(ALLOY_RELEASE) --namespace $(MONITORING_NAMESPACE) --ignore-not-found; \
		$(HELM) uninstall $(LOKI_RELEASE) --namespace $(MONITORING_NAMESPACE) --ignore-not-found; \
		$(HELM) uninstall $(MONITORING_RELEASE) --namespace $(MONITORING_NAMESPACE) --ignore-not-found; \
	fi
	kubectl --context $(KUBE_CONTEXT) --namespace $(MONITORING_NAMESPACE) delete persistentvolumeclaims -l app.kubernetes.io/instance=$(LOKI_RELEASE) --ignore-not-found=true

profile-forward:
	kubectl --context $(KUBE_CONTEXT) --namespace $(KUBE_NAMESPACE) port-forward deployment/store $(PPROF_LOCAL_PORT):6060

profile-capture: profile-cpu profile-heap profile-allocs profile-goroutine

profile-cpu:
	@mkdir -p $(PROFILE_DIR)
	@timestamp="$$(date -u +%Y%m%dT%H%M%SZ)"; \
	curl --fail --silent --show-error --output "$(PROFILE_DIR)/cpu-$$timestamp.pprof" "http://127.0.0.1:$(PPROF_LOCAL_PORT)/debug/pprof/profile?seconds=$(PROFILE_SECONDS)"; \
	printf 'Captured %s\n' "$(PROFILE_DIR)/cpu-$$timestamp.pprof"

profile-heap:
	@mkdir -p $(PROFILE_DIR)
	@timestamp="$$(date -u +%Y%m%dT%H%M%SZ)"; \
	curl --fail --silent --show-error --output "$(PROFILE_DIR)/heap-$$timestamp.pprof" "http://127.0.0.1:$(PPROF_LOCAL_PORT)/debug/pprof/heap?gc=1"; \
	printf 'Captured %s\n' "$(PROFILE_DIR)/heap-$$timestamp.pprof"

profile-allocs:
	@mkdir -p $(PROFILE_DIR)
	@timestamp="$$(date -u +%Y%m%dT%H%M%SZ)"; \
	curl --fail --silent --show-error --output "$(PROFILE_DIR)/allocs-$$timestamp.pprof" "http://127.0.0.1:$(PPROF_LOCAL_PORT)/debug/pprof/allocs"; \
	printf 'Captured %s\n' "$(PROFILE_DIR)/allocs-$$timestamp.pprof"

profile-goroutine:
	@mkdir -p $(PROFILE_DIR)
	@timestamp="$$(date -u +%Y%m%dT%H%M%SZ)"; \
	curl --fail --silent --show-error --output "$(PROFILE_DIR)/goroutine-$$timestamp.pprof" "http://127.0.0.1:$(PPROF_LOCAL_PORT)/debug/pprof/goroutine"; \
	printf 'Captured %s\n' "$(PROFILE_DIR)/goroutine-$$timestamp.pprof"

load-smoke:
	$(PYTHON) tests/load/phase1.py --profile smoke --base-url $(LOAD_BASE_URL) --grafana-url $(GRAFANA_URL)

load-test:
	$(PYTHON) tests/load/phase1.py --profile load --base-url $(LOAD_BASE_URL) --grafana-url $(GRAFANA_URL) --users $(LOAD_USERS) --ramp-seconds $(LOAD_RAMP_SECONDS) --hold-seconds $(LOAD_HOLD_SECONDS)

load-results:
	@printf 'Load-test artifacts: %s/artifacts/load-tests\n' "$(CURDIR)"

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

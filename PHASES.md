# Project Phases

ChaosBox progresses in phases, introducing technologies and architectural patterns only when a specific scaling, reliability, data, networking, or operational problem makes them relevant. Each phase documents the problem, evidence, chosen tradeoff, and measured result.

Return to the [project README](README.md).

## Phase 1

### Goal

Build the simplest working application that ChaosBox can later use to simulate production conditions and failures.

### User Flow

1. Products are seeded into the database.
2. A user fetches the available products.
3. The user buys one or more products and receives an `order_number`.
4. The user fetches that order by its `order_number`.

Fetching all orders would return a growing response as new orders are created, so the flow retrieves only the order just created.

### Endpoints

- `GET /health`: check service health.
- `GET /products`: fetch available products.
- `POST /orders`: buy one or more products and create an order.
- `GET /orders/{id}`: fetch one order by its primary key.

### Stack

- Go
- PostgreSQL
- Docker
- Seed scripts

### Status: Testing In Progress

- The Go REST API implements health, product listing, order creation, and single-order lookup.
- PostgreSQL migrations create products, orders, and order items with constraints and indexes.
- Five products are seeded repeatably without duplicating data on server restart.
- Orders support multiple products and are created transactionally.
- Docker Compose runs PostgreSQL with persistent storage.
- A multi-stage, non-root container image packages the Go store service.
- Minikube manifests deploy the GitHub-published store image, Nginx Ingress routing, and persistent PostgreSQL.
- Structured request logs and request IDs correlate load-test failures with Loki records.
- Prometheus, Grafana, Loki, and Alloy provide a local metrics, dashboard, and logging baseline.
- A private, loopback-only `pprof` listener supports controlled CPU, heap, allocation, and goroutine captures during load tests.
- Unit, race, and Testcontainers end-to-end tests are available through the root Makefile.
- Bruno contains the complete local Phase 1 API flow.
- GitHub Actions runs formatting, tests, vetting, PostgreSQL end-to-end checks, and a container build.

### Architecture

![ChaosBox Phase 1 architecture](assets/architecture/chaosbox-phase1.svg)

The load generator runs outside Minikube. `minikube tunnel` makes the cluster entry point reachable from the host, and Nginx Ingress routes one endpoint by path:

- `/api` routes to the store ClusterIP Service.
- `/grafana` routes to Grafana.
- `/prometheus` routes to Prometheus.

The store Service selects one deliberately constrained Go API pod. The API reaches PostgreSQL through a separate stable ClusterIP Service, and PostgreSQL stores its data in a `1Gi` persistent volume claim. PostgreSQL and Loki remain internal because the external workflow does not require direct access to either service.

Nginx performs HTTP path routing; it is not an additional application load balancer. Kubernetes Services select ready pods and distribute traffic when replicas exist. A cloud load balancer would expose the ingress controller in a hosted environment, while `minikube tunnel` provides that connectivity locally.

### Observability Baseline

The store exposes HTTP, Go runtime, process, and PostgreSQL connection-pool metrics through `/metrics`. Prometheus discovers that endpoint through a `ServiceMonitor` and scrapes it every five seconds. Grafana includes store and load-test dashboards for throughput, errors, latency, resource use, restarts, pool activity, and correlated logs.

The API writes structured JSON logs to standard output. Every response includes an `X-Request-ID`: a valid client-provided ID is preserved, otherwise the API generates one. Completion logs include the request ID, method, matched route, status, duration, and response size. Health and metrics requests use debug level to avoid routine probe noise.

Grafana Alloy collects Kubernetes pod logs and sends them to Loki with 24-hour retention. Failed load-test request IDs and a Grafana URL scoped to each run are retained with the test artifacts, connecting client failures to metrics and server logs on the same timeline.

### Testing

Testing has two layers:

- Correctness testing through Go unit tests, race detection, Testcontainers end-to-end tests, Bruno, and GitHub Actions.
- Load testing in Minikube with CPU and memory limits while traffic ramps from 0 to 100 concurrent users at one new user per second.

The load script in `tests/load/phase1.py` has each virtual user repeatedly perform this flow:

```text
GET /products -> wait 200 ms -> POST /orders -> wait 200 ms -> GET /orders/{id}
```

The load test must fetch each created order by `id` and verify that the returned `order_number` matches it, at least 99% of checks pass, the HTTP failure rate remains below 1%, and the store pod has no unexpected restarts or OOM kills.

With `make minikube-tunnel` running, use `make load-smoke` to run one journey or `make load-test` to ramp up virtual users. `make smoke` checks only health and products on a local server.

### Minikube Resource Limits

| Service | CPU Request | CPU Limit | Memory Request | Memory Limit | Persistent Storage |
| --- | ---: | ---: | ---: | ---: | --- |
| Store | `10m` | `100m` | `16Mi` | `32Mi` | None |
| PostgreSQL | `25m` | `200m` | `64Mi` | `128Mi` | `1Gi` (`ReadWriteOnce`) |

These intentionally low limits establish the first failure baseline. When load testing causes CPU throttling, OOM kills, or unacceptable latency, increase one resource at a time and compare the results.

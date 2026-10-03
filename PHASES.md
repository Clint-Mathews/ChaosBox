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

### Status: Complete

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
- Product responses are cached in-process because the seeded catalog is immutable during the Phase 1 experiment.
- Order creation uses one writable CTE statement, and order retrieval uses the order primary key.
- Repeated comparison runs selected a 12-connection database pool and a `200m` store CPU limit.

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

Testing used two layers:

- Correctness testing through Go unit tests, race detection, Testcontainers end-to-end tests, Bruno, and GitHub Actions.
- Load testing in Minikube with CPU and memory limits while traffic ramps from 0 to 100 concurrent users over 60 seconds.

The load script in `tests/load/phase1.py` has each virtual user repeatedly perform this flow:

```text
GET /products -> wait 200 ms -> POST /orders -> wait 200 ms -> GET /orders/{id}
```

The load test fetches each created order by `id` and verifies that the returned `order_number` matches it. A passing run requires at least 99% of checks to pass, an HTTP failure rate below 1%, and no unexpected store restarts or OOM kills.

With `make minikube-tunnel` running, use `make load-smoke` to run one journey or `make load-test` to ramp up virtual users. `make smoke` checks only health and products on a local server.

### Minikube Resource Limits

| Service | CPU Request | CPU Limit | Memory Request | Memory Limit | Persistent Storage |
| --- | ---: | ---: | ---: | ---: | --- |
| Store | `10m` | `200m` | `16Mi` | `32Mi` | None |
| PostgreSQL | `25m` | `500m` | `64Mi` | `256Mi` | `1Gi` (`ReadWriteOnce`) |

These limits are the final Phase 1 comparison configuration. Resource experiments changed one constrained variable at a time rather than increasing the entire stack together.

### Measured Result

The final repeated configuration used one store replica, a 12-connection pool, a 60-second ramp to 100 users, and a 300-second hold. The latest passing result completed 155,532 requests with zero failures:

- 431.51 HTTP requests per second.
- 143.84 complete journeys per second.
- Product p95: 188.56 ms.
- Create-order p95: 386.26 ms.
- Get-order p95: 345.30 ms.

This is a local closed-model benchmark, not a production capacity rating. One hundred users with 400 ms total think time also impose a theoretical ceiling of 750 RPS. Phase 2 therefore changes both the deployment environment and the capacity-test model.

### Key Insights

- Correct request arithmetic is part of benchmark validation: each successful journey must produce exactly three requests and six checks.
- A growing collection response made the original workload progressively more expensive, so the user flow was changed to fetch only the order just created.
- Caching the immutable product catalog, using the order primary key, and reducing order creation to one SQL round trip removed repeated work before adding resources.
- Database CPU, application CPU, and pool size are separate variables. The 12-connection pool was retained only after a repeated run separated its effect from host disk contention.
- Memory was not the constraint; the store remained at `32Mi` rather than increasing memory without evidence.
- Complete journeys, route latency, failures, resource saturation, profiles, and environmental noise all matter more than an isolated peak-RPS number.

## Phase 2

### Status: Planned

### Goal

Reproduce the trusted baseline in one GCP region, make application replication operationally safe, and measure horizontal scaling from one to multiple store replicas.

### Scope

- Move schema migrations and product seeding out of every application startup into an explicit deployment job.
- Add idempotency for order creation, graceful shutdown, hardened readiness and liveness probes, disruption controls, and topology-aware scheduling.
- Deploy the store to a regional GKE cluster and PostgreSQL to Cloud SQL over private networking.
- Run an open arrival-rate load generator outside the application cluster.
- Compare fixed replica counts with a fixed aggregate database-connection budget before enabling HPA.
- Retain correctness, latency, saturation, resilience, and cost evidence for every promoted result.

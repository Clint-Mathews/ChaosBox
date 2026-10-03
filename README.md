# ChaosBox

A local Kubernetes playground for simulating production conditions such as resource limits, traffic spikes, network latency, regional delays, and service failures.

ChaosBox explores how a deliberately simple system can evolve toward large workloads. Rather than simulating a billion concurrent users locally, each phase introduces production-shaped traffic and failures, identifies the next measurable constraint, and tests a targeted improvement.

See [Project Phases](PHASES.md) for completed and current goals, API behavior, test criteria, and resource limits.

## Getting Started

Run all commands from the repository root. Local development requires Go 1.27.1 or later, Docker with Docker Compose, `make`, and `curl`.

```sh
make dev
```

This starts PostgreSQL, waits for it to become healthy, and runs the API at `http://localhost:8080`. Verify it from another terminal with `make smoke`.

Stop the API with `Ctrl+C`, then stop PostgreSQL with `make db-down`. Run `make help` for all database, Minikube, load-testing, and verification commands.

### Minikube

Minikube workflows additionally require `minikube` and `kubectl`:

```sh
make minikube-start
make minikube-build
make minikube-deploy
make minikube-tunnel
```

Keep the tunnel running, then use `make minikube-url` to print the Nginx gateway URLs. Run `make load-smoke` or `make load-test` from another terminal. See the [Phase 1 details](PHASES.md#phase-1) for the architecture, observability baseline, and test requirements, or the [Minikube deployment guide](deploy/minikube/README.md) for operational commands.

## Current Architecture

![ChaosBox system overview](assets/architecture/chaosbox-overview.svg)

The Phase 1 system uses one Nginx Ingress endpoint for the store and observability tools. See [Phase 1 architecture](PHASES.md#architecture) for the request, metrics, logging, and persistence paths.

## Current Status

### Phase 1: Complete

Phase 1 established a trusted local, single-replica baseline. The final repeated configuration used one Go store replica limited to `200m` CPU and `32Mi` memory, PostgreSQL limited to `500m` CPU and `256Mi` memory, and a 12-connection database pool.

The latest passing run completed 155,532 requests with zero failures:

- 431.51 HTTP requests per second.
- 143.84 complete three-request journeys per second.
- 188.56 ms product p95, 386.26 ms create-order p95, and 345.30 ms get-order p95.

The important result is not the largest number. Phase 1 produced a repeatable method and several concrete lessons:

- Validate complete business journeys and reconcile request counts before trusting RPS.
- Keep the workload bounded; replacing a growing all-orders read with a primary-key lookup made runs comparable.
- Remove repeated work before adding capacity; the product catalog cache and one-statement order write reduced application and database work.
- Tune resources and the connection pool independently; more connections did not compensate for constrained PostgreSQL or store CPU.
- Repeat outliers before accepting or rejecting a configuration; host disk contention distorted one pool-size comparison.
- Preserve synchronized load-test, metrics, logs, and profile windows so every conclusion has supporting evidence.

See [Phase 1 details](PHASES.md#phase-1) for the implementation, test method, and measured result.

### Phase 2: Scale Through GCP

Phase 2 moves the same service into one GCP region and tests horizontal scaling safely. Before replicas increase, migrations and seed work move out of application startup, order creation gains idempotency, shutdown becomes graceful, probes are hardened, and one aggregate database-connection budget is defined.

The test sequence first reproduces the single-replica baseline in GCP, then compares fixed replica counts before enabling autoscaling. Load generation runs outside the application cluster and uses an open arrival-rate model for capacity testing.

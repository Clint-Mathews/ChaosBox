# Project Phases

ChaosBox progresses in phases, introducing technologies and architectural patterns only when a specific scaling, reliability, data, networking, or operational problem makes them relevant. Each phase documents the problem, evidence, chosen tradeoff, and measured result.

Return to the [project README](README.md).

## Phase 1

### Goal

Build the simplest working application that ChaosBox can later use to simulate production conditions and failures.

### User Flow

1. Products are seeded into the database.
2. A user fetches the available products.
3. The user buys one or more products.
4. The user fetches the orders listing.

### Endpoints

- `GET /health`: check service health.
- `GET /products`: fetch available products.
- `POST /orders`: buy one or more products and create an order.
- `GET /orders`: fetch orders.

### Stack

- Go
- PostgreSQL
- Docker
- Seed scripts

### Status: Testing In Progress

- The Go REST API implements health, product listing, order creation, and order listing.
- PostgreSQL migrations create products, orders, and order items with constraints and indexes.
- Five products are seeded repeatably without duplicating data on server restart.
- Orders support multiple products and are created transactionally.
- Docker Compose runs PostgreSQL with persistent storage.
- A multi-stage, non-root container image packages the Go store service.
- Minikube manifests deploy the GitHub-published store image and persistent PostgreSQL.
- Unit, race, and Testcontainers end-to-end tests are available through the root Makefile.
- Bruno contains the complete local Phase 1 API flow.
- GitHub Actions runs formatting, tests, vetting, PostgreSQL end-to-end checks, and a container build.

The [current architecture and diagram](README.md#current-architecture) remain in the main README.

### Testing

Testing has two layers:

- Correctness testing through Go unit tests, race detection, Testcontainers end-to-end tests, Bruno, and GitHub Actions.
- Load testing in Minikube with CPU and memory limits while traffic ramps from 0 to 100 concurrent users at one new user per second.

Each virtual user repeatedly performs this flow:

```text
GET /products -> wait 200 ms -> POST /orders -> wait 200 ms -> GET /orders
```

The load test must verify that each created `order_number` appears in the global orders response, at least 99% of checks pass, the HTTP failure rate remains below 1%, and the store pod has no unexpected restarts or OOM kills.

### Minikube Resource Limits

| Service | CPU Request | CPU Limit | Memory Request | Memory Limit | Persistent Storage |
| --- | ---: | ---: | ---: | ---: | --- |
| Store | `10m` | `100m` | `16Mi` | `32Mi` | None |
| PostgreSQL | `25m` | `200m` | `64Mi` | `128Mi` | `1Gi` (`ReadWriteOnce`) |

These intentionally low limits establish the first failure baseline. When load testing causes CPU throttling, OOM kills, or unacceptable latency, increase one resource at a time and compare the results.

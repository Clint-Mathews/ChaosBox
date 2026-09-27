# ChaosBox

A local Kubernetes playground for simulating production conditions: resource limits, traffic spikes, network latency, regional delays, and service failures.

We will get a Go service up and see how to improve it as we go.

## Phases 

### Phase 1

#### Goal

Build the simplest working application that ChaosBox can later use
to simulate production conditions and failures.

#### User Flow
1. Products are seeded into the database
2. User fetches available products
3. User buys one or more products
4. User fetches the orders listing

#### Endpoints

GET  /health
- Service health check

GET  /products
- Fetch available products

POST /orders
- Buy one or more products / create an order

GET  /orders
- Fetch orders


#### Stack
- Go
- PostgreSQL
- Docker
- Seed scripts

#### Initial Architecture

Client
   |
   v
Go Service
   |
   v
PostgreSQL

#### Current Status: Phase 1 Testing In Progress

- Go REST API implements health, product listing, order creation, and order listing.
- PostgreSQL migrations create products, orders, and order items with constraints and indexes.
- Five products are seeded repeatably without duplicating data on server restart.
- Orders support multiple products and are created transactionally.
- Docker Compose runs PostgreSQL with persistent storage.
- A multi-stage, non-root container image packages the Go store service.
- Minikube manifests deploy the GitHub-published store image and persistent PostgreSQL.
- Unit, race, and Testcontainers end-to-end tests are available through the root Makefile.
- Bruno contains the complete local Phase 1 API flow.
- GitHub Actions runs formatting, tests, vetting, PostgreSQL end-to-end checks, and a container build.

#### Testing

Testing is part of Phase 1 and has two layers:

- Correctness testing through Go unit tests, race detection, Testcontainers end-to-end tests, Bruno, and GitHub Actions.
- Load testing in Minikube with CPU and memory limits while traffic ramps from 0 to 100 concurrent users at one new user per second.

Each virtual user repeatedly performs this flow:

```text
GET /products -> wait 200 ms -> POST /orders -> wait 200 ms -> GET /orders
```

The load test must verify that each created `order_number` appears in the global orders response, at least 99% of checks pass, the HTTP failure rate remains below 1%, and the store pod has no unexpected restarts or OOM kills.

#### Minikube Resource Limits

| Service | CPU Request | CPU Limit | Memory Request | Memory Limit | Persistent Storage |
| --- | ---: | ---: | ---: | ---: | --- |
| Store | `10m` | `100m` | `16Mi` | `32Mi` | None |
| PostgreSQL | `25m` | `200m` | `64Mi` | `128Mi` | `1Gi` (`ReadWriteOnce`) |

These intentionally low Phase 1 limits establish the first failure baseline.
When load testing causes CPU throttling, OOM kills, or unacceptable latency,
increase one resource at a time and compare the results.

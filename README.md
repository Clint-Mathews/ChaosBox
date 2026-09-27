# ChaosBox

A local Kubernetes playground for simulating production conditions: resource limits, traffic spikes, network latency, regional delays, and service failures.

ChaosBox is an iterative lab for exploring how a deliberately simple system can evolve toward serving increasingly large workloads. The long-term question is: can this system evolve toward serving a billion users?

The goal is not to simulate a billion concurrent users from one local cluster. The goal is to model production-shaped traffic and failure conditions, identify the next measurable constraint, implement a strategy to address it, and test the result.

The project will progress in phases. It starts with a Go service and PostgreSQL, then introduces different technologies and architectural patterns only when a specific scaling, reliability, data, networking, or operational problem makes them relevant. Each phase should document the problem, the evidence, the chosen tradeoff, and what changed after the solution was implemented.

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

#### Current Architecture

![ChaosBox Phase 1 architecture](assets/architecture/chaosbox-phase1.svg)

The load generator runs outside Minikube and reaches the store through `kubectl port-forward`. Inside the `chaosbox` namespace, a ClusterIP Service routes requests to one constrained Go API replica, which uses PostgreSQL through its own stable Service. PostgreSQL data survives pod replacement through a `1Gi` persistent volume claim.

[Open the editable Excalidraw source](assets/architecture/chaosbox-phase1.excalidraw)

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

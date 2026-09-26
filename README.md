# ChaosBox

A local Kubernetes playground for simulating production conditions: resource limits, traffic spikes, network latency, regional delays, and service failures.

We will get a Go servcie up and see how to improve it as we go.

## Phases 

### Phase 1

#### Goal

Build the simplest working application that ChaosBox can later use
to simulate production conditions and failures.

#### User Flow
1. Products are seeded into the database
2. User fetch available products
3. User buys a product
4. USer fetch orders listing

#### Endpoints

GET  /health
- Service health check

GET  /products
- Fetch available products

POST /orders
- Buy a product / create an order

GET  /orders
- Fetch orders


#### Stack
- Go
- Postgres
- Docker
- Seed Scripts

#### Initial Architecture

Client
   |
   v
Go Servcie
   |
   v
Postgres

#### Current Status: Boiler plate for server added
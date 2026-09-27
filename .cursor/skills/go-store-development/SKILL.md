---
name: go-store-development
description: Implements and reviews Go HTTP server, REST endpoint, domain model, and test changes for the ChaosBox store service. Use when editing Go files under apps/store, adding endpoints, changing request or response contracts, wiring dependencies, or testing handlers.
---

# Go Store Development

## Project Context

- The Go module is `apps/store`.
- `cmd/server/main.go` owns process configuration, dependency construction, and server startup.
- `rest-api` owns HTTP routing, validation, and JSON responses. Its package name is `restapi`.
- `database` owns PostgreSQL models and persistence.
- Read the repository `README.md` and `apps/store/docs/schema.md` before changing behavior.

## Conventions

- Prefer the standard library and the existing `net/http` method-aware `ServeMux`.
- Register routes in `rest-api/handler.go` using patterns such as `GET /products`.
- Keep database calls behind the small `Store` interface consumed by REST handlers.
- Pass `r.Context()` through every database operation.
- Use snake_case JSON fields and `application/json` responses.
- Return empty slices as `[]`, not `null`.
- Return client-safe JSON errors shaped as `{"error":"message"}`; log internal details separately.
- Limit JSON request bodies, reject unknown fields, reject trailing JSON, and validate values before persistence.
- Keep order creation transactional; do not move persistence logic into handlers.
- Add comments only when behavior or external codes are not self-explanatory.

## Endpoint Workflow

1. Confirm the request, response, validation, and status-code contract.
2. Add or adjust database-facing model types only when the contract requires it.
3. Extend the `Store` interface minimally.
4. Implement the handler and register its route.
5. Add `httptest` coverage using a fake `Store`; do not require PostgreSQL for handler unit tests.
6. Cover the success path, malformed input, validation failures, and mapped store errors.
7. Exercise the endpoint against the compose database when persistence changes.

## Error Handling

- Wrap internal errors with operation context using `%w`.
- Compare sentinel errors with `errors.Is`.
- Do not expose SQL, credentials, or internal error strings in HTTP 500 responses.
- Treat expected validation and missing-reference failures as client errors.

## Validation

Run commands from `apps/store`:

```bash
gofmt -w <changed-go-files>
go test ./...
go vet ./...
go test -race ./...
```

From the repository root, also run:

```bash
git diff --check
```

For a live check, start PostgreSQL with `docker compose up -d` from `apps/store`, run `go run ./cmd/server`, and call the affected endpoint with `curl`.

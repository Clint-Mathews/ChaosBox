---
name: postgres-store-development
description: Implements and reviews PostgreSQL schemas, migrations, seeds, pgx queries, transactions, and database verification for the ChaosBox store service. Use when changing files under apps/store/database, modifying schema.md, adding SQL, changing product or order persistence, or debugging PostgreSQL behavior.
---

# PostgreSQL Store Development

## Project Context

- PostgreSQL runs as the only service in `apps/store/compose.yaml`.
- The persistent named volume is mounted at `/var/lib/postgresql/data`.
- The Go data layer uses `github.com/jackc/pgx/v5/pgxpool`.
- SQL migrations live in `apps/store/database/migrations`.
- Repeatable seed files live in `apps/store/database/seeds`.
- The intended model is documented in `apps/store/docs/schema.md`.

## Schema Rules

- Use `BIGINT GENERATED ALWAYS AS IDENTITY` primary keys.
- Use `TIMESTAMPTZ NOT NULL DEFAULT NOW()` for creation timestamps.
- Enforce required values, uniqueness, positive quantities, and relationships in PostgreSQL rather than only in Go.
- Model products in orders through `order_items` with primary key `(order_id, product_id)`.
- Use `ON DELETE CASCADE` from order items to orders and `ON DELETE RESTRICT` from order items to products.
- Index foreign-key columns used independently in lookups.
- Use `text_pattern_ops` when a B-tree index must support case-sensitive `LIKE 'prefix%'` queries.

## Migration Workflow

1. Read `schema.md`, existing migrations, and the current query code.
2. Never rewrite an already-applied migration to change an existing database. Add the next numbered migration instead.
3. Make migrations preserve existing persisted data unless destructive behavior is explicitly requested.
4. Let the embedded migration runner record successful files in `schema_migrations`.
5. Keep seed scripts repeatable. A server restart must not duplicate seed rows.
6. Do not rely only on `/docker-entrypoint-initdb.d`; it does not rerun for an existing volume.

## Query Conventions

- Use parameter placeholders such as `$1`; never concatenate user input into SQL.
- Pass `context.Context` to every pgx operation.
- Close rows and check `rows.Err()` after iteration.
- Return empty slices instead of nil slices for collection queries.
- Avoid N+1 queries; load orders and their items with a join and group rows in Go.
- Wrap database errors with the operation that failed.

## Transactions And Errors

- Create an order and all order items in one transaction.
- Defer rollback immediately after beginning; commit only after all writes and reads succeed.
- Translate only known PostgreSQL SQLSTATE values into domain errors.
- `23503` means `foreign_key_violation`.
- `23505` means `unique_violation`.
- Preserve unknown PostgreSQL errors as wrapped internal errors.

## Local Operations

Run from `apps/store`:

```bash
docker compose up -d
docker compose ps
docker compose exec -T postgres psql -U chaosbox -d chaosbox
```

Use `docker compose down` to stop services while retaining data. Do not use `docker compose down -v` unless deleting all persisted data is explicitly intended.

## Verification

- Confirm the container reports healthy before testing queries.
- Apply migrations through normal server startup and verify their rows in `schema_migrations`.
- Run seeds repeatedly and confirm row counts do not grow.
- Verify constraints with both valid and invalid writes.
- Verify failed multi-item orders leave no partial order or order items.
- Restart the server and container, then confirm data persists.
- Run `go test ./...`, `go vet ./...`, and `git diff --check` after database changes.

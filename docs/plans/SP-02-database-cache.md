# SP-02 Database and Cache

## Ownership

`db/`, `internal/platform/database/`, `internal/platform/cache/`.

## Deliverables

- Clean SQL migrations for identity, audit, tasks, and seed data.
- pgxpool lifecycle, transaction runner, health check, and sqlc configuration/queries.
- Redis client lifecycle, namespaced key builder, TTL operations, and health check.
- PostgreSQL production schema only; no historical compatibility or SQLite adapter.

## Tests

- Migration up/down/up, seed idempotency, transaction commit/rollback, constraints, Redis TTL, deletion, and unavailable-backend errors.

## Exclusions

- No HTTP handlers or module application services.

# Backend Infrastructure Go Implementation Plan

**Goal:** Build a clean Go backend infrastructure foundation in an independent GitHub repository.

**Architecture:** A modular monolith with explicit composition in `cmd` and `internal/bootstrap`, infrastructure adapters in `internal/platform`, and domain/application behavior in `internal/modules`. Domain packages depend on interfaces rather than HTTP, pgx, Redis, or Asynq implementations.

**Tech Stack:** Go, Chi, OpenAPI, pgxpool, sqlc, PostgreSQL, go-redis, Asynq, Prometheus, golang-migrate, Docker Compose.

## Subplans

1. Engineering and platform bootstrap.
2. Database, migrations, sqlc, and Redis adapters.
3. HTTP contracts, middleware, errors, responses, pagination, health, and rate limiting.
4. IAM and RBAC.
5. Audit, observability, retry, and circuit breaker.
6. Generic task control plane, Asynq worker, scheduler, idempotency, retries, and failure archive.
7. Docker, CI, security hardening, end-to-end tests, architecture review, and final regression.

## Integration order

- Foundation: subplans 1-3.
- Modules: subplans 4-6 after foundation interfaces stabilize.
- Delivery: subplan 7 after application integration.

## Completion

Completion requires fresh evidence for every test and quality gate in `.omx/plans/test-spec-backend-infrastructure-go.md`, a clean worktree, and equality between local `main` and `origin/main`.

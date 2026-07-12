# Backend Infrastructure Go PRD

## Goal

Deliver a clean, reusable Go modular monolith foundation with HTTP, configuration, PostgreSQL, Redis, IAM/RBAC, audit, observability, resilience, and generic asynchronous task infrastructure.

## Decisions

- New private repository and empty database.
- No Python runtime, historical migrations, old data, or dual-stack period.
- Chi plus OpenAPI for HTTP contracts.
- pgxpool plus sqlc for PostgreSQL.
- go-redis for cache and refresh sessions.
- Asynq for tasks, retries, delayed jobs, and scheduling.
- Account/password authentication with Argon2id, JWT access tokens, and opaque refresh tokens.
- Keep the frontend-facing response envelope and core authentication contract.

## Deliverables

1. API, worker, scheduler, and migration commands.
2. Typed configuration, structured logging, graceful shutdown, health endpoints, metrics, CORS, recovery, request IDs, rate limiting, retry, and circuit breaking.
3. Clean identity, RBAC, audit, task, execution, and schedule schemas.
4. IAM, RBAC, audit, and task application modules.
5. Docker Compose, migration gate, CI, security scanning, and complete verification evidence.

## Excluded

- OAuth and vendor captcha.
- Douyin collection, browser agents, ETL, analytics, and dashboard business APIs.
- Compatibility with Alembic, Funboost, Python Redis databases, or old data.

## Acceptance

- All commands build and shut down gracefully.
- OpenAPI and HTTP contract tests pass.
- PostgreSQL migrations pass up, down, and up on an empty database.
- Authentication, RBAC, audit, and task end-to-end workflows pass.
- `go test -race ./...`, `go vet ./...`, `go build ./...`, Staticcheck, and Govulncheck pass.
- Docker Compose starts a healthy API, worker, scheduler, PostgreSQL, Redis, and migration gate.

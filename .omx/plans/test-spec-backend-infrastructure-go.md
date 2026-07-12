# Backend Infrastructure Go Test Specification

## Unit

- Configuration validation and defaults.
- Error mapping, response envelopes, pagination, request IDs, and path normalization.
- Argon2id password verification, JWT validation, refresh revocation, and RBAC wildcard matching.
- Retry, timeout, circuit breaker, task state transitions, and idempotency.

## Integration

- PostgreSQL repositories, constraints, transactions, rollback, and seed idempotency.
- Redis TTL, token revocation, rate-limit Lua behavior, and Asynq lifecycle.
- Audit persistence for authentication, authorization, administration, and tasks.

## Contract and E2E

- OpenAPI paths, methods, validation, status codes, cookies, bearer authentication, envelopes, and pagination.
- Login -> permission check -> enqueue -> worker execution -> status query -> audit record.
- PostgreSQL and Redis outage behavior, token tampering, refresh replay, duplicate task submission, and worker interruption.

## Gates

- `go test ./...`
- `go test -race ./...`
- `go vet ./...`
- `go build ./...`
- `staticcheck ./...`
- `govulncheck ./...`
- Migration `up -> down -> up`
- Fresh Docker Compose E2E run

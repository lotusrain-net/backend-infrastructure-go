# SP-01 Engineering and Platform Bootstrap

## Ownership

`cmd/`, `internal/bootstrap/`, `internal/config/`, `internal/platform/logging/`, `internal/platform/resilience/`, root build tooling.

## Deliverables

- Test-first typed environment configuration with required-value validation.
- Structured `slog` JSON logger.
- API, worker, scheduler, and migrate command entrypoints with context cancellation and graceful shutdown hooks.
- Retry and circuit-breaker primitives that accept `context.Context`.
- Makefile targets for test, race, vet, build, staticcheck, and govulncheck.

## Tests

- Defaults, invalid configuration, secret validation, cancellation, bounded retry, context timeout, and breaker state transitions.

## Exclusions

- No HTTP routes, PostgreSQL queries, Redis commands, authentication, or task handlers.

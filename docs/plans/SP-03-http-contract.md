# SP-03 HTTP Contract and Middleware

## Ownership

`api/openapi.yaml`, `internal/platform/httpserver/`, `internal/shared/apperror/`, `internal/shared/response/`, `internal/shared/pagination/`, `internal/shared/requestcontext/`.

## Deliverables

- Chi router and middleware chain for request ID, recovery, CORS, logging, response handling, and Redis-backed rate limiting.
- Stable `{code,msg,data}` envelope and pagination contract.
- `/health/live`, `/health/ready`, and `/metrics` route contracts with dependency interfaces.
- OpenAPI definitions for shared schemas and base endpoints.

## Tests

- Envelope encoding, status mapping, validation errors, panic recovery, request IDs, CORS, pagination, path normalization, rate-limit boundaries, fail-open, and critical-route fail-closed behavior.

## Exclusions

- No IAM, audit, or task business handlers.

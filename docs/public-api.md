# Public API

## Go

The Go release publishes six importable packages from
`github.com/jyysy/backend-infrastructure-go`:

- `pkg/postgres`: validated pool configuration, health probes, and `pgx.Tx`
  transaction execution with commit, rollback, and panic propagation rules.
- `pkg/httpkit`: Chi router composition, strict JSON decoding, request IDs,
  recovery, access logging, CORS, and injectable rate limiting.
- `pkg/apikit`: `{code,msg,data}` envelopes and stable HTTP/application errors.
- `pkg/lifecycle`: cancellable components and bounded, reverse-order cleanup.
- `pkg/logging`: JSON `slog` construction and strict level parsing.
- `pkg/pagination`: normalized pages and metadata with a size cap of 100.

Only exported identifiers documented by GoDoc are part of the v0.1 contract.
The repository's `internal/` tree is private and is not a supported import
surface. In particular, IAM, task, audit, sqlc/dbgen, migrations, Asynq,
scheduler, and application assembly remain private.

## npm

The public package is `@purplevoid/backend-infrastructure-web@0.1.0`.

| Export | Scope |
| --- | --- |
| `.` | framework-neutral API and theme functions only |
| `./api` | `ApiClient`, `ApiError`, envelopes, pagination, and `withQuery` |
| `./theme` | appearance types, token derivation, parsing, and validation |
| `./ui` | Alert, Badge, Button, Card, Input, Skeleton, Table, Textarea, `cn` |
| `./ui/client` | React client-boundary Radix controls and pagination |
| `./patterns` | data tables, filters, dialogs, loading/error states, and pagination |
| `./styles.css` | precompiled component tokens and styles only |

React and ReactDOM are peer dependencies. The `ApiClient` receives its fetch,
base URL, credentials, refresh callback, expiry callback, and messages through
constructor options. It coalesces concurrent refresh calls and retries a
failed authenticated request at most once; it does not access stores,
QueryClient, or browser navigation.

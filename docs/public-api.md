# Public API

## Go

The Go release publishes importable packages from
`github.com/lotusrain-net/backend-infrastructure-go`:

- `pkg/postgres`: validated pool configuration, health probes, and `pgx.Tx`
  transaction execution with commit, rollback, and panic propagation rules.
- `pkg/httpkit`: Chi router composition, strict JSON decoding, request IDs,
  recovery, access logging, CORS, and injectable rate limiting.
- `pkg/apikit`: `{code,msg,data}` envelopes and stable HTTP/application errors.
- `pkg/lifecycle`: cancellable components and bounded, reverse-order cleanup.
- `pkg/logging`: JSON `slog` construction and strict level parsing.
- `pkg/pagination`: normalized pages and metadata with a size cap of 100.
- `pkg/config`: typed environment configuration with validation.
- `pkg/migrations`: the embedded PostgreSQL migration set and its
  golang-migrate source driver (`FS` and `Source`).
- `pkg/modules/iam`, `pkg/modules/audit`, `pkg/modules/task`: domain and
  application behavior.
- `pkg/platform/...`: infrastructure adapters (database, httpserver,
  observability, queue, scheduler, cache, authcrypto, authcache, mailer,
  resilience, logging).
- `pkg/shared/...`: small cross-cutting contracts (apperror, pagination,
  requestcontext, response).

Only exported identifiers documented by GoDoc are part of the public contract.
The `internal/` tree remains private and is not a supported import surface:
application assembly (`internal/app`), command wiring (`cmd`), composition
bootstrap (`internal/bootstrap`), and integration fixtures
(`internal/testutil`).

## npm

The public package is `@lotusrain-net/backend-infrastructure-web@0.2.1`.

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

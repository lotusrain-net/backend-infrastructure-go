# SP-07 Delivery and Verification

## Ownership

`deployments/`, `.github/workflows/`, end-to-end test harness, final security and architecture checks.

## Deliverables

- Multi-stage non-root images for API, worker, scheduler, and migrate commands.
- Compose services for PostgreSQL, Redis with AOF, migration gate, API, worker, and scheduler.
- CI gates for test, race, vet, build, Staticcheck, Govulncheck, migrations, and Docker E2E.
- Placeholder-only environment templates and private service networking.

## Tests

- Fresh Compose startup, dependency health, graceful shutdown, login/RBAC/task/audit workflow, migration up/down/up, outage scenarios, and secret scanning.

## Completion

- Changed-files cleanup, complete regression, architecture review, security review, clean worktree, and local/remote SHA equality.

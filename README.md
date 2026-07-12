# Backend Infrastructure Go

Reusable Go backend infrastructure built with Chi, pgx/sqlc, PostgreSQL, Redis, and Asynq.

The repository is a clean implementation. The Python dashboard project is reference material only and is not a runtime or migration dependency.

## Status

Implementation follows the approved plans in `docs/plans/` and `.omx/plans/`.

## Deployment

The root `Dockerfile` builds one non-root image containing the API, worker, scheduler, and migration executables. Copy `.env.example` to a deployment-specific environment file and replace every `replace-` placeholder with a secret supplied by the deployment platform.

Validate and start the stack with:

```sh
docker compose --env-file .env -f deployments/compose.yml config
docker compose --env-file .env -f deployments/compose.yml up -d --build --wait
```

PostgreSQL and Redis are only attached to the internal Compose network and publish no host ports. Redis uses AOF persistence. API, worker, and scheduler startup is gated on healthy dependencies and successful database migrations.

For local delivery verification, run `scripts/verify-delivery.ps1`; `scripts/docker-smoke.ps1` additionally builds the image, starts a fresh stack, checks migrations and dependency health, and verifies graceful shutdown.

# Backend Infrastructure Go

Reusable Go backend infrastructure built with Chi, pgx/sqlc, PostgreSQL, Redis, and Asynq.

The repository is a clean implementation. The Python dashboard project is reference material only and is not a runtime or migration dependency.

The `web/` application is the approved frontend scaffold for the backend APIs. It is derived from a sanitized upstream dashboard shell and keeps only generic product infrastructure, not Douyin business pages or branded assets.

## Status

Implementation follows the approved plans in `docs/plans/` and `.omx/plans/`.

## Deployment

The root `Dockerfile` builds one non-root image containing the API, worker, scheduler, and migration executables. `.env.example` is a local HTTP Compose development profile. `./scripts/compose-up.sh` creates `.env` with generated development-only values on its first run. The profile uses ECR Public for Docker images and `goproxy.cn` for Go modules; both can be overridden in `.env`. For manual configuration, copy it to `.env`, replace every `replace-` placeholder with development-only values, and do not reuse it for deployment.

## Local Development

Backend-only checks remain available through the root `Makefile`. The frontend scaffold lives in `web/` and uses Node 20 with `npm`.

Run the backend locally:

```sh
go test ./...
go run ./cmd/api
```

Run the frontend locally against the backend:

```sh
cd web
npm ci
npm run dev
```

For direct local frontend development, point `API_PROXY_TARGET` at `http://localhost:8080`. The Compose stack sets `API_PROXY_TARGET=http://api:8080` automatically. `WEB_PORT` controls the published host port for the containerized frontend and defaults to `3000`.

Start the local HTTP development stack with one command:

```sh
./scripts/compose-up.sh
```

The first run creates a gitignored `.env` from `.env.example`, generates development-only database, JWT, and administrator secrets, then waits for all Compose health checks. The frontend is available at `http://127.0.0.1:3000`; use `ADMIN_EMAIL` and `ADMIN_PASSWORD` from `.env` to sign in. Public images default to `public.ecr.aws/docker`; set `IMAGE_REGISTRY` in `.env` to use another Docker-compatible mirror or an internal registry. Stop the stack with `docker compose --env-file .env -f deployments/compose.yml down`. When Make is installed, `make compose-up` and `make compose-down` provide the same commands.

To validate and start the local HTTP development stack manually:

```sh
docker compose --env-file .env -f deployments/compose.yml config
docker compose --env-file .env -f deployments/compose.yml up -d --build --wait
```

PostgreSQL and Redis are only attached to the internal Compose network and publish no host ports. Redis uses AOF persistence. API, worker, scheduler, and web startup is gated on dependency order, successful database migrations, and the web proxy healthcheck at `/api/healthz`.

The local profile binds the web app to `http://127.0.0.1:3000` and deliberately sets `COOKIE_SECURE=false`, so browser-managed HttpOnly session cookies work over HTTP. For production, supply a separate environment file with `APP_ENV=production`, `COOKIE_SECURE=true`, an HTTPS public endpoint, and an HTTPS `CORS_ALLOWED_ORIGINS` value. The Compose default remains `COOKIE_SECURE=true` when a deployment profile does not set it.

For local delivery verification, run `scripts/verify-delivery.ps1`; `scripts/docker-smoke.ps1` additionally builds the image, starts a fresh stack, checks migrations and dependency health, and verifies graceful shutdown.

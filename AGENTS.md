# AGENTS.md

## Project

This repository is a clean Go implementation of reusable backend infrastructure.

## Required workflow

- Follow test-driven development: add a failing test before production behavior.
- Keep domain packages independent from HTTP, PostgreSQL, Redis, and Asynq adapters.
- Use `context.Context` for I/O boundaries.
- Do not add Python, Alembic, Funboost, Playwright, or Douyin business code.
- Run `go test ./...`, `go vet ./...`, and `go build ./...` before completion.
- Keep commits focused and use the Lore Commit Protocol from the source workspace instructions.

## Package boundaries

- `cmd` and `internal/bootstrap` own composition.
- `internal/app` and `internal/testutil` are private application assembly.
- `pkg/platform` owns infrastructure adapters.
- `pkg/modules` owns domain and application behavior.
- `pkg/shared` contains small cross-cutting contracts only.
- `pkg/config` and the other `pkg/*` packages form the public, importable surface.

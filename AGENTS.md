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
- `internal/platform` owns infrastructure adapters.
- `internal/modules` owns domain and application behavior.
- `internal/shared` contains small cross-cutting contracts only.

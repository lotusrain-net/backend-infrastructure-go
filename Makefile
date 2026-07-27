.PHONY: test race vet build staticcheck govulncheck sqlc-verify delivery-verify docker-build compose-config compose-up compose-down docker-smoke web-install web-lint web-typecheck web-test web-build web-check backend-check check

STATICCHECK_VERSION ?= v0.7.0
GOVULNCHECK_VERSION ?= v1.6.0

test:
	go test ./...

race:
	go test -race ./...

vet:
	go vet ./...

build:
	go build ./...

staticcheck:
	go run honnef.co/go/tools/cmd/staticcheck@$(STATICCHECK_VERSION) ./...

govulncheck:
	go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...

sqlc-verify:
	pwsh -File ./db/verify-sqlc.ps1

delivery-verify:
	pwsh -File ./scripts/verify-delivery.ps1

docker-build:
	docker build -t backend-infrastructure-go:local .

compose-config:
	docker compose --env-file .env.example -f deployments/compose.yml config

compose-up:
	./scripts/compose-up.sh

compose-down:
	docker compose --env-file .env -f deployments/compose.yml down

docker-smoke:
	pwsh -File ./scripts/docker-smoke.ps1

web-install:
	npm --prefix ./web ci

web-lint:
	npm --prefix ./web run lint

web-typecheck:
	npm --prefix ./web run typecheck

web-test:
	npm --prefix ./web run test

web-build:
	npm --prefix ./web run build

web-check: web-install web-lint web-typecheck web-test web-build

backend-check: test race vet build staticcheck govulncheck sqlc-verify delivery-verify

check: backend-check web-check

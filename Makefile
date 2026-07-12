.PHONY: test race vet build staticcheck govulncheck sqlc-verify delivery-verify docker-build compose-config docker-smoke check

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

docker-smoke:
	pwsh -File ./scripts/docker-smoke.ps1

check: test race vet build staticcheck govulncheck sqlc-verify delivery-verify

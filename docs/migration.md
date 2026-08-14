# Migration Guide

## Go module

Change the dependency to the canonical module and use a released tag:

```sh
go get github.com/jyysy/backend-infrastructure-go@v0.1.0
go mod tidy
go mod vendor
```

Replace the old module prefix in imports with
`github.com/jyysy/backend-infrastructure-go`. The `web/go.mod` file is a
separate nested module at `/web`; it is not released as a Go submodule.

During local simultaneous development, use an uncommitted `go.work` or a
temporary `replace`. Release and CI verification must set `GOWORK=off` and use
the formal tag. Downstream repositories own and commit their own `vendor/`.

Repository owners publish and fetch the private Go module over SSH:

```sh
git remote set-url origin git@github.com:jyysy/backend-infrastructure-go.git
ssh -T git@github.com
git fetch --tags
```

Developers configure the private namespace and map HTTPS GitHub module fetches
to SSH without embedding credentials in module metadata:

```sh
go env -w GOPRIVATE=github.com/jyysy/*
git config --global url."ssh://git@github.com/".insteadOf https://github.com/
```

CI must use a read-only deploy key, GitHub App, or organization-scoped
credential for this private module. Never put a personal access token in a Git
URL, script, `go.mod`, or committed configuration.

## Web package

Install the public package from npm and import shared capabilities by export:

```sh
npm install @purplevoid/backend-infrastructure-web@0.1.0
```

Use `@purplevoid/backend-infrastructure-web/api`, `/theme`, `/ui`,
`/ui/client`, `/patterns`, and `/styles.css` for the listed public surfaces.
Keep authentication refresh, session clearing, theme persistence, QueryClient,
Next navigation, stores, providers, Sidebar/AppShell, and business DTOs in a
private adapter inside the consuming application. Do not import those concerns
from the npm package.

The package is ESM-only and ships declarations and CSS. React and ReactDOM
must be supplied by the consumer as peer dependencies.

Repository maintainers should follow [the release procedure](releasing.md) for
tag ordering, npm Trusted Publishing setup, and post-release verification.

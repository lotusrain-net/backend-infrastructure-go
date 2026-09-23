# Compatibility Policy

Go `v0.2.x` and npm `0.2.x` are compatible releases. Existing exported Go
identifiers, npm export paths, component props, JSON fields, and documented CSS
tokens remain supported throughout the minor line.

The `v0.2.0` line moved the reusable Go packages from `internal/` to `pkg/` and
renamed the module path to `github.com/lotusrain-net/backend-infrastructure-go`;
the npm package moved to `@lotusrain-net/backend-infrastructure-web`. These are
the public entry points as of `v0.2.0`.

The following require `v0.3.0` or later: deleting an exported identifier,
changing a function signature, changing an npm export path, removing or
renaming a component prop, changing a wire-format field, or removing a CSS
token. Additive APIs may ship in a minor release when they do not change
existing behavior.

Private `internal/` packages (`internal/app`, `internal/bootstrap`,
`internal/testutil`), `cmd/`, and the Next application are deliberately outside
this policy. They may change with the application and are never a substitute
for a public package import.

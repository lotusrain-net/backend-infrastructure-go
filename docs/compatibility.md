# Compatibility Policy

Go `v0.1.x` and npm `0.1.x` are compatible releases. Existing exported Go
identifiers, npm export paths, component props, JSON fields, and documented CSS
tokens remain supported throughout the minor line.

The following require `v0.2.0` or later: deleting an exported identifier,
changing a function signature, changing an npm export path, removing or
renaming a component prop, changing a wire-format field, or removing a CSS
token. Additive APIs may ship in a minor release when they do not change
existing behavior.

Private `internal/` packages and the Next application are deliberately outside
this policy. They may change with the application and are never a substitute
for a public package import.

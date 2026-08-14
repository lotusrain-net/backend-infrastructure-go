# Release Procedure

The `v0.1.0` release publishes the Go module and
`@purplevoid/backend-infrastructure-web@0.1.0` from the same commit. The
workflow is deliberately limited to this version and must be dispatched from
the `main` branch.

## One-time repository setup

- Configure the GitHub `release` environment with required reviewers.
- Protect the `v0.1.0` tag with a repository ruleset that prevents updates and
  deletion while allowing the release workflow to create it.
- Keep the repository's default `GITHUB_TOKEN` workflow permission read-only;
  the release workflow grants tag and GitHub Release writes only to the jobs
  that need them.
- Do not add an npm token to GitHub Actions. The publish job requests an OIDC
  identity using `id-token: write`.

For an npm package that already exists, configure its trusted publisher on
npmjs.com with these exact values:

| Setting | Value |
| --- | --- |
| Organization or user | `jyysy` |
| Repository | `backend-infrastructure-go` |
| Workflow filename | `release.yml` |
| Environment | `release` |
| Allowed action | `npm publish` |

npm trusted publishers are configured from an existing package's settings.
At the time this procedure was written,
`@purplevoid/backend-infrastructure-web` did not yet exist. Its first verified
tarball must therefore be published once by the owner interactively; then
configure the table above and use only OIDC for subsequent releases. Do not
store that interactive credential in the repository or GitHub Actions.
The package manifest's `repository.url` must exactly match the GitHub
repository. npm trusted publishing generates provenance automatically only
when that source repository is public; OIDC publishing remains available for a
private source repository, but npm does not generate provenance for it.

For that one-time bootstrap, download the tarball artifact produced by the
failed release run after its tag job has succeeded, verify its recorded
`SHA256SUMS`, and run from the artifact directory:

```sh
sha256sum --check SHA256SUMS
npm publish purplevoid-backend-infrastructure-web-0.1.0.tgz --access public
```

Configure the trusted publisher immediately afterward, then rerun the same
release workflow. The retry verifies that npm's integrity is exactly the
verified tarball before it creates the GitHub Release.

## Release execution

Before dispatching, confirm that the intended commit is on `main`, that normal
CI passed, and that the npm package version is `0.1.0`. Dispatch the `Release`
workflow with the only accepted input, `v0.1.0`.

The workflow performs these operations in order:

1. It checks out the triggering commit by SHA, runs Go tests, race detection,
   vet, build, staticcheck, govulncheck, dependency boundaries, and the
   external Go consumer fixture.
2. It runs the frontend lint, typecheck, unit tests, package and Next builds,
   publint, dry-run packing, and Vite/Next tarball consumer fixtures.
3. It retains the real npm tarball installed by both consumer fixtures,
   records its source commit and SHA-256 digest, and transfers that exact
   artifact between jobs.
4. It creates an annotated `v0.1.0` tag for the triggering commit and verifies
   its peeled remote target before any npm publication.
5. It verifies the released Go module through the tag, publishes the verified
   tarball through npm Trusted Publishing, verifies both registries, and
   creates the GitHub Release. npm adds provenance automatically when the
   source repository is public.

The workflow can be rerun after an interrupted release. It accepts an existing
tag only when it is annotated and peels to the triggering commit. It accepts
an existing npm version only when its registry integrity equals the verified
tarball; otherwise it fails rather than treating a different artifact as a
successful retry.

For the first publication, the `bootstrap` job intentionally stops after tag
creation and artifact verification and prints the owner-only command above.
No token is accepted by the workflow. Once the owner publishes that artifact
and configures Trusted Publishing, rerunning verifies the existing tarball and
completes the GitHub Release. The next unpublished npm version exercises the
OIDC publisher; private source repositories do not receive npm provenance.

After completion, independently verify:

```sh
GOWORK=off go list -m github.com/jyysy/backend-infrastructure-go@v0.1.0
npm view @purplevoid/backend-infrastructure-web@0.1.0 version dist.integrity
```

The nested `web/go.mod` is not tagged separately. Downstream Go repositories
must use the formal tag with `GOWORK=off`, then run `go mod tidy` and
`go mod vendor` and commit their own `vendor/` directory.

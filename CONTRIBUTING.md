# Contributing to Kan

Thanks for your interest in contributing! This guide covers how to build, test, and
make changes. For architecture and deeper context, see [AGENTS.md](AGENTS.md),
[SPEC.md](SPEC.md), and [docs/COMPAT.md](docs/COMPAT.md).

Please also read [AI_POLICY.md](AI_POLICY.md) before opening a PR - it sets out the
rules for AI-assisted contributions.

## Prerequisites

- **Go** 1.24+ (see `go.mod`)
- **Node** - the version pinned in [`.nvmrc`](.nvmrc) (`nvm use` will pick it up). The
  exact version matters: the embedded frontend bundle must build reproducibly (see
  [The embedded frontend rule](#the-embedded-frontend-rule)).

## Getting started

```bash
git clone https://github.com/amterp/kan
cd kan
make build   # builds the frontend, then the Go binary
make test    # runs Go + frontend tests
```

`make help` lists all targets. The most useful ones:

| Target            | What it does                                              |
|-------------------|-----------------------------------------------------------|
| `make build`      | Build frontend (`web/`) then the Go binary into `bin/kan` |
| `make test`       | `go test ./...` and the frontend (vitest) suite           |
| `make validate-go`| Go gate: gofmt check, `go vet`, tests, build              |
| `make fmt`        | Format Go code in place                                   |
| `make verify-dist`| Rebuild the frontend and confirm the committed embed matches source |
| `make serve`      | Build then run the local server                           |
| `make vuln`       | Scan Go dependencies with govulncheck                     |

## Frontend dev loop

For fast iteration on the web UI, run the Vite dev server and a Go server that proxies
to it (instead of serving the embedded bundle):

```bash
cd web && npm run dev          # Vite on http://localhost:5173
# in another terminal:
go run -tags dev ./cmd/kan serve
```

The `dev` build tag swaps the embedded file server for a proxy to Vite
(`internal/api/embed_dev.go`), so you get hot reload without rebuilding.

## The embedded frontend rule

The Go binary embeds the built frontend from `internal/api/dist/` via `//go:embed`
(`internal/api/embed.go`). That directory is **committed** so that `go install` and
`go build` work from a clean checkout without Node.

This means: **if you change anything under `web/`, you must rebuild and commit the
embedded assets** so the binary ships your reviewed source:

```bash
cd web && npm ci && npm run build
git add internal/api/dist
```

CI verifies this by rebuilding from source and diffing against the committed assets.
If they don't match, the frontend check fails and a bot leaves a comment telling you
exactly what to run. Build with the `.nvmrc` Node version (`nvm use`) and `npm ci` so
the output is reproducible - a different Node version can produce a different bundle
that fails the check.

## Schema changes and migrations

Kan versions its on-disk schema and migrates user data forward. If you change the
shape of a board config, card, or other persisted file, you must bump the version and
provide a migration. See [docs/COMPAT.md](docs/COMPAT.md) and the checklist at the top
of `internal/version/version.go`. In short:

1. Bump the relevant `Current*Version` constant in `internal/version/version.go`
2. Add a `MinKanVersion` entry for the new schema string
3. Add a migration step in `internal/service/migrate_service.go`
4. Add fixtures under `internal/service/testdata/migrations/vN/`
5. Add migration + idempotency tests in `migrate_service_test.go`

Migration tests are strict (e.g. `TestMigrationFixturesComplete` requires a fixture
for every version), so run `go test ./internal/service/...` after schema changes.

## Submitting changes

- Branch off `main`, keep changes focused, and make sure `make validate-go`,
  `make test`, and (if you touched `web/`) `make verify-dist` pass locally.
- All CI checks must pass on your PR.
- Keep auxiliary docs in sync when behavior changes - see the maintenance notes in
  [AGENTS.md](AGENTS.md) (CLI docs, skill file, COMPAT, etc.).

Contributions are accepted under the project's [Apache 2.0 license](LICENSE).

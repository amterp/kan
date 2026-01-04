# AGENTS.md

Guidance for AI agents working with this codebase.

**Maintenance**: Keep auxiliary files (AGENTS.md, README.md, etc.) up to date when making relevant changes.

**Note**: SPEC.md is a temporary bootstrapping document and will be removed once the initial implementation stabilizes.

## Project Overview

Kan is a file-based kanban board CLI tool. All data lives as plain files in `.kan/`—no database, no server, no external dependencies. Works with any VCS (or none). The Go binary embeds a React web frontend served via `kan serve`.

## Build & Development Commands

**Prefer using `./dev`** for common tasks—it handles the frontend build automatically. The script is written in [Rad](https://github.com/amterp/rad) (see [syntax reference](https://github.com/amterp/rad/blob/main/SYNTAX.md)).

```bash
./dev -b          # Build (frontend + Go binary)
./dev -v          # Build and run tests
./dev -p          # Build, validate, push to origin
./dev -s          # Build, validate, run ./kan serve
```

Manual commands (for reference):

```bash
# Build Go binary (embeds web frontend from internal/api/dist)
go build ./cmd/kan

# Run tests
go test ./...

# Run a single test
go test ./internal/store -run TestFileCardStore_CreateAndGet

# Frontend (from web/ directory)
cd web
npm install
npm run build    # outputs to internal/api/dist, embedded in binary
npm run dev      # dev server with HMR, proxies /api/* to localhost:8080
npm run lint     # eslint
```

During development: run `go run ./cmd/kan serve` in one terminal and `npm run dev` in `web/` for hot-reloading.

## Architecture

### Layered Backend Structure

```
cmd/kan/main.go
  └── internal/cli/app.go (App struct - DI container)
       ├── Stores (internal/store/) - File I/O, interface-based for testability
       ├── Services (internal/service/) - Business logic
       ├── Resolvers (internal/resolver/) - ID/alias resolution
       └── API (internal/api/) - HTTP handlers for web frontend
```

**Key pattern**: All dependencies flow through the `App` struct created in `NewApp()`. CLI commands and API handlers access stores/services through this container.

### Data Flow

- **CLI**: `internal/cli/*.go` → App → Services → Stores → Files
- **Web**: React hooks → API client → Go HTTP handlers → Services → Stores → Files

### One File Per Card

Each card is a separate JSON file in `.kan/boards/<board>/cards/<flexid>.json`. This is intentional—VCS merges at file level, so concurrent card additions rarely conflict.

### JSON for Data, TOML for Config

- Cards: JSON (machine-oriented)
- Board/global config: TOML (human-editable)

## Key Abstractions

- **Stores** are interfaces (`CardStore`, `BoardStore`, `GlobalStore`) with file-based implementations. Tests use temp directories.
- **Prompter** interface (`HuhPrompter` for interactive, `NoopPrompter` for `-I` flag) allows the same service code to work in both modes.
- **Card.CustomFields** uses custom JSON marshaling to flatten board-defined fields into the top-level JSON object.

## File Locations

- Global config: `~/.config/kan/config.toml`
- Board data: `.kan/boards/<name>/config.toml` and `.kan/boards/<name>/cards/*.json`
- Web build output: `internal/api/dist/` (embedded via `//go:embed`)

## CLI Framework

Uses [ra](https://github.com/amterp/ra) for command-line parsing. Commands are registered in `internal/cli/root.go`.

## Testing Patterns

Tests create temp directories and clean up via deferred functions. See `setupTestCardStore()` in `internal/store/card_store_test.go` for the standard pattern.

## Schema Versioning

When modifying schemas (`internal/version/version.go`):

1. **Bump version constants** — `CurrentCardVersion`, `CurrentBoardVersion`, `CurrentGlobalVersion`
2. **Update MinKanVersion map** — Maps schema to minimum Kan version
3. **Add migration fixtures** — `internal/service/testdata/migrations/vN/`
4. **Add migration tests** — In `migrate_service_test.go`
5. **Update COMPAT.md** — Document the schema change

Tests enforce invariants 2 and 3 automatically:
- `TestMinKanVersionCompleteness` fails if MinKanVersion entry missing
- `TestMigrationFixturesComplete` fails if fixtures missing

See `COMPAT.md` for design rationale and compatibility policy.

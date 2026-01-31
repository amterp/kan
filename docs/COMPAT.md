# Kan Compatibility Policy

This document describes Kan's file schema design, versioning strategy, and forward/backward compatibility policy. It serves as an internal reference for contributors and maintainers—documenting not just what we decided, but *why*.

> **Note**: The `.kan/` file schema is internal to Kan. While we design it cleanly (as if it could be public), we make no stability guarantees to external tooling. This document is for Kan developers, not a public API contract.

## Background

Kan is a file-based kanban board CLI tool. All data lives as plain files in a `.kan/` directory—no database, no server, no external dependencies. This design means:

- Your kanban data can be version-controlled alongside your code (with any VCS)
- Multiple team members can work on the same board with VCS handling merges
- Files are human-readable and diffable

Because the files are the persistence layer, schema decisions have long-term consequences. This document captures those decisions and their rationale.

## File Structure Overview

```
.kan/
  config.toml               # Project configuration (name, favicon)
  boards/
    <board-name>/
      config.toml           # Board configuration (columns, labels, settings)
      cards/
        <id>.json           # One file per card

~/.config/kan/config.toml   # Global user configuration
```

### Why One File Per Card?

**Decision**: Each card is a separate JSON file rather than all cards in one file.

**Rationale**: VCS merges at file level. Two people adding different cards to the same board rarely conflict—most VCS tools see two new files being added. Two people editing the *same* card will conflict, but that's a genuine conflict requiring human resolution anyway.

**Alternative considered**: Single `cards.json` with all cards. Rejected because every card change would conflict with every other card change during merge.

## Schema Versioning

Each file type has an independent schema version.

### Card Files (JSON)

```json
{
  "_v": 1,
  "id": "22Dm2sjM",
  "title": "Fix login bug",
  "labels": ["bug"],
  ...
}
```

**Decision**: Use `_v` (integer) for card schema version. Missing `_v` is treated as version 0 (legacy/unversioned).

**Why `_v` instead of `$v`?**

We considered `$v` (following JSON Schema conventions), but:
- Shell escaping friction: `jq '."$v"'` vs `jq ._v`
- `_` prefix is already reserved, so `_v` fits naturally
- Simpler for debugging and scripting during development

**Why integer, not string?** Cards are machine-oriented (read/written by Kan). Integers are compact and easily comparable.

### Board Configuration (TOML)

```toml
kan_schema = "board/1"
id = "22B8gm7g"
name = "main"

[[columns]]
name = "Backlog"
...
```

**Decision**: Use `kan_schema = "type/version"` string format.

**Why string path instead of integer?** Config files are human-edited. `"board/1"` is more readable than just `1`, and the type prefix makes it clear what's being versioned. Also allows future flexibility (e.g., `"board/1-beta"`).

### Global Configuration (TOML)

```toml
kan_schema = "global/1"
editor = "vim"
...
```

Same pattern as board config.

### Project Configuration (TOML)

```toml
kan_schema = "project/1"
id = "p_abc123"
name = "my-project"

[favicon]
background = "#3b82f6"
icon_type = "letter"
letter = "M"
```

**Decision**: Project config is stored at `.kan/config.toml` (not board-specific).

**Why separate from board config?** Project-level settings (name, favicon) apply to all boards in a project. Board config is per-board. Keeping them separate follows single-responsibility.

**Auto-creation**: Unlike board/card schemas which require `kan init`, project config is auto-created on first CLI interaction if missing. This provides a graceful upgrade path for projects created before project config existed. The `EnsureInitialized` pattern (in `ProjectStore`) generates a stable project ID that persists for the lifetime of the project.

**Project ID**: Each project has a unique ID (e.g., `p_abc123`). This ID is used to derive deterministic values like favicon colors. The ID is stable even if the project name changes, ensuring visual consistency.

### Why Independent Versions?

**Decision**: Card schema and board schema evolve independently. A card at `_v: 2` can exist in a board at `board/1`.

**Rationale**: Different file types change at different rates. Coupling them would force unnecessary migrations. If we add a new card field, we shouldn't have to bump board schema version.

### Version Semantics

**Decision**: Version 0 is implicit (legacy/unversioned), version 1 is the first explicit schema.

- **v0 (implicit)**: Missing `_v` in card or `kan_schema` in config. This represents legacy data from before versioning was implemented. Cards at v0 may have a `column` field which is no longer used.
- **v1**: First versioned schema. Cards have `_v: 1`, no `column` field. Board configs have `kan_schema = "board/1"`.
- **board/2**: Converts labels from first-class `[[labels]]` to custom fields with type `"tags"`. Adds `card_display.badges` for label visibility.
- **board/3**: Adds optional `[[pattern_hooks]]` for running commands when cards are created with matching titles.
- **board/4 (current)**: Adds optional `wanted` field to custom field schemas. Wanted fields emit warnings when missing from cards.

Running `kan migrate` upgrades data to the current version. The migration is incremental—v0 → v1 → v2 → v3 → v4.

**Rationale**: Strict versioning—Kan refuses to read files without version stamps (or with incompatible versions). This catches schema drift early and forces explicit migration.

### Wanted Fields (board/4)

**Added in**: board/4

Wanted fields are custom fields that should ideally be set on every card. When a card is missing a wanted field:

- CLI commands (`kan add`, `kan edit`) print warnings
- `--strict` flag converts warnings to errors (blocking the operation)
- `kan doctor` reports cards missing wanted fields
- Frontend shows asterisk on wanted field labels and warning icon on cards

```toml
[custom_fields.type]
type = "enum"
wanted = true  # NEW in board/4
options = [
  { value = "bug", color = "#dc2626" },
  { value = "feature", color = "#22c55e" },
]
```

**Design rationale**: Wanted fields encourage data quality without enforcing rigid schemas. The `--strict` flag is opt-in for workflows that need hard enforcement, while the default warning behavior is forgiving for quick card creation.

**Migration**: board/3 → board/4 only updates the schema version. Existing custom fields gain an implicit `wanted = false` (the default).

### Pattern Hooks (board/3)

**Added in**: board/3

Pattern hooks allow running external commands when cards are created with titles matching specified patterns. This is useful for integrations like:

- Syncing with external issue trackers (Jira, GitHub Issues)
- Auto-populating card descriptions from external sources
- Triggering notifications or webhooks

```toml
[[pattern_hooks]]
name = "jira-sync"
pattern_title = "^[A-Z]+-\\d+$"  # Matches JIRA-123, PROJ-456, etc.
command = "~/.kan/hooks/jira-sync.sh"
timeout = 60  # Optional, defaults to 30s
```

**Execution model**:
1. Card is fully created and persisted
2. Matching hooks run sequentially (in config order)
3. Hook receives `<card_id> <board_name>` as arguments
4. Hooks can use `kan` CLI to modify the card
5. Hook stdout is shown to user
6. Non-zero exit shows warning but doesn't roll back card creation

**Design rationale**: Hooks run after persistence to ensure the card exists before modification. Sequential execution prevents race conditions. Non-fatal failures ensure card creation succeeds even if external services are unavailable.

## Reserved Field Prefixes

**Decision**: Reserve `_*` and `kan_*` prefixes for Kan's internal use.

**Enforcement**: Validate on write. If a user tries to create a custom field named `_priority` or `kan_status`, Kan rejects it with a terse error.

**Rationale**: Protects Kan's ability to add new core fields in the future without colliding with user-defined custom fields. This is internal namespace hygiene—we don't need to explain the "why" to users, just prevent the collision.

**Why not just document "don't use these"?** Users don't read docs. Validation catches mistakes before they become migration problems.

## Custom Fields

Kan supports user-defined custom fields on cards. These are stored flat at the top level of the card JSON:

```json
{
  "_v": 1,
  "id": "22Dm2sjM",
  "title": "Fix login bug",
  "priority": "high",
  "assignee": "alice"
}
```

Here `priority` and `assignee` are custom fields defined in the board's configuration.

### Why Flat Instead of Nested?

**Decision**: Custom fields live at the top level of card JSON, not in a nested `custom_fields` or `x` object.

**Alternatives considered**:

1. **Nested object** (`"x": {"priority": "high"}`): Cleaner separation but worse ergonomics for jq, API queries, templates.

2. **Prefixed fields** (`x_priority`): Explicit but verbose. Makes every custom field ugly.

3. **Flat at top level** (chosen): Best ergonomics. Risk is future collision with core fields.

**Why we chose flat**: The ergonomics win for now. Reserved prefixes (`_*`, `kan_*`) protect our namespace. If collisions become a problem, we have escape hatches.

### Collision Handling

**Decision**: Detect-and-refuse strategy. If a future Kan version introduces a core field that conflicts with an existing custom field:

```
Error: Card abc123 has custom field 'priority' which conflicts
with core field in kan 1.2. Run: kan migrate --rename-field priority x_priority
```

**Rationale**: User decides how to resolve. No silent data loss or shadowing. Migration command makes the fix explicit.

### Escape Hatch: `x_` Prefix

**Decision**: The `x_` prefix is documented as collision-safe for custom fields.

If users want guaranteed collision-free naming, they can use `x_priority` instead of `priority`. This is optional—most users won't need it.

**Why `x_`?** Short, obvious, won't collide with anything we'd add to core. If we later mandate namespacing, `x_` is the migration target.

## Column Membership

**Decision**: Cards do NOT store which column they belong to. Column membership is determined solely by the `card_ids` arrays in board config:

```toml
[[columns]]
name = "In Progress"
card_ids = ["22Dm2sjM", "22DnGln7"]
```

### Why Remove Column from Cards?

**Previous state**: Cards had a `column` field, and board config had `card_ids`. Two sources of truth.

**Problem**: Two sources of truth = two places to get out of sync. The code comment said "backward compat" but there was no v0 to be compatible with—it was vestigial design.

**Alternative considered**: Keep `column` as a "cache" for:
- Orphan recovery if board config corrupted
- Git forensics (card history shows moves)
- Standalone card reads by external tools

**Why we rejected the cache argument**:
- A cache without invalidation is a bug waiting to happen
- If board config is lost, so is the card file (same Git history)
- External tooling isn't our concern (schema is internal)
- Single source of truth is simpler and safer

**Migration**: Removing `column` was included in the v0 → v1 migration (the initial versioning migration). `kan migrate` removes the `column` field from legacy cards.

## Compatibility Guarantees

### Pre-v1 (Current Phase)

Kan is in early development. During this phase:

- **Breaking schema changes are allowed** with a migration path
- Run `kan migrate` after upgrading if schema changed
- CHANGELOG documents all schema-breaking changes
- Migration tooling provided—no manual file editing required

**Rationale**: This flexibility lets us fix design mistakes before they're locked in forever. Fear of breaking early adopters shouldn't calcify bad decisions for future users.

### v1 Criteria

**Decision**: v1 is an intentional stability declaration, not a passive observation.

We will release v1 when we're *ready to commit* to external stability, not just when the schema happens to be stable. Specifically:

1. Migration tooling is battle-tested
2. 3+ external users are using Kan without issues
3. Web UI is feature-complete for core workflows
4. We've announced intent (v0.9.0 "last breaking release" pattern)

**Why not "60 days stable"?** Time-based criteria encourages either rushing to hit arbitrary deadlines or never reaching them. v1 should be a deliberate choice.

**Announcement pattern**: Before v1, ship a release explicitly marked as "last breaking release before v1." This gives early adopters a heads-up and a final window for feedback.

### Post-v1

After v1, we follow semantic versioning:

- **Patch releases** (1.0.x): Bug fixes, no schema changes
- **Minor releases** (1.x.0): New features, backward-compatible schema additions
- **Major releases** (x.0.0): Breaking schema changes with migration path

Guarantees:
- New Kan can always read schemas from the previous major version
- `kan migrate` will upgrade data to the current schema
- Clear error messages when schema version is incompatible

## Version Compatibility Behavior

### New Kan Reading Old Schema

**Behavior**: Auto-migrate on write, or prompt user.

```
Warning: Board 'main' uses schema board/1, current is board/2.
Run 'kan migrate' to upgrade.
```

### Old Kan Reading New Schema

**Behavior**: Refuse with clear error and required version.

```
Error: This board requires Kan >= 0.5.0 (found board/3, supports up to board/2).
Please upgrade: brew upgrade kan
```

**Rationale**: Old Kan cannot safely operate on schemas it doesn't understand. Clear error prevents silent corruption.

### Version Mapping

**Decision**: Hardcoded in binary, documented externally.

```go
var MinKanVersion = map[int]string{
    1: "0.1.0",
    2: "0.3.0",
}
```

**Why not in schema files?** Simpler. Migration code already knows version semantics. External spec documents the mapping for humans.

## Migration

The `kan migrate` command handles schema upgrades:

```bash
# Migrate all boards in current project
kan migrate

# Preview what would change
kan migrate --dry-run
```

### VCS Noise from Bulk Migration

**Concern**: When user first runs `kan migrate`, every card file gets touched to add `_v: 1`. This creates a large diff.

**Mitigation**: Document "run `kan migrate` in its own commit." One-time pain for schema clarity going forward. Can suggest `git blame --ignore-rev` in output.

## Key Design Decisions Summary

### JSON for Cards, TOML for Config

**Rationale**: Cards are machine-oriented—read/written by Kan, need custom field flexibility. Config files are human-edited—need readability. Each format serves its audience.

### Immutable Card IDs, Mutable Aliases

**Rationale**: IDs (like `22Dm2sjM`) are used for relationships (parent cards, cross-board references). These must be stable. Aliases (like `fix-login-bug`) are for human convenience and can change when titles change.

### Cross-Board Card Moves

**Decision**: When card moves from board A to board B:

- **Alias collision**: Auto-generate new alias (`fix-bug` → `fix-bug-2`). Same as duplicate handling.
- **Column mapping**: Use target board's `default_column`, allow `--column` override.
- **Parent references**: If card.parent points to card in source board, it remains valid (cross-board refs use stable IDs).

### Custom Field Type Evolution

**Decision**: Board owner's problem. If they change a custom field from enum to string, existing values become strings. Kan can warn but won't block:

```
Warning: Changing 'priority' from 'enum' to 'string'.
15 cards have enum values. These will be treated as strings.
```

## Deferred Decisions

These are known considerations we've explicitly deferred:

### Comment Scalability

**Concern**: Cards embed comments. 1000-comment card = huge JSON file.

**Deferral**: Theoretical problem. Revisit if real users hit it. Escape hatch: split to separate file (`.kan/boards/<board>/comments/<card-id>.json`) with `"comments": "$ref:comments"` in card.

### Orphan Custom Field Schemas

**Concern**: Board config can define custom field schemas that no cards use.

**Deferral**: Valid to define unused schemas—they're templates for future cards. Nice-to-have: `kan board cleanup` to remove unused definitions.

## Design Principles

These principles guided our decisions:

1. **Single source of truth**: Don't store the same fact in two places.
2. **Design for VCS**: File-per-card, atomic changes, merge-friendly.
3. **Explicit versioning**: Every file type declares its schema version.
4. **Fail loudly**: Unknown schemas cause errors, not silent corruption.
5. **Migration over compatibility hacks**: Clean breaks with tooling beat accumulated workarounds.
6. **Internal quality, external freedom**: Design clean schemas internally; don't make external stability promises until ready.

## Summary Table

| Aspect | Decision | Rationale |
|--------|----------|-----------|
| Card versioning | `"_v": 1` (integer) | Compact, no shell escaping |
| Config versioning | `kan_schema = "type/1"` | Human-readable, typed |
| Project config | `.kan/config.toml` with auto-creation | Graceful upgrade for existing projects |
| Project ID | Stable ID for deterministic derivation | Favicon colors persist even if name changes |
| Reserved prefixes | `_*`, `kan_*` | Protect future core fields |
| Custom fields | Flat at top level | Best ergonomics; reserved prefixes protect us |
| Collision handling | Detect-and-refuse | User decides, no silent loss |
| Column storage | Board config only | Single source of truth |
| Pre-v1 breaking changes | Allowed with migration | Fix mistakes before lock-in |
| v1 criteria | Intentional declaration | Not time-based |
| Post-v1 breaking changes | Major version bump | Semver stability |
| External tooling | Best-effort, not guaranteed | Schema is internal |

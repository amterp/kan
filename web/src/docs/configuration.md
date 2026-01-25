# Configuration

Kan uses TOML files for configuration. This page documents the board configuration structure.

## File Locations

- **Board config**: `.kan/boards/<board-name>/config.toml`
- **Global user config**: `~/.config/kan/config.toml`

## Board Configuration

A board's `config.toml` defines its structure, columns, custom fields, and display options.

### Minimal Example

```toml
kan_schema = "board/2"
id = "k7xQ2m"
name = "main"

[[columns]]
name = "todo"
color = "#6b7280"

[[columns]]
name = "in-progress"
color = "#f59e0b"

[[columns]]
name = "done"
color = "#10b981"
```

### Full Example

```toml
kan_schema = "board/2"
id = "k7xQ2m"
name = "main"
default_column = "backlog"

[[columns]]
name = "backlog"
color = "#6b7280"

[[columns]]
name = "in-progress"
color = "#f59e0b"

[[columns]]
name = "done"
color = "#10b981"

[custom_fields.type]
type = "enum"
options = [
  { value = "feature", color = "#16a34a" },
  { value = "bug", color = "#dc2626" },
  { value = "task", color = "#4b5563" },
]

[custom_fields.labels]
type = "tags"
options = [
  { value = "blocked", color = "#dc2626" },
  { value = "urgent", color = "#f59e0b" },
]

[card_display]
type_indicator = "type"
badges = ["labels"]

[link_rules]
JIRA = "https://jira.example.com/browse/$1"
GH = "https://github.com/org/repo/issues/$1"
```

## Fields Reference

### Root Fields

| Field | Required | Description |
|-------|----------|-------------|
| `kan_schema` | Yes | Schema version (e.g., `"board/2"`) |
| `id` | Yes | Unique board identifier (auto-generated) |
| `name` | Yes | Board name (also used as directory name) |
| `default_column` | No | Column for new cards via `kan add` (defaults to first column) |

### Columns

Columns are defined as an ordered array:

```toml
[[columns]]
name = "backlog"
color = "#6b7280"
```

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Column name (must be unique within board) |
| `color` | Yes | Hex color for column header |
| `card_ids` | No | Ordered list of card IDs (managed by Kan) |

**Default columns** when creating a new board: `backlog`, `next`, `in-progress`, `done`.

### Custom Fields

See [Custom Fields](/docs/custom-fields) for full documentation.

```toml
[custom_fields.<field-name>]
type = "enum"  # or "tags", "string", "date"
options = [    # required for enum/tags
  { value = "...", color = "#..." },
]
```

### Card Display

Controls how custom fields appear on cards in the board view:

```toml
[card_display]
type_indicator = "type"      # enum field shown as badge
badges = ["labels"]          # tags fields shown as chips
metadata = ["assignee"]      # any fields shown as text
```

See [Custom Fields](/docs/custom-fields#card-display) for details on display slots.

### Link Rules

Auto-link patterns for references like ticket IDs:

```toml
[link_rules]
JIRA = "https://jira.example.com/browse/$1"
GH = "https://github.com/org/repo/issues/$1"
```

See [Link Rules](/docs/link-rules) for full documentation.

## Global User Configuration

The global config at `~/.config/kan/config.toml` stores user preferences:

```toml
editor = "vim"  # Editor for interactive editing

[projects]
my-project = "/Users/name/src/my-project"

[repos."/Users/name/src/my-project"]
default_board = "features"
```

| Field | Description |
|-------|-------------|
| `editor` | Editor for interactive mode (falls back to `$EDITOR`, then `vim`) |
| `projects` | Registry of known Kan projects (populated automatically) |
| `repos.<path>.default_board` | Default board when a repo has multiple boards |

## Managing Columns via CLI

```bash
# Add a column
kan column add review
kan column add review --color "#9333ea" --position 2

# Rename a column
kan column rename review code-review

# Change column color
kan column edit review --color "#ec4899"

# Reorder columns
kan column move review --position 1
kan column move review --after backlog

# Delete a column
kan column delete review
```

See [CLI Reference](/docs/cli#column) for all column commands.

# Configuration

Kan uses TOML files for configuration. This page documents the board configuration structure.

## File Locations

- **Board config**: `.kan/boards/<board-name>/config.toml`
- **Global user config**: `~/.config/kan/config.toml`

## Board Configuration

A board's `config.toml` defines its structure, columns, custom fields, and display options.

### Minimal Example

```toml
kan_schema = "board/8"
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
kan_schema = "board/8"
id = "k7xQ2m"
name = "main"
default_column = "backlog"

[[columns]]
name = "backlog"
color = "#6b7280"
description = "Planned work not yet started"

[[columns]]
name = "in-progress"
color = "#f59e0b"
description = "Currently being worked on"
limit = 5

[[columns]]
name = "done"
color = "#10b981"
description = "Completed work"

[custom_fields.type]
type = "enum"
options = [
  { value = "feature", color = "#16a34a" },
  { value = "bug", color = "#dc2626" },
  { value = "task", color = "#4b5563" },
]

[custom_fields.labels]
type = "enum-set"
options = [
  { value = "blocked", color = "#dc2626" },
  { value = "urgent", color = "#f59e0b" },
]

[custom_fields.topics]
type = "free-set"

[card_display]
type_indicator = "type"
badges = ["labels"]

[[link_rules]]
name = "JIRA"
pattern = "([A-Z]+-\\d+)"
url = "https://jira.example.com/browse/{1}"

[[link_rules]]
name = "GitHub"
pattern = "#(\\d+)"
url = "https://github.com/org/repo/issues/{1}"
```

## Fields Reference

### Root Fields

| Field | Required | Description |
|-------|----------|-------------|
| `kan_schema` | Yes | Schema version (e.g., `"board/5"`) |
| `id` | Yes | Unique board identifier (auto-generated) |
| `name` | Yes | Board name (also used as directory name) |
| `default_column` | No | Column for new cards via `kan add` (defaults to first column) |

### Columns

Columns are defined as an ordered array:

```toml
[[columns]]
name = "in-progress"
color = "#f59e0b"
description = "Currently being worked on"
limit = 5
```

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Column name (must be unique within board) |
| `color` | Yes | Hex color for column header |
| `description` | No | Purpose of this workflow stage |
| `limit` | No | Max cards allowed (0 or omitted = no limit) |
| `card_ids` | No | Ordered list of card IDs (managed by Kan) |

**Default columns** when creating a new board: `backlog`, `next`, `in-progress`, `done`.

**Column Limits**: When a column has a `limit`, adding or moving cards into it is refused once the limit is reached. Column headers show the count as `(X/Y)` when a limit is set. This is a core kanban practice for controlling flow.

### Custom Fields

See [Custom Fields](/docs/custom-fields) for full documentation.

```toml
[custom_fields.<field-name>]
type = "enum"  # or "enum-set", "free-set", "string", "date"
wanted = true  # optional: warn if field is missing
options = [    # required for enum/enum-set
  { value = "...", color = "#..." },
]
```

#### Wanted Fields

Mark a field as `wanted = true` to encourage its use without strict enforcement:

- `kan add` and `kan edit` print warnings when wanted fields are missing
- Use `--strict` flag to convert warnings to errors
- `kan doctor` reports cards missing wanted fields
- Frontend shows asterisk on wanted field labels and warning icon on cards

This is useful for fields like "type" that should ideally be set on every card but shouldn't block quick card creation.

### Card Display

Controls how custom fields appear on cards in the board view:

```toml
[card_display]
type_indicator = "type"      # enum field shown as badge
badges = ["labels"]          # set fields shown as chips
metadata = ["assignee"]      # any fields shown as text
```

See [Custom Fields](/docs/custom-fields#card-display) for details on display slots.

### Link Rules

Auto-link patterns for references like ticket IDs:

```toml
[[link_rules]]
name = "JIRA"
pattern = "([A-Z]+-\\d+)"
url = "https://jira.example.com/browse/{1}"

[[link_rules]]
name = "GitHub"
pattern = "#(\\d+)"
url = "https://github.com/org/repo/issues/{1}"
```

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Human-readable name for the rule |
| `pattern` | Yes | Regex pattern with capture groups |
| `url` | Yes | URL template using `{0}` for full match, `{1}`, `{2}`, etc. for groups |

See [Link Rules](/docs/link-rules) for full documentation.

### Pattern Hooks

Pattern hooks run commands when cards are created with titles matching specified patterns. This is useful for integrating with external systems.

```toml
[[pattern_hooks]]
name = "jira-sync"
pattern_title = "^[A-Z]+-\\d+$"  # Matches JIRA-123, PROJ-456
command = "~/.kan/hooks/jira-sync.sh"
timeout = 60  # Optional, defaults to 30s
```

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Human-readable hook name (for logs/errors) |
| `pattern_title` | Yes | Regex pattern to match card titles |
| `command` | Yes | Path to executable (see note below) |
| `timeout` | No | Timeout in seconds (default: 30) |

**Important:** The `command` field must be a **path to an executable file**, not a shell command with arguments. For example:
- ✅ `"~/.kan/hooks/my-hook.sh"` — direct path to script with shebang
- ✅ `"/usr/local/bin/my-tool"` — absolute path to binary
- ❌ `"python script.py"` — won't work (not a shell command)
- ❌ `"./hook.sh --verbose"` — won't work (arguments not parsed)

The `~` prefix is expanded to your home directory. If you need to pass arguments or use shell features, create a wrapper script.

**Execution details:**
- Hooks run **after** the card is fully created and saved
- Multiple matching hooks run sequentially in config order
- Hook receives `<card_id> <board_name>` as command-line arguments
- Hooks can use `kan` CLI commands to modify the card
- Hook stdout is shown to the user
- Non-zero exit code shows a warning but doesn't roll back card creation

**Example hook script** (`~/.kan/hooks/jira-sync.sh`):
```bash
#!/bin/bash
CARD_ID="$1"
BOARD="$2"

# Fetch JIRA ticket description and update the card
TITLE=$(kan show "$CARD_ID" -b "$BOARD" --format '{{.Title}}')
DESCRIPTION=$(curl -s "https://jira.example.com/rest/api/2/issue/$TITLE" | jq -r '.fields.description')

kan edit "$CARD_ID" -b "$BOARD" -d "$DESCRIPTION"
echo "Synced description from JIRA"
```

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
kan column add review --limit 5

# Rename a column
kan column rename review code-review

# Edit column properties
kan column edit review --color "#ec4899"
kan column edit review --description "Cards under review"
kan column edit review --limit 3
kan column edit review --limit 0    # Clear limit

# Reorder columns
kan column move review --position 1
kan column move review --after backlog

# Delete a column
kan column delete review
```

See [CLI Reference](/docs/cli#column) for all column commands.

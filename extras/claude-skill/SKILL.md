---
name: kan
description: Manage kanban boards using the Kan CLI. Use when working with tasks, cards, boards, columns, or project tracking via the kan command.
---

# Kan CLI

[Kan](https://github.com/amterp/kan) is a file-based kanban board CLI. All data lives in `.kan/` as plain files.

Full documentation is available at [amterp.github.io/kan/docs](https://amterp.github.io/kan/docs).

## Getting Help

Every command supports `--help` for detailed usage:

```bash
kan --help
kan add --help
kan column --help
```

## Board Setup Wizard

When a user asks to create a new board (`kan board create`) or initialize a new Kan project (`kan init`), run this interactive setup process rather than creating a board with defaults. This applies to both new projects and additional boards.

Before writing any config, consult the [Kan documentation](https://amterp.github.io/kan/docs) to double-check available options and TOML structure.

### Step 1: Understand the Project and User

Before suggesting anything, learn about what the user is building:
- What kind of project is this? (software, personal, team, open source, etc.)
- What workflow are they trying to track?
- Is this their first board or are they adding to an existing setup?

This context lets you make relevant, project-specific suggestions in later steps.

Also gauge whether the user is already familiar with Kan. If it's unclear, ask. If they're new, explain concepts (like wanted fields, card display roles, enum-set types) as they come up during the wizard. If they're experienced, keep it snappy.

### Step 2: Columns

Ask the user what columns they want. Offer these templates as inspiration - they are not rigid. The user can mix, match, rename, add, or remove columns freely.

**Simple** - Good default for most projects:

| Column | Description |
|--------|-------------|
| backlog | Planned work not yet started |
| next | Ready to be picked up next |
| in-progress | Currently being worked on |
| done | Completed work |

**Prioritized Backlog** - Splits the backlog for triage:

| Column | Description |
|--------|-------------|
| backlog-lo | Low priority planned work |
| backlog-hi | High priority planned work |
| next | Ready to be picked up next |
| in-progress | Currently being worked on |
| done | Completed work |

**With Ideas** - Adds a staging area for uncommitted thoughts:

| Column | Description |
|--------|-------------|
| uncommitted | Ideas and thoughts not yet committed to |
| backlog | Planned work not yet started |
| next | Ready to be picked up next |
| in-progress | Currently being worked on |
| done | Completed work |

**Full** - Prioritized backlog with ideas column:

| Column | Description |
|--------|-------------|
| uncommitted | Ideas and thoughts not yet committed to |
| backlog-lo | Low priority planned work |
| backlog-hi | High priority planned work |
| next | Ready to be picked up next |
| in-progress | Currently being worked on |
| done | Completed work |

**Important**: Every column should have a description. Descriptions serve as self-documentation and help guide AI agents using the board. Suggest descriptions if the user doesn't provide them.

The first column in the list becomes the default column for new cards.

### Step 3: Custom Fields

Walk the user through what fields they want on their cards. For each field, discuss:
- What type? (`string`, `enum`, `enum-set`, `free-set`, `date`)
- What are the options/values? (for `enum` and `enum-set` types)
- Descriptions for the field itself and each of its options
- Should this field be **wanted**? (If the user is new, explain: wanted fields generate a warning when a card is created without them, encouraging consistent metadata across cards)

#### Type Field (Strongly Recommended)

Unless the user explicitly doesn't want one, recommend a `type` field (`enum`, wanted). This categorizes what kind of work a card represents. Suggest these default values:

| Value | Color | Description |
|-------|-------|-------------|
| bug | #dc2626 | A defect in existing functionality |
| enhancement | #2563eb | An improvement to existing functionality |
| feature | #16a34a | New functionality to be added |
| chore | #4b5563 | Maintenance, refactoring, or housekeeping |

The user can customize these - add, remove, rename, or change descriptions to fit their project.

#### Other Fields

The `type` field alone is often enough, especially for new boards. Lean toward simplicity. But if the user asks about other fields, or if their project clearly calls for one based on what you learned in Step 1, here are some examples to get their thinking going:

- **labels** (`enum-set`) - Flexible tagging. Example values: `ai-hi` (well-suited for autonomous AI work - low complexity, little judgment needed), `ai-lo` (likely suitable for AI but less certain). Cards without an AI label are implicitly not suitable for autonomous AI work.
- **effort** (`enum`) - T-shirt sizing for rough estimation: xs, s, m, l, xl
- **area** (`enum-set`) - What part of the project a card touches, e.g. backend, frontend, infra, docs

Don't actively suggest these - just mention that additional fields are possible and offer examples if the user is interested.

### Step 4: Card Display

Configure how fields appear on cards in the web UI and CLI output. Apply sensible defaults without overwhelming the user with implementation details:

- If the user has a `type` field, set it as the `type_indicator` (colored badge on each card).
- Any `enum-set` or `free-set` fields (like `labels`) should go in `badges` (chip-style labels on cards).
- Other fields like dates or effort can go in `metadata` (small text below the card).

For new users, just set these defaults and briefly mention that their type will show as a colored badge on cards and any tags will show as labels. Don't expose config key names like `type_indicator` or `badges` unless the user asks for details.

### Step 5: Pattern Hooks (Optional)

Pattern hooks automate actions when cards are created with titles matching a pattern. This is entirely optional - offer it, but don't push it.

If the user set up a `type` field, suggest the **type shortcut hook**: when creating a card in the web UI with `!bug` anywhere in the title (e.g. `!bug Fix login crash` or `Fix login crash !bug`), the hook automatically strips the `!bug` and sets the type field to `bug`. It also supports aliases like `!feat` for `feature` and `!enh` for `enhancement`.

For newcomers, keep the explanation focused on what it does rather than how: "This is a convenience shortcut. Instead of setting the type field manually after creating a card, you can just include `!bug`, `!feat`, `!chore`, etc. anywhere in your title and it gets set for you automatically."

If the user wants this hook:

1. **Detect Rad**: Silently run `rad -v`. If Rad is installed, ask whether they'd prefer the hook written in Rad or Bash. If not installed, silently default to Bash.

2. **Create the hook script** at `.kan/hooks/type-shortcut.rad` (or `.sh` if writing Bash). Here's the Rad version - translate to Bash if needed:

```rad
#!/usr/bin/env rad
---
Pattern hook for setting card type from title shortcuts like !bug, !feat, !chore.
Receives card_id and board_name as arguments from Kan pattern hooks.
---
args:
    card_id str
    board_name str

type_aliases = {
    "feat": "feature",
    "enh": "enhancement",
}

code, stdout = quiet $`kan show {card_id} -b {board_name} --json`
if code != 0:
    exit(1)

card_data = parse_json(stdout)
title = card_data["card"]["title"]

if not matches(title, "![a-zA-Z]+", partial=true):
    exit(0)

match_result = replace(title, "(?i).*!([a-z]+).*", "$1")
type_keyword = lower(match_result)

card_type = type_keyword
if type_keyword in type_aliases:
    card_type = type_aliases[type_keyword]

new_title = replace(title, "(?i)\\s*![a-z]+\\s*", " ")
new_title = trim(new_title)
new_title = replace(new_title, "\\s+", " ")

quiet $`kan edit {card_id} -b {board_name} -t "{new_title}" -f type={card_type}`

print("Set type to '{card_type}'")
```

3. **Make executable**: `chmod +x .kan/hooks/type-shortcut.rad` (or `.sh`)

4. **Add the hook** to the board config:

```toml
[[pattern_hooks]]
name = "type-shortcut"
pattern_title = "![a-zA-Z]+"
command = ".kan/hooks/type-shortcut.rad"  # or .sh for bash
```

### Step 6: Execute and Verify

1. **Create the board**:
   - New project: `kan init -c <columns> -n <board-name> -p <project-name>`
   - Additional board: `kan board create <board-name>`, then edit the TOML to set up columns

2. **Edit the board config** at `.kan/boards/<name>/config.toml` to add:
   - Column descriptions (and column definitions if using `kan board create`)
   - Custom fields with types, options, descriptions, colors, and wanted flags
   - Card display configuration

3. **Verify** the result: `kan board describe`

Reference TOML format for custom fields and card display:

```toml
[custom_fields.type]
type = "enum"
wanted = true
description = "The category of work this card represents"

[[custom_fields.type.options]]
value = "bug"
color = "#dc2626"
description = "A defect in existing functionality"

[[custom_fields.type.options]]
value = "enhancement"
color = "#2563eb"
description = "An improvement to existing functionality"

[custom_fields.labels]
type = "enum-set"
description = "Flexible tags for categorization"

[[custom_fields.labels.options]]
value = "ai-hi"
description = "Well-suited for autonomous AI work"

[[custom_fields.labels.options]]
value = "ai-lo"
description = "Likely suitable for AI but less certain"

[card_display]
type_indicator = "type"
badges = ["labels"]
```

Column descriptions are added to the `[[columns]]` entries:

```toml
[[columns]]
name = "backlog"
color = "#6b7280"
description = "Planned work not yet started"
card_ids = []
```

## Initialize

```bash
kan init                              # Create .kan/ in current directory
kan init -l .kanboard                 # Custom location
kan init -c todo,doing,done           # Custom columns
kan init -n myboard                   # Custom board name
kan init -p myproject                 # Custom project name for favicon/title
kan init -c a,b,c -n project          # Both custom columns and name
```

| Flag | Description |
|------|-------------|
| `-l, --location` | Custom location for .kan directory |
| `-c, --columns` | Comma-separated column names (default: backlog,next,in-progress,done) |
| `-n, --name` | Board name (default: main) |
| `-p, --project-name` | Project name for favicon and page title (default: git repo or directory name) |

## Adding Cards

```bash
kan add "Fix login bug"                         # Add card, prompted for details
kan add "Fix login bug" -c backlog              # Add to specific column
kan add "Feature" -b features -c todo           # Specify board and column
kan add "Title" "Description here" -c backlog   # Title + description
kan add "Subtask" -p 12                         # Add as child of card 12
kan add "Task" -f priority=high -f type=bug     # Add with custom fields
```

| Flag | Description |
|------|-------------|
| `-b, --board` | Target board |
| `-c, --column` | Target column |
| `-p, --parent` | Parent card ID or alias |
| `-f, --field` | Custom field (key=value, repeatable) |
| `--strict` | Error if wanted fields are missing (default: warn) |

## Listing Cards

```bash
kan list                     # List all cards grouped by column
kan list -c done             # Filter by column
kan list -b myboard          # Filter by board
```

## Showing Card Details

```bash
kan show 12              # Show card by ID
kan show fix             # Show card by alias or partial match
kan show fix -b myboard  # Specify board
```

## Editing Cards

```bash
kan edit 12                              # Edit interactively
kan edit fix -t "New title"              # Update title
kan edit fix -c done                     # Move to column
kan edit fix -d "New description"        # Update description
kan edit fix -f priority=low             # Update custom field
```

| Flag | Description |
|------|-------------|
| `-b, --board` | Board name |
| `-t, --title` | Set card title |
| `-d, --description` | Set card description |
| `-c, --column` | Move card to column |
| `-p, --parent` | Set parent card |
| `-a, --alias` | Set explicit alias |
| `-f, --field` | Set custom field (key=value, repeatable) |
| `--strict` | Error if wanted fields are missing (default: warn) |

## Board Management

```bash
kan board create features    # Create a new board
kan board list               # List all boards
kan board describe           # Show board documentation (columns, fields, settings)
kan board describe --json    # Machine-readable board docs
```

## Column Management

```bash
kan column add review                                    # Add column to end
kan column add review --color "#9333ea"                  # With custom color
kan column add review --position 2                       # Insert at position
kan column add review --description "Cards under review" # With description
kan column delete review                 # Delete column
kan column rename review code-review     # Rename column
kan column edit review --color "#ec4899" # Change column color
kan column edit review --description "Updated purpose"   # Change description
kan column list                          # List columns
kan column move review --position 1      # Reorder column
kan column move review --after backlog   # Insert after another
```

## Comments

```bash
kan comment add fix-login "Found the issue"   # Add comment to card
kan comment add fix-login                     # Add comment (opens editor)
kan comment edit c_9kL2x "Updated text"       # Edit comment by ID
kan comment delete c_9kL2x                    # Delete comment
```

| Flag | Description |
|------|-------------|
| `-b, --board` | Board name |

## Web Interface

```bash
kan serve                # Start web UI (opens browser)
kan serve -p 8080        # Custom port
kan serve --no-open      # Don't auto-open browser
```

## Migration

```bash
kan migrate              # Migrate data to current schema
kan migrate --dry-run    # Preview changes without applying
kan migrate --all        # Migrate all projects in global config
kan migrate --all --dry-run  # Preview changes for all projects
```

| Flag        | Description                                        |
|-------------|----------------------------------------------------|
| `--dry-run` | Show what would be changed without modifying files |
| `--all`     | Migrate all projects registered in global config   |

## Health Checks

```bash
kan doctor               # Check for consistency issues
kan doctor --fix         # Apply automatic fixes
kan doctor --dry-run     # Preview fixes without applying
kan doctor -b main       # Check specific board only
kan doctor --json        # Machine-readable output
```

**Exit codes:** 0 = no errors (warnings OK), 1 = errors found

**Fixable issues:** orphaned cards, missing card references, duplicate IDs, invalid default column, invalid parent refs.

## Global Flags

| Flag | Description |
|------|-------------|
| `-I, --non-interactive` | Fail instead of prompting for input |
| `--json` | Output results as JSON (supported by: show, list, add, edit, board list, column list, comment add, doctor) |

## Board Configuration

Board configuration is stored in `.kan/boards/<boardname>/config.toml`. Key features:

### Pattern Hooks

Run commands when cards are created with matching titles:

```toml
[[pattern_hooks]]
name = "jira-sync"
pattern_title = "^[A-Z]+-\\d+$"  # Matches JIRA-123, PROJ-456
command = "~/.kan/hooks/jira-sync.sh"
timeout = 60  # Optional, defaults to 30s
```

Hooks receive `<card_id> <board_name>` as arguments and run after card creation. The `command` must be a path to an executable (not a shell command with arguments). Use `~` for home directory.

### Link Rules

Auto-link patterns in card descriptions:

```toml
[[link_rules]]
name = "jira"
pattern = "([A-Z]+-\\d+)"
url = "https://jira.example.com/browse/{1}"
```

## JSON Output

Use `--json` for programmatic access to Kan data:

```bash
kan show fix-login --json | jq .card.title
kan list --json | jq '.cards | length'
kan add "New task" --json | jq .card.id
kan board list --json | jq .boards
kan column list --json | jq '.columns[].name'
kan comment add fix-login "Note" --json | jq .comment.id
```

## Tips

- Cards are identified by flexible IDs: numeric ID, alias, or partial match
- Use `-I` for scripting to ensure commands fail rather than prompt
- Use `--json` for programmatic access to card data

## Documentation

Full documentation is available at [amterp.github.io/kan/docs](https://amterp.github.io/kan/docs). Consult the docs when setting up boards or editing config files to verify available options and TOML structure.

---
name: kan
description: Manage kanban boards using the Kan CLI. Use when working with tasks, cards, boards, columns, or project tracking via the kan command.
---

# Kan CLI

[Kan](https://github.com/amterp/kan) is a file-based kanban board CLI. All data lives in `.kan/` as plain files.

## Getting Help

Every command supports `--help` for detailed usage:

```bash
kan --help
kan add --help
kan column --help
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

## Board Management

```bash
kan board create features    # Create a new board
kan board list               # List all boards
```

## Column Management

```bash
kan column add review                    # Add column to end
kan column add review --color "#9333ea"  # With custom color
kan column add review --position 2       # Insert at position
kan column delete review                 # Delete column
kan column rename review code-review     # Rename column
kan column edit review --color "#ec4899" # Change column color
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
```

## Global Flags

| Flag | Description |
|------|-------------|
| `-I, --non-interactive` | Fail instead of prompting for input |
| `--json` | Output results as JSON (supported by: show, list, add, edit, board list, column list, comment add) |

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

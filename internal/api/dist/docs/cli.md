# CLI Reference

The `kan` command line tool lets you manage your kanban boards from the terminal.

## Getting Started

Initialize Kan in your project directory:

```bash
kan init
```

This creates a `.kan/` directory with a default board named "main" and four columns: backlog, next, in-progress, done.

You can customize the initialization:

```bash
kan init -l .kanboard              # Custom location
kan init -c todo,doing,done        # Custom columns
kan init -n myboard                # Custom board name
kan init -p myproject              # Custom project name for favicon/title
kan init -c a,b,c -n project       # Both custom columns and name
```

## Git Worktree Support

When you run `kan` commands inside a git worktree, Kan automatically uses the board from the main worktree. All worktrees share the same kanban board by default - no extra setup needed.

If you run `kan init` inside a worktree, Kan warns that this will create a separate, independent board. If you confirm, the worktree gets its own `.kan/` directory with `worktree_independent = true` in its project config.

## Commands

### init

Initialize Kan in the current directory.

| Flag                 | Description                                                                   |
|----------------------|-------------------------------------------------------------------------------|
| `-l, --location`     | Custom location for .kan directory (relative path)                            |
| `-c, --columns`      | Comma-separated column names (default: backlog,next,in-progress,done)         |
| `-n, --name`         | Board name (default: main)                                                    |
| `-p, --project-name` | Project name for favicon and page title (default: git repo or directory name) |

### board

Manage boards.

**Create a board:**

```bash
kan board create "features"
```

**List all boards:**

```bash
kan board list
```

**Delete a board:**

```bash
kan board delete features
```

Deleting the last remaining board is not allowed. If the deleted board was set as the default, the default is cleared automatically.

**Describe a board:**

Show full board documentation including columns, custom fields, card display settings, link rules, and pattern hooks.

```bash
kan board describe
kan board describe features
kan board describe --json
```

| Flag       | Description            |
|------------|------------------------|
| `-b, --board` | Target board       |
| `--json`   | Machine-readable output |

### column

Manage columns within a board.

**Add a column:**

```bash
kan column add review
kan column add review --color "#9333ea" --position 2
kan column add review --description "Cards under code review"
kan column add review --limit 5
```

| Flag                | Description                                       |
|---------------------|---------------------------------------------------|
| `-b, --board`       | Target board                                      |
| `-C, --color`       | Hex color (default: auto from palette)            |
| `-d, --description` | Description of the column's purpose               |
| `-l, --limit`       | Max cards allowed in column (0 = no limit)        |
| `-p, --position`    | Insert position (0-indexed, default: end)         |

**Delete a column:**

```bash
kan column delete review
```

| Flag          | Description                                                              |
|---------------|--------------------------------------------------------------------------|
| `-b, --board` | Target board                                                             |

**Rename a column:**

```bash
kan column rename review code-review
```

| Flag          | Description  |
|---------------|--------------|
| `-b, --board` | Target board |

**Edit column properties:**

```bash
kan column edit review --color "#ec4899"
kan column edit review --description "Updated purpose"
kan column edit review --limit 3
kan column edit review --limit 0    # Clear limit
```

| Flag                | Description                                       |
|---------------------|---------------------------------------------------|
| `-b, --board`       | Target board                                      |
| `-C, --color`       | New hex color                                     |
| `-d, --description` | New description for the column                    |
| `-l, --limit`       | Column limit (0 = clear, >0 = set max cards)      |

**List columns:**

```bash
kan column list
kan column list -b features
```

| Flag          | Description  |
|---------------|--------------|
| `-b, --board` | Target board |

**Move/reorder a column:**

```bash
kan column move review --position 1
kan column move review --after backlog
```

| Flag             | Description              |
|------------------|--------------------------|
| `-b, --board`    | Target board             |
| `-p, --position` | Target index (0-indexed) |
| `-a, --after`    | Insert after this column |

### add

Add a new card.

```bash
kan add "Fix login bug"
kan add "Update docs" "Description goes here"
```

| Flag           | Description                                   |
|----------------|-----------------------------------------------|
| `-b, --board`  | Target board                                  |
| `-c, --column` | Target column                                 |
| `-p, --parent` | Parent card ID or alias                       |
| `--position`   | Insert at index (0 = top, -1 = end, negatives count from end) |
| `--before`     | Insert before this card (ID or alias)         |
| `--after`      | Insert after this card (ID or alias)          |
| `-f, --field`  | Custom field in key=value format (repeatable; set-typed fields also accept comma-separated values) |
| `--strict`     | Error if wanted fields are missing (default: warn) |
| `-g, --global` | Target the designated global board (see [global](#global)) |

`--position`, `--before`, and `--after` are mutually exclusive. By default a card
is appended to the end of its column. Without `-c`, the card is placed in the
anchor card's column.

`--position` is most useful for the boundaries (`0` = top, `-1` = end). For a spot
in the middle, prefer `--before`/`--after` with a card you can see in `kan list`,
since `kan list` does not print numeric indices to count against.

**Examples:**

```bash
kan add "Task title" -c backlog
kan add "Feature" -b features -c todo -f priority=high
kan add "Urgent" -c backlog --position 0    # top of backlog
kan add "Follow-up" --after fix-login       # right after fix-login, in its column
kan add "Buy milk" -g                       # add to the global board from anywhere
```

### show

Display card details.

```bash
kan show fix-login-bug
kan show fix-log            # Partial match (fuzzy)
```

Card identifiers accept partial substring matches against a card's alias or ID
(case-insensitive, min 3 chars). A single match resolves; multiple matches
produce a disambiguation error listing up to 5 candidates. Exact ID or alias
always wins over fuzzy. This applies to `show`, `history`, `edit`, `delete`, and
`comment add`.

`show` also reports how long the card has been in its current column
(e.g. `Column: review (3 days)`).

| Flag           | Description                                                |
|----------------|------------------------------------------------------------|
| `-b, --board`  | Board name                                                 |
| `-g, --global` | Target the designated global board (see [global](#global)) |

### history

Show a card's column transition timeline - which columns it has passed through
and how long it spent in each. Kan records each transition on the card with its
event time, so this is accurate regardless of your commit cadence. Content edits
(title/description) are intentionally left to your VCS history.

```bash
kan history fix-login-bug
kan history fix-login --json
```

| Flag           | Description                                                |
|----------------|------------------------------------------------------------|
| `-b, --board`  | Board name                                                 |
| `-g, --global` | Target the designated global board (see [global](#global)) |

### list

List cards, grouped by column.

```bash
kan list
kan list -b features
kan list -c done
kan list --sort priority            # order each column by the priority field
kan list --sort priority --descending  # high → low instead of low → high
```

| Flag               | Description                                                |
|--------------------|------------------------------------------------------------|
| `-b, --board`      | Filter by board                                            |
| `-c, --column`     | Filter by column                                           |
| `-s, --sort`       | Sort cards within each column by a custom field (e.g. `priority`) instead of by manual position |
| `-d, --descending` | Sort descending (use with `--sort`)                        |
| `-g, --global`     | Target the designated global board (see [global](#global)) |

`--sort` orders cards by a custom field's value. For `enum`/`enum-set` fields
the order follows the option order defined in your board config (so
`priority` sorts `low → medium → high` when that's how the options are listed),
not alphabetically. Cards missing a value for the field are always listed last,
in both directions. Sorting is non-destructive — it only changes the listing
order, never the cards' saved positions.

### edit

Edit an existing card. Run without flags for interactive mode, or use flags to apply changes directly.

```bash
kan edit fix-login-bug
kan edit fix-login-bug -t "New title" -c done
```

| Flag                | Description                                       |
|---------------------|---------------------------------------------------|
| `-b, --board`       | Board name                                        |
| `-t, --title`       | Set card title                                    |
| `-d, --description` | Set card description                              |
| `-c, --column`      | Move card to column                               |
| `-p, --parent`      | Set parent card ID or alias                       |
| `--position`        | Move to index in column (0 = top, -1 = end, negatives count from end) |
| `--before`          | Move before this card (ID or alias)               |
| `--after`           | Move after this card (ID or alias)                |
| `-a, --alias`       | Set explicit alias                                |
| `-f, --field`       | Set custom field in key=value format (repeatable; set-typed fields also accept comma-separated values) |
| `--strict`          | Error if wanted fields are missing (default: warn) |
| `-g, --global`      | Target the designated global board (see [global](#global)) |

`--position`, `--before`, and `--after` are mutually exclusive and can reorder a
card within its current column or place it precisely when moving between columns.
Without `-c`, the card is placed in the anchor card's column. As with `add`,
prefer `--before`/`--after` over `--position` for non-boundary spots.

**Examples:**

```bash
kan edit fix-login -c doing --position 0   # top of doing
kan edit fix-login --before deploy         # reorder relative to deploy
kan edit fix-login --position -1           # bottom of its column
```

### delete

Delete a card.

```bash
kan delete fix-login-bug
```

| Flag           | Description                                                |
|----------------|------------------------------------------------------------|
| `-b, --board`  | Board name                                                 |
| `-g, --global` | Target the designated global board (see [global](#global)) |

### serve

Start the web interface.

```bash
kan serve
kan serve -p 8080
kan serve --no-open
```

| Flag         | Description                       |
|--------------|-----------------------------------|
| `-p, --port` | Port to listen on (default: 5260). When unspecified, auto-increments if in use. When specified explicitly, errors out if unavailable. |
| `--no-open`  | Don't open browser automatically  |

### comment

Manage card comments.

**Add a comment:**

```bash
kan comment add fix-login-bug "Found the issue in session.go"
kan comment add fix-login-bug  # Opens editor for body
```

| Flag           | Description                                                |
|----------------|------------------------------------------------------------|
| `-b, --board`  | Board name                                                 |
| `-g, --global` | Target the designated global board (see [global](#global)) |

The first argument is the card ID or alias. The second argument is the comment body - if omitted, your editor opens to
write the comment.

**Edit a comment:**

```bash
kan comment edit c_9kL2x "Updated comment text"
kan comment edit c_9kL2x  # Opens editor with existing text
```

| Flag           | Description                                                |
|----------------|------------------------------------------------------------|
| `-b, --board`  | Board name                                                 |
| `-g, --global` | Target the designated global board (see [global](#global)) |

**Delete a comment:**

```bash
kan comment delete c_9kL2x
```

| Flag           | Description                                                |
|----------------|------------------------------------------------------------|
| `-b, --board`  | Board name                                                 |
| `-g, --global` | Target the designated global board (see [global](#global)) |

### commit

Stage and commit kan data files to git.

```bash
kan commit                              # Commit with default message
kan commit -m "update board"           # Custom commit message
```

| Flag             | Description                                     |
|------------------|-------------------------------------------------|
| `-m, --message`  | Commit message (default: "chore: update kan files") |

Only kan data files are committed - any other staged changes are left untouched.

Fails if not in a git repository or if kan is not initialized.

### global

Designate a single board as the *global board* so you can act on it from any
working directory with the `-g`/`--global` flag - handy for an "inbox" board you
capture to from anywhere, without `cd`-ing to its project.

```bash
kan global set            # designate the current project's board (interactive / single-board)
kan global set inbox      # designate a specific board by name
kan global show           # show the current designation (and warn if it's stale)
kan global unset          # clear the designation
```

Once set, `-g` works across the card commands - `add`, `list`, `show`, `history`,
`edit`, `delete`, and `comment add`/`edit`/`delete`:

```bash
kan add -g "Buy milk"        # from anywhere, lands on the global board
kan list -g                  # list the global board's cards
kan edit -g buy-milk -c done # move a global-board card
```

`-g` targets the global board's *project*, with the designated board as its
default. An explicit `-b` still overrides it, so `kan add -g -b other "..."`
adds to a different board in the same project. Commands print which board they
acted on so a mistaken target is visible.

There is **no implicit fallback**: commands never silently use the global board
based on your working directory - `-g` must be explicit. Running a bare command
outside any kan project still errors rather than capturing to the global board.

### migrate

Migrate board data to current schema version.

```bash
kan migrate
kan migrate --dry-run
kan migrate --all
kan migrate --all --dry-run
```

| Flag        | Description                                        |
|-------------|----------------------------------------------------|
| `--dry-run` | Show what would be changed without modifying files |
| `--all`     | Migrate all projects registered in global config (prompts per project) |

### doctor

Check board data for consistency issues and optionally fix them.

```bash
kan doctor
kan doctor --fix
kan doctor --dry-run
kan doctor -b main
kan doctor --json
```

| Flag          | Description                                         |
|---------------|-----------------------------------------------------|
| `--fix`       | Apply automatic fixes for issues with deterministic solutions |
| `--dry-run`   | Show what fixes would be applied without making changes |
| `-b, --board` | Check only a specific board (default: all)          |

**Exit codes:**

- `0`: No errors (warnings are OK)
- `1`: Errors found

**Issues detected:**

- **Errors** (must be fixed):
  - `MALFORMED_BOARD_CONFIG`: Board config.toml fails to parse
  - `MALFORMED_CARD`: Card JSON fails to parse
  - `MISSING_CARD_FILE`: Card ID in column but file not found (fixable)
  - `ORPHANED_CARD`: Card file not in any column (fixable)
  - `DUPLICATE_CARD_ID`: Same ID in multiple columns (fixable)

- **Warnings** (should be addressed):
  - `SCHEMA_OUTDATED`: Board/card needs migration (run `kan migrate`)
  - `INVALID_DEFAULT_COLUMN`: References missing column (fixable)
  - `INVALID_CARD_DISPLAY`: References missing custom field (fixable)
  - `INVALID_LINK_RULE`: Regex doesn't compile
  - `INVALID_PATTERN_HOOK`: Regex doesn't compile
  - `MISSING_HOOK_FILE`: Pattern hook references non-existent file
  - `INVALID_PARENT_REF`: Parent points to non-existent card (fixable)
  - `MISSING_WANTED_FIELDS`: Card is missing fields marked as `wanted`
  - `MALFORMED_GLOBAL_CONFIG`: Global config.toml fails to parse
  - `GLOBAL_SCHEMA_OUTDATED`: Global config needs migration

### completion

Output shell completion scripts for TAB completion of commands, flags, board names, card IDs/aliases, and column names.

```bash
kan completion bash
kan completion zsh
```

To enable, add one of these to your shell profile (e.g. `~/.zshrc` or `~/.bashrc`):

```bash
eval "$(kan completion zsh)"
eval "$(kan completion bash)"
```

## Global Flags

| Flag                    | Description                                                                              |
|-------------------------|------------------------------------------------------------------------------------------|
| `-I, --non-interactive` | Fail instead of prompting for missing input                                              |
| `--json`                | Output results as JSON (supported by: show, list, add, edit, board list, column list, comment add, doctor) |

## JSON Output

Use the `--json` flag for programmatic access to Kan data. Output is structured with wrapper objects for
forward-compatibility:

```bash
# Get card details as JSON
kan show fix-login --json
# Output: {"card": {...}}

# List all cards as JSON
kan list --json
# Output: {"cards": [...]}

# Create a card and get the result as JSON
kan add "New task" --json
# Output: {"card": {...}}

# Edit a card and get the updated result as JSON
kan edit fix-login -t "New title" --json
# Output: {"card": {...}}

# List boards as JSON
kan board list --json
# Output: {"boards": ["main", "features"]}

# List columns as JSON
kan column list --json
# Output: {"columns": [{"name": "backlog", "color": "#...", "card_count": 5}, ...]}

# Add a comment and get the result as JSON
kan comment add fix-login "Found the issue" --json
# Output: {"comment": {...}}
```

**Example with jq:**

```bash
kan show fix-login --json | jq .card.title
kan list --json | jq '.cards | length'
kan list --json | jq '.cards[] | select(.column == "in-progress") | .title'
```

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

### column

Manage columns within a board.

**Add a column:**

```bash
kan column add review
kan column add review --color "#9333ea" --position 2
```

| Flag             | Description                               |
|------------------|-------------------------------------------|
| `-b, --board`    | Target board                              |
| `-C, --color`    | Hex color (default: auto from palette)    |
| `-p, --position` | Insert position (0-indexed, default: end) |

**Delete a column:**

```bash
kan column delete review
kan column delete review --force
```

| Flag          | Description                                                              |
|---------------|--------------------------------------------------------------------------|
| `-b, --board` | Target board                                                             |
| `-f, --force` | Skip confirmation (required in non-interactive mode if column has cards) |

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
```

| Flag          | Description   |
|---------------|---------------|
| `-b, --board` | Target board  |
| `-C, --color` | New hex color |

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
| `-f, --field`  | Custom field in key=value format (repeatable) |

**Examples:**

```bash
kan add "Task title" -c backlog
kan add "Feature" -b features -c todo -f priority=high
```

### show

Display card details.

```bash
kan show fix-login-bug
```

| Flag          | Description |
|---------------|-------------|
| `-b, --board` | Board name  |

### list

List cards, grouped by column.

```bash
kan list
kan list -b features
kan list -c done
```

| Flag           | Description      |
|----------------|------------------|
| `-b, --board`  | Filter by board  |
| `-c, --column` | Filter by column |

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
| `-a, --alias`       | Set explicit alias                                |
| `-f, --field`       | Set custom field in key=value format (repeatable) |

### serve

Start the web interface.

```bash
kan serve
kan serve -p 8080
kan serve --no-open
```

| Flag         | Description                       |
|--------------|-----------------------------------|
| `-p, --port` | Port to listen on (default: 5260) |
| `--no-open`  | Don't open browser automatically  |

### comment

Manage card comments.

**Add a comment:**

```bash
kan comment add fix-login-bug "Found the issue in session.go"
kan comment add fix-login-bug  # Opens editor for body
```

| Flag          | Description |
|---------------|-------------|
| `-b, --board` | Board name  |

The first argument is the card ID or alias. The second argument is the comment body - if omitted, your editor opens to
write the comment.

**Edit a comment:**

```bash
kan comment edit c_9kL2x "Updated comment text"
kan comment edit c_9kL2x  # Opens editor with existing text
```

| Flag          | Description |
|---------------|-------------|
| `-b, --board` | Board name  |

**Delete a comment:**

```bash
kan comment delete c_9kL2x
```

| Flag          | Description |
|---------------|-------------|
| `-b, --board` | Board name  |

### migrate

Migrate board data to current schema version.

```bash
kan migrate
kan migrate --dry-run
```

| Flag        | Description                                        |
|-------------|----------------------------------------------------|
| `--dry-run` | Show what would be changed without modifying files |

## Global Flags

| Flag                    | Description                                                                              |
|-------------------------|------------------------------------------------------------------------------------------|
| `-I, --non-interactive` | Fail instead of prompting for missing input                                              |
| `--json`                | Output results as JSON (supported by: show, list, add, edit, board list, column list, comment add) |

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

# CLI Reference

The `kan` command line tool lets you manage your kanban boards from the terminal.

## Getting Started

Initialize Kan in your project directory:

```bash
kan init
```

This creates a `.kan/` directory to store your board data. You can specify a custom location:

```bash
kan init -l .kanboard
```

## Commands

### init

Initialize Kan in the current directory.

| Flag | Description |
|------|-------------|
| `-l, --location` | Custom location for .kan directory (relative path) |

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

| Flag | Description |
|------|-------------|
| `-b, --board` | Target board |
| `-C, --color` | Hex color (default: auto from palette) |
| `-p, --position` | Insert position (0-indexed, default: end) |

**Delete a column:**

```bash
kan column delete review
kan column delete review --force
```

| Flag | Description |
|------|-------------|
| `-b, --board` | Target board |
| `-f, --force` | Skip confirmation (required in non-interactive mode if column has cards) |

**Rename a column:**

```bash
kan column rename review code-review
```

| Flag | Description |
|------|-------------|
| `-b, --board` | Target board |

**Edit column properties:**

```bash
kan column edit review --color "#ec4899"
```

| Flag | Description |
|------|-------------|
| `-b, --board` | Target board |
| `-C, --color` | New hex color |

**List columns:**

```bash
kan column list
kan column list -b features
```

| Flag | Description |
|------|-------------|
| `-b, --board` | Target board |

**Move/reorder a column:**

```bash
kan column move review --position 1
kan column move review --after backlog
```

| Flag | Description |
|------|-------------|
| `-b, --board` | Target board |
| `-p, --position` | Target index (0-indexed) |
| `-a, --after` | Insert after this column |

### add

Add a new card.

```bash
kan add "Fix login bug"
kan add "Update docs" "Description goes here"
```

| Flag | Description |
|------|-------------|
| `-b, --board` | Target board |
| `-c, --column` | Target column |
| `-p, --parent` | Parent card ID or alias |
| `-f, --field` | Custom field in key=value format (repeatable) |

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

| Flag | Description |
|------|-------------|
| `-b, --board` | Board name |

### list

List cards, grouped by column.

```bash
kan list
kan list -b features
kan list -c done
```

| Flag | Description |
|------|-------------|
| `-b, --board` | Filter by board |
| `-c, --column` | Filter by column |

### edit

Edit an existing card. Run without flags for interactive mode, or use flags to apply changes directly.

```bash
kan edit fix-login-bug
kan edit fix-login-bug -t "New title" -c done
```

| Flag | Description |
|------|-------------|
| `-b, --board` | Board name |
| `-t, --title` | Set card title |
| `-d, --description` | Set card description |
| `-c, --column` | Move card to column |
| `-p, --parent` | Set parent card ID or alias |
| `-a, --alias` | Set explicit alias |
| `-f, --field` | Set custom field in key=value format (repeatable) |

### serve

Start the web interface.

```bash
kan serve
kan serve -p 8080
kan serve --no-open
```

| Flag | Description |
|------|-------------|
| `-p, --port` | Port to listen on (default: 3000) |
| `--no-open` | Don't open browser automatically |

### migrate

Migrate board data to current schema version.

```bash
kan migrate
kan migrate --dry-run
```

| Flag | Description |
|------|-------------|
| `--dry-run` | Show what would be changed without modifying files |

## Global Flags

| Flag | Description |
|------|-------------|
| `-I, --non-interactive` | Fail instead of prompting for missing input |

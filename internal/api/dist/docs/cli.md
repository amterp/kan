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

## Board and Card References

Commands name a board with `-b` and a card with their card argument.

- A **board ref** is a board name in the current project, or a board ID such as
  `b_2hH3HWg0E`. `kan board describe` and `kan registry list` show board IDs.
- A **card ref** is a card ID or alias, optionally prefixed with a board ref and a
  colon: `b_2hH3HWg0E:a_2n5h6dlsT`.

Kan finds the card a ref names like this:

1. `board:card` looks only on that board: an exact card ID, then an alias, then a
   fuzzy match. It can't be combined with `-b` or `-g`.
2. With `-b` or `-g`, kan looks only on that board.
3. A bare ref is first matched as an exact card ID on the current project's
   boards. If none matches, kan looks for a card with that ID in the other
   projects listed by `kan registry list`. If no ID matches anywhere, kan tries
   an alias, then a fuzzy match, in the current project only, because aliases
   are unique only within a board.

A board ID that isn't in the current project is looked up among the visible
registered projects. The current project's own boards always win. Hidden
entries never count. When two visible projects hold the same board ID, as two
clones of one repo do, the ref is ambiguous; hide or remove one with
[registry](#registry).

Reads work from any directory, including one with no board:

```bash
kan show b_2hH3HWg0E:a_2n5h6dlsT    # a card on another project's board
kan show a_2n5h6dlsT                # found by ID across registered projects
kan list -b b_2hH3HWg0E             # another project's board
```

Kan reads the other project without changing any file in it, even when its data
uses an older schema. Text output names the board and its path on stderr, and
`show --json` and `history --json` add a `"project"` path beside `"card"`.

Commands that change a card or board work only in the current project, or on the
global board through `-g`. Otherwise kan refuses before touching the other
project:

```
card a_2n5h6dlsT is on board b_2hH3HWg0E in /Users/alex/src/api; run kan from there to change it.
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

**Delete a board:**

```bash
kan board delete features
```

Deleting the last remaining board is not allowed. If the deleted board was set as the default, the default is cleared automatically.

**Describe a board:**

Show full board documentation including columns with their categories, `[ready]` settings, custom fields, card display settings, link rules, and pattern hooks.

```bash
kan board describe
kan board describe features
kan board describe --json
```

| Flag       | Description            |
|------------|------------------------|
| `-b, --board` | Target board (name or ID) |
| `--json`   | Machine-readable output |

### column

Manage columns within a board.

**Add a column:**

```bash
kan column add review
kan column add review --color "#9333ea" --position 2
kan column add review --description "Cards under code review"
kan column add review --limit 5
kan column add review --on-move-default-position bottom
kan column add review --category review
```

| Flag                | Description                                       |
|---------------------|---------------------------------------------------|
| `-b, --board`       | Target board (name or ID)                         |
| `--category`        | Column category: `inbox`, `idea`, `open`, `active`, `review`, `done` or `dropped` |
| `-C, --color`       | Hex color (default: auto from palette)            |
| `-d, --description` | Description of the column's purpose               |
| `-l, --limit`       | Max cards allowed in column (0 = no limit)        |
| `--on-move-default-position` | Where a card moved into this column lands: `top` (default) or `bottom` |
| `-p, --position`    | Insert position (0-indexed, default: end)         |

**Delete a column:**

```bash
kan column delete review
```

| Flag          | Description                                                              |
|---------------|--------------------------------------------------------------------------|
| `-b, --board` | Target board (name or ID)                                                |

**Rename a column:**

```bash
kan column rename review code-review
```

| Flag          | Description  |
|---------------|--------------|
| `-b, --board` | Target board (name or ID) |

**Edit column properties:**

```bash
kan column edit review --color "#ec4899"
kan column edit review --description "Updated purpose"
kan column edit review --limit 3
kan column edit review --limit 0    # Clear limit
kan column edit review --on-move-default-position bottom
kan column edit review --category review
kan column edit review --category ""   # Clear category
```

| Flag                | Description                                       |
|---------------------|---------------------------------------------------|
| `-b, --board`       | Target board (name or ID)                         |
| `--category`        | Column category, or `""` to clear it              |
| `-C, --color`       | New hex color                                     |
| `-d, --description` | New description for the column                    |
| `-l, --limit`       | Column limit (0 = clear, >0 = set max cards)      |
| `--on-move-default-position` | Where a card moved into this column lands: `top` (default) or `bottom` |

**List columns:**

```bash
kan column list
kan column list -b features
```

Each column shows its card count and, when set, its category. A board with `[ready]` settings
lists them after the columns.

| Flag          | Description  |
|---------------|--------------|
| `-b, --board` | Target board (name or ID) |

**Categorize columns:**

Suggest a category for each column that has none, from its name, using the "Column names it
usually fits" list in [Configuration](/docs/configuration#columns). Names match ignoring case
but otherwise exactly; columns that already have a category are left alone. Nothing is written without `--apply`.

```bash
kan column categorize           # Print suggestions
kan column categorize --apply   # Write them
kan column categorize --json    # {"board": "main", "applied": false, "suggestions": [{"column": "backlog", "category": "open"}, ...]}
```

| Flag          | Description                                         |
|---------------|-----------------------------------------------------|
| `-b, --board` | Target board (name or ID)                           |
| `--apply`     | Write the suggested categories instead of printing them |

**Move/reorder a column:**

```bash
kan column move review --position 1
kan column move review --after backlog
```

| Flag             | Description              |
|------------------|--------------------------|
| `-b, --board`    | Target board (name or ID) |
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
| `-b, --board`  | Target board (name or ID)                     |
| `-c, --column` | Target column                                 |
| `-p, --parent` | Parent card ID or alias                       |
| `--blocked-by` | Blocking card IDs, aliases or `board:card` refs (repeatable or variadic) |
| `--position`   | Insert at index (0 = top, -1 = end, negatives count from end) |
| `--top`        | Insert at the top of the column                |
| `--bottom`     | Insert at the bottom of the column             |
| `--before`     | Insert before this card (ID or alias)         |
| `--after`      | Insert after this card (ID or alias)          |
| `-f, --field`  | Custom field in key=value format (repeatable; set-typed fields also accept comma-separated values) |
| `--strict`     | Error if wanted fields are missing (default: warn) |
| `-g, --global` | Target the designated global board (see [global](#global)) |

`--parent` accepts a card ID or alias on the same board and stores the resolved
ID. Renaming the parent does not break the relationship. Self-parents and parent
cycles are rejected.

The five placement flags are mutually exclusive. By default a card is appended to
the end of its column, whatever that column's `on_move_default_position` says -
that setting governs cards *moving into* a column, not new ones. Without `-c`, the
card is placed in the anchor card's column.

`--position` is most useful for the boundaries (`0` = top, `-1` = end). For a spot
in the middle, prefer `--before`/`--after` with a card you can see in `kan list`,
since `kan list` does not print numeric indices to count against.

**Examples:**

```bash
kan add "Task title" -c backlog
kan add "Feature" -b features -c todo -f priority=high
kan add "Urgent" -c backlog --top           # top of backlog
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
`comment add`. A card on another board or project can be named as `board:card`;
see [Board and Card References](#board-and-card-references).

`show` also reports how long the card has been in its current column
(e.g. `Column: review (3 days)`).

| Flag           | Description                                                |
|----------------|------------------------------------------------------------|
| `-b, --board`  | Board name or ID                                           |
| `-g, --global` | Target the designated global board (see [global](#global)) |

### link

Add blockers or related cards. References accept card IDs, aliases, or an
unambiguous partial match on the same board, and are stored as card IDs. To link
a card on another board, prefix it with the board's name or ID:
`features:login-form`, or `b_2hH3HWg0E:a_2n5h6dlsT` for a board in another
registered project. Kan stores such a link as `boardID:cardID`, so it keeps
working whichever clone of that repo is on the machine.

```bash
kan link deploy --blocked-by tests review
kan link deploy --related rollout
kan link deploy --blocked-by b_2hH3HWg0E:api-endpoint   # a card in another repo
kan add "Deploy" --blocked-by tests
```

`--blocked-by` and `--related` accept multiple values and can be repeated. At
least one is required. Blocker cycles and blockers that name a card's ancestor
are rejected before writing. A card cannot name itself as related. Related links
do not block work. Adding an existing
link is a no-op. Cycle checks follow links into other boards and projects as far
as kan can read them.

Linking to a board in another project also records that board in the current
project's config, under `[linked_boards]`: its project and board name, and the
repo's `origin` URL with any credentials removed (a local path, or a URL kan
can't parse, isn't recorded). Kan never uses this to find the board. When the
board isn't on this machine, `show` uses it to say what is missing and where to
get it:

```
a_2nQ0vLx1k waits on api/main (b_2hH3HWg0E), which isn't on this machine.
One place to get it: git@github.com:example/api.git
```

### unlink

Remove references from both the blocker and related lists.

```bash
kan unlink deploy tests rollout
kan unlink deploy deleted-id
```

Stored strings match first, so a dangling reference can be removed even when its
target cannot be found. Otherwise references resolve as card IDs or aliases, and
`board:card` references as they do for `link`.
Unmatched references are reported on stderr. If no links match, the command
exits 1 and leaves the card unchanged. If some match, those links are removed
and the command succeeds while naming the unmatched references.

Both commands accept `-b/--board`, `-g/--global`, and `--json`. `show` lists
blockers with their columns and categories, cards that wait on this one, and
related cards. Cards that wait on this one are gathered from every visible
registered project, so closing a card shows what waits on it in other repos too;
a project that can't be read is skipped with a note on stderr. A card on another
board is shown as `boardID:cardID`, with its project's path when it's in another
repo. Unresolved references show their lookup state and reason. A missing
card is reported as absent only after its board was read completely. JSON keeps
`blocked_by` and `related` inside the existing card object.

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
| `-b, --board`  | Board name or ID                                           |
| `-g, --global` | Target the designated global board (see [global](#global)) |

### list

List cards, grouped by column.

```bash
kan list
kan list -b features
kan list -c done
kan list --sort priority            # order each column by the priority field
kan list --sort priority --descending  # high → low instead of low → high
kan list -f labels=needs-decision   # cards labeled needs-decision
kan list -f type=bug -x labels=wontfix  # bugs, except those labeled wontfix
kan list --category open            # cards in open columns
kan list --all -f labels=needs-decision  # across every registered project
```

| Flag               | Description                                                |
|--------------------|------------------------------------------------------------|
| `-b, --board`      | Board name or ID                                           |
| `-c, --column`     | Filter by column                                           |
| `-f, --filter`     | Keep cards whose custom field matches `key=value` (repeatable) |
| `-x, --exclude`    | Drop cards whose custom field matches `key=value` (repeatable) |
| `--category`       | Keep cards in columns with this category (`inbox`, `idea`, `open`, `active`, `review`, `done`, `dropped`) |
| `-s, --sort`       | Sort cards within each column by a custom field (e.g. `priority`) instead of by manual position |
| `-d, --descending` | Sort descending (use with `--sort`)                        |
| `-g, --global`     | Target the designated global board (see [global](#global)) |
| `--all`            | List every board of every visible registered project (see [Across projects](#across-projects)) |

`--sort` orders cards by a custom field's value. For `enum`/`enum-set` fields
the order follows the option order defined in your board config (so
`priority` sorts `low → medium → high` when that's how the options are listed),
not alphabetically. Cards missing a value for the field are always listed last,
in both directions. Sorting is non-destructive — it only changes the listing
order, never the cards' saved positions.

`-f` and `-x` match custom fields, following each field's type in the board
config: a set field (`enum-set`, `free-set`) matches when it contains the
value, a `boolean` compares as `true` or `false`, and anything else compares
as an exact, case-sensitive string. Every filter must hold, so two `-f` on one
field keep only cards matching both. A card without the field never matches:
`-f` drops it and `-x` keeps it. Filtering by a field the board doesn't define
is an error that lists the board's fields; built-in fields such as the column
have their own flags. `--sort` and `--descending` still apply after filtering.

### ready

List open cards that can be started now, in priority order.

```bash
kan ready
kan ready --limit 3                 # the first three ready cards
kan ready -x labels=needs-decision  # also keep these out
kan ready --json
kan ready --all                     # every registered project's boards
```

A card is ready when all of these hold:

1. Its column's category is `open`.
2. Every blocker is found, in a column whose category is `done`.
3. It has no unfinished children, meaning children in any category other than
   `done` or `dropped`. A child in a column without a category counts as
   unfinished.
4. It matches none of the board's `[ready] exclude` filters or the command's
   `-x` filters.

A blocker in a column without a category, or one kan can't find or read, keeps
the card out of ready; `kan show` says why. A card whose blocker was dropped is
listed under "Needs attention" instead, even when an exclusion matches it,
because the reason for the card may be gone: remove the link with `kan unlink`
or drop the card.

Open columns are read from right to left, each top to bottom, so the open
column furthest right comes first. `[ready] order` in the board config names
columns to read first (see [Configuration](/docs/configuration)).

```
$ kan ready
next
  a_2nQ0vLx1k  Add kan ready
  a_2nQ0uA7cd  Add blocked_by and related to cards
backlog-hi
  a_2nQ0w9mPe  Show blockers in the card modal

Needs attention
  a_2nQ0zz0aa  Rework the board picker  blocker dropped: a_2nQ0yy9bb (wont-do)
```

| Flag             | Description                                                |
|------------------|------------------------------------------------------------|
| `-b, --board`    | Board name or ID                                           |
| `-x, --exclude`  | Also keep cards whose custom field matches `key=value` out of ready (repeatable) |
| `--no-exclude`   | Ignore the board's `[ready] exclude` list; `-x` still applies |
| `--limit`        | Print only the first N ready cards; "Needs attention" is never cut |
| `-g, --global`   | Target the designated global board (see [global](#global)) |
| `--all`          | Every board of every visible registered project; `--limit` applies per board (see [Across projects](#across-projects)) |

`-x` matches the way `kan list -x` does. On a board with no column categories,
`kan ready` fails and points to `kan column categorize`.

With `--json`, each entry holds the query's context beside the card's own JSON,
so a custom field can never collide with a context field:

```json
{
  "ready": [
    {
      "ref": "b_2nPzK4wQe:a_2nQ0vLx1k",
      "board_id": "b_2nPzK4wQe",
      "board": "main",
      "project_path": "/Users/alex/src/kan",
      "category": "open",
      "blockers": [{"ref": "a_2nQ0uA7cd", "result": "found", "category": "done"}],
      "card": {"id": "a_2nQ0vLx1k", "title": "Add kan ready", "...": "..."}
    }
  ],
  "attention": [],
  "skipped": []
}
```

A blocker's `result` is one of `found`, `absent`, `board_not_here`,
`ambiguous_board`, `unsupported_schema` or `unreadable`, and `category` is
present when it was found in a column that has one. `skipped` lists boards an
`--all` query couldn't read (see below).

#### Across projects

`kan list --all` and `kan ready --all` read every board of every visible
project in the registry (see [registry](#registry)), plus the project you are
in. They work from any directory, including one with no board, and can't be
combined with `-b` or `-g`.

- Results are grouped by board, in order of project path, then board name.
  Boards aren't ranked against each other.
- `--limit` applies per board.
- Filters apply on each board. A card on a board that doesn't define a field
  never matches it, and a field no board defines is an error. `-c`, `--sort`
  and `--category` likewise apply where the board has that column, field or
  category.
- Boards kan can't read, whose schema is newer than this kan, or, for
  `kan ready`, that have no column categories, are skipped. Text output names
  each one and the reason in a note on stderr; JSON lists them under
  `"skipped"` with `path`, `board` (its name), `board_id` and `reason`.
  `board_id` is empty when the board's config can't be read, and both are
  empty when kan can't tell which boards a project has. Skipping a board
  doesn't change the exit status. A project kan can't open at all, such as one
  with a malformed card file, is skipped with all its boards.

`kan list --all --json` uses the same envelope as `kan ready`, with each card's
raw `blocked_by` and `related` inside it, so an outside tool can draw the
dependency graph. `category` is left out when the card's column has none:

```json
{
  "cards": [
    {
      "ref": "b_2nPzK4wQe:a_2nQ0vLx1k",
      "board_id": "b_2nPzK4wQe",
      "board": "main",
      "project_path": "/Users/alex/src/kan",
      "category": "open",
      "card": {"id": "a_2nQ0vLx1k", "blocked_by": ["a_2nQ0uA7cd"], "...": "..."}
    }
  ],
  "skipped": [{"path": "/Users/alex/src/old", "board": "main", "board_id": "b_2hH3HWg0E", "reason": "..."}]
}
```

`kan list --json` without `--all` keeps its `{"cards": [...]}` shape.

### edit

Edit an existing card. Run without flags for interactive mode, or use flags to apply changes directly.

```bash
kan edit fix-login-bug
kan edit fix-login-bug -t "New title" -c done
```

| Flag                | Description                                       |
|---------------------|---------------------------------------------------|
| `-b, --board`       | Board name or ID                                  |
| `-t, --title`       | Set card title                                    |
| `-d, --description` | Set card description                              |
| `-c, --column`      | Move card to column                               |
| `-p, --parent`      | Set parent card ID or alias                       |
| `--position`        | Move to index in column (0 = top, -1 = end, negatives count from end) |
| `--top`             | Move to the top of the column                     |
| `--bottom`          | Move to the bottom of the column                  |
| `--before`          | Move before this card (ID or alias)               |
| `--after`           | Move after this card (ID or alias)                |
| `-a, --alias`       | Set explicit alias                                |
| `-f, --field`       | Set custom field in key=value format (repeatable; set-typed fields also accept comma-separated values) |
| `--strict`          | Error if wanted fields are missing (default: warn) |
| `-g, --global`      | Target the designated global board (see [global](#global)) |

`--parent` accepts a card ID or alias on the same board and stores the resolved
ID. A card cannot be its own parent, and a parent change cannot form a cycle.

The five placement flags are mutually exclusive. They reorder a card within its
current column, or place it precisely when moving between columns. Without `-c`,
the card is placed in the anchor card's column. As with `add`, prefer
`--before`/`--after` over `--position` for non-boundary spots.

With `-c` and no placement flag, the destination column's
[`on_move_default_position`](/docs/configuration#columns) decides where the card lands.

To move a card, [`move`](#move) is the shorter spelling. Use `edit -c` when you
are changing other fields in the same command, so the move and the edit share
one history entry.

**Examples:**

```bash
kan edit fix-login -c doing --top          # top of doing
kan edit fix-login --before deploy         # reorder relative to deploy
kan edit fix-login --bottom                # bottom of its column
kan edit fix-login -c done -t "Shipped"    # move and retitle together
```

### move

Move a card to a different column.

```bash
kan move fix-login-bug done
```

| Flag           | Description                                                |
|----------------|------------------------------------------------------------|
| `-b, --board`  | Board name or ID                                           |
| `--position`   | Move to index in column (0 = top, -1 = end, negatives count from end) |
| `--top`        | Move to the top of the column                              |
| `--bottom`     | Move to the bottom of the column                           |
| `--before`     | Move before this card (ID or alias)                        |
| `--after`      | Move after this card (ID or alias)                         |
| `-g, --global` | Target the designated global board (see [global](#global)) |

The column is optional when you give `--before` or `--after`: the card follows
the anchor into whatever column the anchor is in.

With no placement flag, the destination column's
[`on_move_default_position`](/docs/configuration#columns) decides where the card lands -
top unless the column says bottom. Moving a card to the column it already
occupies does nothing; use a placement flag to reorder it.

**Examples:**

```bash
kan move fix-login done              # wherever `done` says arrivals go
kan move fix-login next --bottom     # back of the queue
kan move fix-login --after deploy    # column inferred from deploy
```

### delete

Delete a card.

```bash
kan delete fix-login-bug
```

| Flag           | Description                                                |
|----------------|------------------------------------------------------------|
| `-b, --board`  | Board name or ID                                           |
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
| `-b, --board`  | Board name or ID                                           |
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
| `-b, --board`  | Board name or ID                                           |
| `-g, --global` | Target the designated global board (see [global](#global)) |

**Delete a comment:**

```bash
kan comment delete c_9kL2x
```

| Flag           | Description                                                |
|----------------|------------------------------------------------------------|
| `-b, --board`  | Board name or ID                                           |
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

Once set, `-g` works on `add`, `list`, `ready`, `show`, `history`, `edit`,
`move`, `delete`, `link`, `unlink`, `doctor`, and `comment add`/`edit`/`delete`:

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

### registry

List and clean up the projects kan knows on this machine (the `[repos]` table
in `~/.config/kan/config.toml`). Kan registers each project it runs in, except
ones under a temp directory.

```bash
kan registry list              # Visible projects: path, name, state, boards with IDs
kan registry list --hidden     # Include hidden entries
kan registry hide <path>       # Stop offering a project outside its own directory
kan registry unhide <path>     # Offer a hidden project again
kan registry remove <path>     # Delete the entry and its settings
kan registry prune             # Remove entries whose project is gone
kan registry prune --dry-run   # Show what prune would remove
```

A hidden project still works when you run kan inside it, but it doesn't appear
in the web UI's project switcher or omnibar. When kan registers a second copy of
a board that another visible entry already holds, it registers the copy hidden
and says so.

`list` reports each entry's state: `ok`, `missing` (the path, or its board
folder, is gone), `unreadable` (kan couldn't check, e.g. a permission error), or
`duplicate board ID` (another visible entry holds the same board). `prune`
removes only `missing` entries. Use `hide` for a project that still exists:
`remove` deletes the entry, but kan registers the project again the next time it
runs there.

### migrate

Migrate board data to current schema version.

You rarely need to run this yourself. Any command that changes data, and
`kan serve`, migrates an older project on disk before it runs. Commands that
only read (`list`, `ready`, `show`, `history`, `board list`, `board describe`,
`column list`, `column categorize` without `--apply`, `registry list`, `doctor`
without `--fix`, and shell completion) never modify files, in this project or
in any other project they read: they upgrade older data in memory and leave the files as they are.

`kan migrate` finds the project the same way other commands do: it honors a
registered custom data location, and in a linked git worktree it migrates the
main worktree's board, which every command there uses.

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
kan doctor --all
kan doctor --json
```

| Flag          | Description                                         |
|---------------|-----------------------------------------------------|
| `--fix`       | Apply automatic fixes for issues with deterministic solutions |
| `--dry-run`   | Show what fixes would be applied without making changes |
| `-b, --board` | Check only a specific board, by name or ID (default: all) |
| `-g, --global` | Check the designated global board |
| `--all`       | Check every visible registered project and the current project |

**Exit codes:**

- `0`: No errors (warnings are OK)
- `1`: Errors found

**Issues detected:**

- **Errors** (must be fixed):
  - `MALFORMED_BOARD_CONFIG`: Board config.toml fails to parse
  - `MALFORMED_CARD`: Card JSON fails to parse, or its ID disagrees with its filename
  - `ORPHANED_CARD`: Card has no column, or a column the board doesn't define (fixable)

- **Warnings** (should be addressed):
  - `SCHEMA_OUTDATED`: Board/card needs migration (run `kan migrate`)
  - `INVALID_DEFAULT_COLUMN`: References missing column (fixable)
  - `INVALID_CARD_DISPLAY`: References missing custom field (fixable)
  - `INVALID_LINK_RULE`: Regex doesn't compile
  - `INVALID_PATTERN_HOOK`: Regex doesn't compile
  - `MISSING_HOOK_FILE`: Pattern hook command not found (missing file, or not on `PATH`)
  - `HOOK_NOT_EXECUTABLE`: Pattern hook file exists but lacks the executable bit
  - `HOOK_COMMAND_ARGS`: Pattern hook command appears to take arguments (hooks take none)
  - `INVALID_PARENT_REF`: Parent ID or alias does not resolve on the same board (fixable)
  - `SELF_PARENT`: Card names itself as its parent (manual repair)
  - `PARENT_CYCLE`: Cards form a parent cycle (manual repair)
  - `ABSENT_LINK`: A `blocked_by` or `related` target is confirmed absent (fixable)
  - `UNRESOLVED_LINK`: A link cannot be resolved; the message includes its state and reason
  - `BLOCKED_BY_CYCLE`: Blockers form a cycle, including self-blocking (manual repair)
  - `BLOCKED_BY_ANCESTOR`: A blocker is the card's own ancestor (manual repair)
  - `UNCATEGORIZED_LINK`: A card with links, or a linked target, has no usable column category
  - `INVALID_READY_CONFIG`: A `[ready]` entry fails validation
  - `DUPLICATE_BOARD_ID`: Multiple visible registered projects hold the same board ID (`--all`)
  - `MISSING_WANTED_FIELDS`: Card is missing fields marked as `wanted`
  - `DUPLICATE_POSITION_KEYS`: Two cards in a column share a position key, so no card can be placed between them (fixable)
  - `LEGACY_POSITION_KEYS`: A column has cards with no position key, or with keys written by an older Kan (fixable)
  - `LONG_POSITION_KEYS`: A column has position keys longer than 12 characters (fixable)
  - `MALFORMED_GLOBAL_CONFIG`: Global config.toml fails to parse
  - `GLOBAL_SCHEMA_OUTDATED`: Global config needs migration

`--fix` removes a link only when its target is still confirmed absent on a fresh lookup immediately before writing. It edits only source cards in the current project. Missing boards, ambiguous board IDs, newer schemas, and read failures leave links intact. A skipped or damaged card file prevents the board from proving a target absent. Cycles, ancestor blockers, category problems, and invalid ready settings require manual repair.

`kan doctor --all` checks every readable board of the current project and every visible registered
project, even when run outside a project. It checks links between projects and reports duplicate
board IDs as `DUPLICATE_BOARD_ID`, naming the visible copies. Board statistics and issues include
`project_path` in JSON. Skipped projects or boards appear under `skipped` (`path`, `board_id`,
`reason`) and on stderr in text mode; skips alone do not change the exit status.

With `--all --fix`, only the current project's findings can be repaired. Other projects remain
read-only; run doctor from their directory to repair them. Outside a project, `--all --fix`
performs no project writes. `--all` cannot be combined with `-b/--board` or `-g/--global`.
`-g` checks the designated global board; `-b` accepts a local board name or ID.

Fixing the position-key warnings rewrites cards' `position` fields and keeps their order. When most of a column needs new keys, every card in it is renumbered, so on a board used with an older Kan the fix touches many card files: commit it on its own.

### docs

Print these docs from the binary. They match the version of `kan` you're running, and work offline and outside any project.

```bash
kan docs                # topic index: every page, its command, and its sections
kan docs cli            # print one page
kan docs index          # the docs landing page (not the generated index above)
kan docs all            # the index plus every page
```

| Flag     | Description                                    |
|----------|------------------------------------------------|
| `--json` | Print the topic index as JSON instead of a list |

Pages print as raw Markdown. Links to other pages are rewritten into the command that prints them, so a link to the Custom Fields page reads as `Custom Fields (see: kan docs custom-fields)`.

`--json` covers the topic index only, as `{"topics": [{slug, title, sections}]}`. Pages are Markdown, so `kan docs cli --json` warns and prints the page unchanged.

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
| `--json`                | Output results as JSON (supported by: add, edit, move, show, history, list, ready, link, unlink, board list, board describe, column list, column categorize, comment add, registry list, doctor, docs) |

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

# List cards across every registered project as JSON
kan list --all --json
# Output: {"cards": [{"ref": "...", "project_path": "...", "card": {...}}, ...], "skipped": [...]}

# List ready cards as JSON
kan ready --json
# Output: {"ready": [{"ref": "...", "blockers": [...], "card": {...}}, ...], "attention": [...], "skipped": []}

# List boards as JSON
kan board list --json
# Output: {"boards": ["main", "features"]}

# List columns as JSON
kan column list --json
# Output: {"columns": [{"name": "backlog", "color": "#...", "category": "open", "card_count": 5}, ...]}
# A board with [ready] settings also gets "ready": {"order": [...], "exclude": [...]}

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

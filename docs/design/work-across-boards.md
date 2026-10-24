# Links, ready work, and work across boards

Agreed design, 2026-10-24, not yet built. Cards on kan's own board track the work, one parent card
per phase (`a_2naKEWqdp`, `a_2naKFEs7S`, `a_2naKFgTg0`, `a_2naKGb6Hi`), and each card points at its
section here.

## Why

Agents now do most of the work on the maintainer's boards, about 40 boards in as many repos. Kan
gives them three things too few:

1. **What can be worked on now.** An agent reads the columns and guesses. Dependencies between
   cards live in prose ("after a_2n5h6dlsT lands"): descriptions and comments across those boards
   cite another card's ID about 1,700 times.
2. **Dependencies between cards**, so a card waiting on another stays out of the way until the
   other is done.
3. **One view of the work across every board**, while each board keeps its own columns and custom
   fields. Fourteen different column layouts are in use.

We considered moving to beads, an issue tracker built for coding agents, and chose to grow kan
instead. The reasons are under [Rejected alternatives](#rejected-alternatives). From beads we take
the dependency model and the ready query.

## Constraints

- Kan stays a simple, offline, fast board that lives in the repo. Everything here is opt-in: a
  board that uses none of it behaves as it does today, and nothing needs a network, a daemon or a
  server.
- Kan doesn't impose a workflow. It offers categories, links and queries. What a board's columns
  mean, and how agents use the queries, is the board owner's business.
- Kan computes derived state (ready, blocked, reverse links) when asked and never stores it. Card
  files stay the only source of truth.
- New code reads and writes through the store interfaces in `internal/store/interface.go`, never
  through `.kan/` paths, so a different storage backend can come later without touching it.
- A command builds the cards and links it needs once and answers everything from that: blockers,
  reverse links, cycle checks and output. No index is stored between commands.
- Kan is pre-v1. Breaking schema changes are fine with a migration, per `docs/COMPAT.md`. Every
  schema bump below follows the checklist in `AGENTS.md`: version constant, `MinKanVersion` entry,
  fixtures, migration and no-op tests, and a `docs/COMPAT.md` section.

## Terms

| Term | Meaning |
|---|---|
| category | One of seven fixed kinds of column (inbox, idea, open, active, review, done, dropped). A column may name one. |
| link | A card's reference to another card: `blocked_by` or `related`. |
| blocker | A card named in another card's `blocked_by`. |
| ready | Computed: an open card with no unfinished blocker. See [Ready](#ready). |
| registry | The list of projects kan knows on this machine, the `[repos]` table in `~/.config/kan/config.toml`. Entries can be hidden. |
| board ref | A board name in the current project, or a board ID. |
| card ref | A card ID or alias, optionally prefixed with a board ref and a colon: `b_2hH3HWg0E:a_2n5h6dlsT`. |
| stored ref | What a card's `blocked_by` or `related` holds: a card ID, or `boardID:cardID` for another board. Never an alias. |

## Phase 1: foundations

### Read commands never write inside a project

Kan migrates a project with an older schema before running any command, so `kan list` in a repo
whose board is at `board/12` rewrites its `config.toml`. An agent that only meant to read is left
with a dirty repo, and a query across 40 boards would dirty every repo with an older schema. Reads
also write in other ways: a missing `.kan/config.toml` is created, and an existing one is rewritten
whenever its schema differs from the current one (`FileProjectStore.EnsureInitialized`).

The rule: a command that only reads writes nothing under any project's root. It may still write the
global config, so registering a project and migrating the global config stay as they are.

- Read commands are `list`, `show`, `history`, `ready`, `board list`, `board describe`,
  `column list`, `registry list`, `doctor` without `--fix`, and shell completion.
- They read older board, card and project schemas by transforming them in memory, using the same
  transformation functions `kan migrate` applies before writing. There is one implementation of
  each migration step, with two callers.
- A missing project config is filled with defaults in memory and not written.
- `doctor` keeps reading raw files, so it can still report schema defects.
- Write commands keep migrating the whole project before they run, as today.
- `kan serve` is an editing session. It migrates the project it opens, and a project the web UI
  switches to, as today.

Project config joins the migration machinery, which today covers only global config, boards and
cards (`MigrateService.MigrationPlan`). It gets future-version protection like the others: kan
refuses to load, and never saves, a project config whose schema is newer than its own. Without
this, an older kan would rewrite a `project/3` config as `project/2` and drop fields it doesn't
know, such as `[linked_boards]` from phase 4. This protection must ship before any project schema
bump.

A board, card or project with a schema newer than the running kan still fails on its own. Under
`--all` it is skipped and reported (see [Queries across boards](#queries-across-boards)).

`kan migrate` builds its own discovery today: it passes an empty global config and skips the
worktree redirect, so it ignores a custom `data_location` and, run in a linked worktree, migrates
the worktree's copy instead of the shared board. It switches to the discovery every other command
uses.

This fixes the tooling-feedback card "a read-only 'kan list' migrates an older board".

### The project registry

`~/.config/kan/config.toml` keeps two lists of projects. `[projects]` maps a name, which is the
folder's basename, to a path. `[repos]` maps a path to that project's settings. Registering a
project writes an entry to both. Two folders with the same basename share one `[projects]` slot, so
registering the second silently drops the first from the web UI's project switcher, the main
reader of `[projects]`. On the maintainer's machine a scratch folder replaced a real project with
the same name this way.

We drop `[projects]` (global schema `global/3`, with a migration) and keep `[repos]` as the only
registry. The switcher names each project from the `name` field in its `.kan/config.toml`, and
shows the path as well when two names match. `kan migrate`, the other reader of `[projects]`,
switches to `[repos]`.

Before:

```toml
kan_schema = "global/2"

[projects]
  kan = "/Users/alex/src/kan"
  api = "/private/tmp/scratchpad/api"

[repos]
  [repos."/Users/alex/src/kan"]
  [repos."/Users/alex/src/api"]
  [repos."/private/tmp/scratchpad/api"]
  [repos."/Users/alex/notes"]
    data_location = "kan-data"
```

After the migration:

```toml
kan_schema = "global/3"

[repos]
  [repos."/Users/alex/src/kan"]
  [repos."/Users/alex/src/api"]
  [repos."/private/tmp/scratchpad/api"]
    hidden = true
  [repos."/Users/alex/notes"]
    data_location = "kan-data"
```

`hidden` is the next change. Kan registers every project it runs in, so test projects, scratch
folders and second clones pile up: 73 of the 124 entries on the maintainer's machine point into
temp directories, and every one of them shows up in the web UI's project switcher and omnibar.

#### Hidden entries

A registry entry does two jobs. It lets discovery find a board at a custom `data_location`, and it
decides which projects kan offers beyond the one you're in: in the switcher and omnibar, in `--all`
queries, and when resolving a board ID or a bare card ID from another project. One-off copies need
the first job and not the second, so an entry can be hidden.

A hidden project works normally when you run kan inside it, and its `data_location` still applies.
It doesn't appear in the switcher or omnibar, `--all` skips it, and kan doesn't use it to resolve
references from other projects.

Kan keeps one-off copies out of the way itself:

- **Temp directories.** When a command discovers an unregistered project (`registerProject` in
  `internal/cli/app.go`) whose path, with symlinks resolved, is inside `os.TempDir()`, `/tmp` or
  `/private/tmp`, kan doesn't register it. The test compares whole path components, so `/tmpfoo`
  is not inside `/tmp`. `kan init` there registers a hidden entry only when given a custom
  `--location`, which discovery can't find without one, and otherwise registers nothing, since
  discovery finds `.kan/` by walking up from the working directory.
- **Second copies of a board.** When kan registers a project whose board ID is already held by a
  visible entry at another path, it registers the new one hidden and says so:
  `board b_2hH3HWg0E is already registered at /Users/alex/src/api; registered /Users/alex/src/api2
  as hidden. Run "kan registry unhide /Users/alex/src/api2" to use this copy instead.`
- **The `global/3` migration** hides existing entries under temp directories. It leaves other
  duplicates visible; `kan doctor` and `kan registry list` report them.

Commands:

- **`kan registry list`** prints each visible path, its project name, its boards with their IDs,
  and its state: ok, missing (the path or its board folder is gone), unreadable, or duplicate board
  ID. `--hidden` includes hidden entries, marked as such.
- **`kan registry hide <path>`** and **`kan registry unhide <path>`** set and clear `hidden`.
  Unhiding a path whose board ID another visible entry holds warns that the two are now ambiguous.
- **`kan registry remove <path>`** deletes the entry, including its settings such as
  `data_location`. A project that still exists registers itself again the next time kan runs
  there, so use `hide` for a live project and `remove` for one that's gone.
- **`kan registry prune`** removes entries whose state is missing: the path doesn't exist, or it
  exists and holds no board folder at its data location. A permission error or any other failure to
  read is not missing, and prune leaves that entry. `--dry-run` prints what it would remove.

#### Duplicate board IDs

Two clones of one repo carry the same board IDs, and their cards can differ. Kan never picks one
copy over another:

- A command always uses the current project's own boards for the current project, whatever else in
  the registry shares their IDs.
- Hidden entries never count.
- When two visible entries hold a board with the same ID, references to that board resolve as
  ambiguous. A link to a card on it counts as blocked, and `kan doctor` and `kan registry list`
  name both paths. Hiding or removing one resolves it.

Linked worktrees don't cause duplicates, because kan sends a worktree to the main checkout's board
(`ResolveWorktree` in `internal/discovery/discovery.go`) and registers the main checkout.

### Card creation never overwrites

`FileCardStore.Create` writes the card file without checking whether one already exists. A card ID
is the number of 10 ms ticks since 2026-01-01 plus 3 random base62 characters
(`internal/id/generator.go`), so two IDs collide only when made in the same 10 ms with the same
draw: 1 in 238,328 for a pair made in one tick. That is rare (no ID repeats across the 3,924 cards
on the maintainer's boards), but a collision silently replaces an existing card.

`Create` opens the file exclusively and fails if it exists, and the card service retries with a
fresh ID, up to five times. This guarantees unique IDs within a board only. Across boards, a
duplicate is handled by reference resolution, which reports it as ambiguous (see
[References](#references)). We keep 3 random characters: a fourth would make collisions 62 times
rarer at the cost of a longer ID for everyone, and the rate doesn't call for it.

## Phase 2: column categories

Each column may name one category from a fixed set that kan owns. Anything that reads across
columns or boards (`kan ready`, `--all`) reads categories, never column names, so boards keep
naming their columns as they like.

| Category | Meaning | Column names it usually fits |
|---|---|---|
| inbox | Waiting for someone to decide what it is: new and untriaged, or a question that could be answered now. | inbox |
| idea | Looked at and kept, but the direction is uncertain. | uncommitted, undecided, ideas |
| open | Accepted and not started. Order within and across open columns is priority. | backlog, backlog-lo, backlog-hi, todo, next, today |
| active | Being worked on. | in-progress, doing |
| review | Finished by whoever worked on it, waiting to be checked. | review |
| done | Finished. | done |
| dropped | Closed without being done. | wont-do |

The set has one `open` category rather than separate backlog and next categories. Card order
already ranks open work: the board's column order, then each card's position within its column.
What a `next` column adds over a backlog is commitment, and a board that wants that keeps its
`next` column. Kan doesn't need to know which open column is which.

### Schema and defaults

Board schema `board/14` adds both phase 2's column categories and phase 3's `[ready]` settings
(see [Order](#order) and [Exclusions](#exclusions)), so that phase 3 needs no second board bump.
Otherwise a phase-2 kan, accepting the same schema, would drop `[ready]` the next time it saved
the board's config.

```toml
[[columns]]
  name = "next"
  color = "#3b82f6"
  category = "open"

[ready]
  order = ["next", "backlog-hi", "backlog-lo"]
  exclude = ["labels=needs-decision"]
```

Both are optional. A column without a category is invisible to `kan ready`. A board with no
categorized columns, such as a board whose columns are project lifecycle states, stays out of
`kan ready` entirely. The migration to `board/14` leaves existing columns uncategorized.

`kan init` and `kan board create` set categories on the default columns: `backlog` and `next` are
open, `in-progress` is active, `done` is done.

Validation, as warnings like the other board-config checks: an unknown category, a `[ready] order`
entry that names no column, names a column twice, or names a column that isn't open, and an
`exclude` entry that doesn't parse as a filter.

### CLI, API and web

- `kan column add --category <c>` and `kan column edit --category <c>`. An empty value clears it.
- `kan column list` and `kan board describe` show each column's category and the `[ready]`
  settings.
- `kan column categorize` suggests a category for each uncategorized column from its name, using
  the last column of the table above, and prints the suggestions. It writes them only with
  `--apply`. Kan never infers a category from a column name anywhere else.
- `BoardService` keeps `[ready] order` in step when a column is renamed or deleted, so the CLI and
  the HTTP API get the same behavior.
- The column endpoints, their Go request and response types and the TypeScript types carry
  `category`. The web UI's column menu shows and sets it.

## Phase 3: links and ready, within a board

### Links

Cards get two new fields (card schema `card/4`), both lists of stored refs:

- `blocked_by`: this card can't start until each listed card is done.
- `related`: an annotation with no effect on ready.

A link is stored only on the card that waits or refers. The reverse direction ("blocks", "related
to") is computed when shown. A link to a card on the same board stores the bare card ID. A link to
another board stores `boardID:cardID` (phase 4). Commands accept aliases but store IDs, because
aliases change when titles do.

```json
{
  "_v": 4,
  "id": "a_2nQ0vLx1k",
  "title": "Add kan ready",
  "column": "backlog-hi",
  "blocked_by": ["a_2nQ0uA7cd", "b_2hH3HWg0E:a_2n5h6dlsT"],
  "related": ["a_2nQ0w9mPe"]
}
```

#### Collisions with existing custom fields

Custom fields live at the top level of a card's JSON, so a board that already has a custom field
named `blocked_by` or `related` would collide with the new fields. That includes values left in
card files after their field's definition was removed from the board config.

The `card/4` migration checks raw board configs and raw card keys for both names before it changes
anything. On a collision it renames the custom field to `x_blocked_by` or `x_related`, the escape
hatch in `docs/COMPAT.md`: in the board's `[custom_fields]`, in `[card_display]`, in
`[ready] exclude`, and in every card that holds a value. It prints each rename. This runs on raw
TOML and JSON, before kan decodes cards into its types, and the in-memory path for read commands
applies the same rename. No board on the maintainer's machine has either name.

#### Parents

`parent` doesn't block anything, but the ready rule below reads it, so it has to be reliable.
Today `kan add --parent` and `kan edit --parent` check that the argument resolves, then store the
argument as typed, so a parent can be an alias that later stops matching. Kan also accepts a card
as its own parent, and parent cycles.

- Writes store the parent's card ID, from the CLI and the HTTP API alike.
- The `card/4` migration rewrites each parent that resolves as an alias to that card's ID. A parent
  that resolves to nothing is left as it is, and `kan doctor` reports it.
- Kan refuses a card as its own parent and a parent that would form a cycle. `kan doctor` reports
  existing ones.
- Parents stay within one board in this design. `docs/COMPAT.md` claims a parent survives a move
  to another board, which the code doesn't do; that section is corrected.

On the maintainer's boards, 2,031 of 2,037 parent values are already IDs, and the other 6 are test
fixtures.

#### Commands

- `kan link <card> --blocked-by <ref>... --related <ref>...` adds links.
- `kan unlink <card> <ref>...` removes each ref from whichever list holds it. It matches the
  stored string first, then the resolved ID, so it works even when the target can't be found.
- `kan add --blocked-by <ref>` files a card with its blockers in one step, through the same link
  service as `kan link`.
- `kan show` lists the card's blockers with their column and state (below), then the cards it
  blocks and the cards that name it as related.

`kan link` refuses a `blocked_by` link that would close a cycle among the cards it can read, and a
`blocked_by` link from a card to one of its own ancestors: by the parent rule below, the ancestor
waits for the card while the card waits for the ancestor, and neither is ever ready.

#### Lookup results

Every stored ref resolves to exactly one of these. Kan never treats "not in what I loaded" as
"deleted":

| Result | Meaning | Counts as |
|---|---|---|
| found | The card was read, with its column's category. | blocking unless its category is done; see below |
| absent | The card's board was read completely and the card isn't on it. | blocking; `doctor --fix` may remove the link |
| board not here | No registered project holds the board. | blocking |
| ambiguous board | Two registered paths hold the board. | blocking |
| unsupported schema | The card or its board is newer than this kan. | blocking |
| unreadable | A read or parse error, or `FileCardStore.List` skipped the card file. | blocking |

"Read completely" means no file on that board was skipped: `FileCardStore.List` logs and skips a
malformed or wrong-version card file while still returning success, and a board where that happened
can't prove a card is absent. `kan doctor --fix` removes a link only when the result is absent, and
checks again just before writing. It edits only cards in the current project.

### Ready

A card is ready when all of these hold:

1. Its column's category is open.
2. Every blocker is found, in a done column.
3. It isn't the parent of a card that is still unfinished, meaning in any category other than done
   or dropped. A parent with unfinished children is a container for work, and the children are
   what's ready.
4. It matches none of the board's ready exclusions or the command's (see below).

Every blocker state other than found-and-done keeps the card out of ready. Some get more:

- **Found, in a dropped column.** The card is flagged "blocker dropped" and listed under "Needs
  attention" in `kan ready`'s output, because its premise may be gone. Someone, usually an agent,
  removes the link with `kan unlink` or drops the card. Every tracker we surveyed releases such
  cards silently instead.
- **Found, in a column with no category**, or on a board with no categories. Kan can't tell
  whether it's done. `kan doctor` names the column.
- **Any result other than found.** `kan show` says which result and why: the missing card, the
  board that isn't here, both paths of an ambiguous board, the kan version a newer schema needs.

#### Order

`kan ready` reads the open columns from right to left in the board's column order, and each column
top to bottom. All but one of the maintainer's work boards put the lowest-priority open column
leftmost (`backlog-lo`, `backlog-hi`, `next`), so right to left yields `next` first. A board with
one open column gets that column top to bottom, and its first few ready cards are in effect its
"next".

The exception is a sprint board with `p0` left of `p1`. A board like that sets `[ready] order`.
Open columns the list leaves out come after the listed ones, right to left.

#### Exclusions

`[ready] exclude` lists filters that keep a card out of ready on this board, so agents don't have
to remember to pass them. The command adds more with `-x key=value`, and `--no-exclude` ignores
the board's list. Exclusions decide what is ready, not what needs attention: a card with a dropped
blocker is listed under "Needs attention" even if an exclusion matches it.

#### Filters

`kan list` and `kan ready` share one filter implementation:

- `-f key=value` keeps cards that match; `-x key=value` drops cards that match. Both repeat.
- All filters must hold at once. Two `-f` filters on one field keep only cards matching both.
- Matching follows the field's type in the board config: a set field matches when it contains the
  value, a boolean compares as `true` or `false`, anything else compares as a string.
- A card without the field, or on a board that doesn't define it, never matches. So `-f` drops it
  and `-x` keeps it.
- `kan list` also gains `--category <c>`. Its existing `--sort` and `--descending` still apply
  after filtering. `kan ready` keeps manual order.

#### Output

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

`--limit N` prints the first N ready cards.

JSON keeps query context out of the card object. The card's own JSON, whose custom fields sit at
the top level and may use any name, goes unchanged inside an envelope, so context fields such as
`ref` can't collide with a custom field of the same name:

```json
{
  "ready": [
    {
      "ref": "2R9nzEvz:a_2nQ0vLx1k",
      "board_id": "2R9nzEvz",
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

`kan list --json` without `--all` keeps its current shape.

On a board with no categories, `kan ready` fails with: `board "main" has no column categories, so
kan can't tell which cards are open work. Run "kan column categorize" to set them.`

### Web

The card JSON the API returns gains `blocked_by` and `related` as built-in fields, and columns carry
their category. The browser works out the blocked marker for same-board links itself, from the
cards and columns it already has loaded. That way, when a blocker moves to done and the file
watcher reloads only that card, the cards waiting on it update too. The card modal lists blockers
and the cards a card blocks, each linking to that card. Adding and removing links in the web UI is
a later card; until then links are set from the CLI.

### Doctor

`kan doctor` gains checks for: links whose result is absent (fixable), cycles, a `blocked_by` link
to an ancestor, parents that resolve to nothing, self-parents and parent cycles, links and blockers
in columns without a category, and `[ready]` entries that fail validation.

## Phase 4: across boards

### Running without a project

`kan list --all`, `kan ready --all`, and a read of a card on another project's board
(`kan show b_2hH3HWg0E:a_2n5h6dlsT`) work from any directory, including one with no board. Today
every read command requires a project, so these need a mode that reads only the registry.

### References

How kan finds the card a command argument names:

1. **`board:card`** looks only on that board: an exact card ID first, then an alias, then the
   existing fuzzy match.
2. **With `-b` or `-g`**, kan looks only on that board, as today. `-b` now also accepts a board ID.
   Combining `-b` with a `board:card` argument is an error.
3. **A bare argument** is first matched as an exact card ID on the current project's boards. If
   none matches, kan checks every visible registered project for a file named after the ID, which
   costs one file lookup per board and no parsing. One match is the card; two or more is an error
   listing each as a `board:card` ref. If no ID matches anywhere, kan tries an alias, then the
   fuzzy match, in the current project only, because aliases are unique only within a board.

A stored ref resolves by exact ID only, never by alias or fuzzy match.

Kan already searches every board of the current project for a bare argument
(`ResolveAcrossBoards` in `internal/resolver/card_resolver.go`), with exceptions this design keeps:
`-b` and `-g` pin one board, and a project with 10 or more boards prompts or errors instead.

### Writing

A command that writes may change a card only in the current project, or on the designated global
board through `-g`, as today. The current project is the one discovery returns after the worktree
redirect, with symlinks resolved, so a linked worktree still writes its shared board. Kan resolves
the target card and checks this rule before it migrates, initializes or runs hooks on the target's
project. Otherwise it refuses:

```
card a_2n5h6dlsT is on board b_2hH3HWg0E in /Users/alex/src/api; run kan from there to change it.
```

Reads work from anywhere; writes don't, because editing another repo's working tree from somewhere
else would surprise people.

### Links across boards

A link to another board's card is stored as `boardID:cardID`. When the link service records one,
from `kan link` or `kan add --blocked-by`, kan also records the target board in the project config
(project schema `project/3`):

```toml
[linked_boards.b_2hH3HWg0E]
  name = "api/main"
  remote = "git@github.com:example/api.git"
```

`name` is the target's project name and board name. `remote` is the URL of the target repo's
`origin` remote, or of its only remote; with several remotes and none named `origin`, or with none,
kan leaves it out. Kan removes any user name or password from the URL before storing it, because
the project config is committed. Kan never uses either field to find a board: any clone or mirror
holding that board ID satisfies the link. They only improve the message when the board isn't on
this machine:

```
a_2nQ0vLx1k waits on api/main (b_2hH3HWg0E), which isn't on this machine.
One place to get it: git@github.com:example/api.git
```

`kan show` computes reverse links across all registered boards, so closing a card shows what waits
on it in other repos too.

The web UI resolves cross-board links when it loads a board and on reload; it doesn't watch other
projects' files. Clicking a card on another project's board opens it read-only in the modal,
without switching the server to that project.

### Queries across boards

`--all` on `kan list` and `kan ready` reads every visible registered project. It can't be
combined with `-b` or `-g`.

- Results are grouped by board, in order of project path, then board name. They aren't ranked
  across boards: which project's work matters more is a decision about projects, and kan doesn't
  make it.
- `--limit N` applies per board.
- `kan ready --all` skips boards without categories. Both commands skip boards they can't read or
  whose schema is newer than this kan. Text output names skipped boards and the reason in a note on
  stderr. JSON lists them under `"skipped"` with `path`, `board_id` and `reason`. Skipping a board
  doesn't change the exit status.
- `kan list --all --json` uses the envelope shown under [Output](#output), and the card inside
  carries its raw `blocked_by` and `related`, so an outside tool can draw the dependency graph.

Every `--all` query reads the card files. Reading and parsing all 3,955 cards across the
maintainer's 40 boards takes about 150 ms once the files are in the OS cache. We add no index or
cache until queries are measurably slow.

`kan doctor --all` runs the link checks across boards and reports duplicate board IDs in the
registry. With `--fix` it still edits only the current project.

## Agent guide

The skill (`extras/skill/SKILL.md`) and the web docs get a section on finding and taking work:
`kan ready` to choose, `kan link` while filing cards that depend on others, a move into an active
column when starting, and the "Needs attention" list as work in its own right. Each phase's cards
also update the docs for the commands they add, as `AGENTS.md` requires, and the shell completion.

## Deferred

Each of these gets a design card, not an implementation card.

- **Claims.** Moving a card from open into active, done atomically: the move fails if someone else
  moved the card first, and it records who holds the card and an expiry the claimer picks, with a
  default. The holder renews the claim while working. An expired claim leaves the card in active,
  flagged as stale, and anyone can take it over; kan tells them who held it. Claims and releases go
  into the card's existing history. A lock file makes this atomic on one machine. Across machines
  with plain files, nothing can, short of an optional central lock.
- **Shared definitions.** Fields, columns and pattern hooks defined once on the owner's machine and
  copied into each board that uses them. The board's copy stays what kan reads, so someone who
  clones the repo without the owner's config sees a complete board. Open: what happens when a
  board edits its copy. The simplest rule is that the edit detaches that board from the shared
  definition, after a warning. A smaller step, `kan board create --from <board>` to copy another
  board's columns and fields, belongs on the same card.
- **Fetched copies of boards not on this machine.** Kan fetches a linked board's `.kan/` folder
  from its recorded remote into a cache and reads it read-only. A registered clone always wins, and
  registering one deletes the fetched copy. Fetching is an explicit command or a prompt, never a
  side effect of a query. It can't work for a repo with no remote, and it finds the board only in
  `.kan/`, because a custom `data_location` is set in the cloner's own global config, not in the
  repo.
- **Board storage in a git ref.** Cards committed under a ref such as `refs/kan/board` instead of
  `.kan/` on the main branch: no board-only commits on main, and every write committed as it
  happens instead of left uncommitted in the main checkout. Costs: a second storage backend, a
  `kan sync` command, and cards that are no longer files you can open. It would be per-repo opt-in,
  with plain files the default.
- **Moving cards between boards.** Kan has no command for it today; `kan move` moves between
  columns. A card's identity is `boardID:cardID`, so moving its file to another board breaks every
  link to it and turns its own bare refs into refs on the new board. A design for moves has to
  choose between redirects and explicit repair. Restoring a deleted card to a different board
  (`CardService.Restore`) can leave links dangling the same way, and `kan doctor` reports them.
- **A visual view of dependencies across repos** belongs outside kan. Another tool reads
  `kan list --all --json` and `kan ready --all --json`.

## Rejected alternatives

- **Moving to beads.** Beads' statuses have no order, so a board can't have columns; it has no
  typed custom fields; it has no view across repos; and it stores issues in a Dolt database instead
  of files, after changing its storage design three times between January and April 2026.
- **A priority field, global or per board.** A 0-100 number drifts upward as everything gets marked
  important, and mapping each board's own priority onto it adds false precision. Card order already
  ranks the work on a board, and ranking work across boards is a decision about projects.
- **Separate backlog and next categories.** See [Phase 2](#phase-2-column-categories).
- **Kan knowing about specific fields such as provenance.** That would impose a workflow. Filters
  across boards match fields by name, which is enough.
- **A board per worktree**, so a worker's card moves travel with its branch. One board per repo,
  shared by every worktree and seen by every query at once, is what we want, and kan already does
  it on purpose.
- **Picking one copy when clones share a board ID**, for example the path that sorts first. Clones
  can disagree, and picking the stale one would release cards whose blocker was reopened.
- **Refusing the `card/4` migration on a field-name collision** and asking the user to rename the
  field. Kan has no command to rename a field, and `docs/COMPAT.md` promises migrations that need
  no hand-editing.
- **A stored index across boards.** Reading the files is fast enough, and a stored index would be a
  second place for data to live and drift.
- **Writing to other projects' boards from anywhere.** See [Writing](#writing).

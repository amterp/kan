# Keyboard Shortcuts

## Quick Search (Omnibar)

Press **⌘K** to open quick search. Start typing to filter cards in real-time.

| Shortcut | Action |
|----------|--------|
| ⌘K | Open/close quick search |
| ↑ ↓ | Navigate between cards in a column |
| ← → | Navigate between columns |
| Enter | Open highlighted card |
| Escape | Close quick search |

### Slash Commands

Type `/` in quick search to see available commands with autocomplete. Use ↑ ↓ to navigate suggestions and Enter to select.

| Command | Action |
|---------|--------|
| /board | Switch to another board |
| /compact | Toggle compact view |
| /slim | Toggle slim view (vertical columns) |

### How Filtering Works

Quick search uses **word-based substring matching**. Each word in your query must appear as a consecutive substring somewhere in the card. Multiple words are AND'd together, but can match different fields. For example:

- `bug` matches "fixing a **bug**" and "de**bug**ging"
- `fix bug` matches a card with "**fix** login" in title and "**bug** report" in description
- `fg` does *not* match "fixing a bug" (not a consecutive substring)

The search looks across all card fields: title, alias, description, and any custom fields defined on your board.

### Filtering Behavior

- Cards that don't match your query disappear from the board
- Empty columns are hidden while filtering
- Drag-and-drop continues to work with the filtered set
- The filter clears when you close quick search

## View Modes

| Shortcut | Action |
|----------|--------|
| ⌘C | Toggle compact view |
| ⌘J | Toggle slim view (vertical columns) |

**Compact mode** reduces card padding and hides aliases to show more cards at once.

**Slim mode** stacks columns vertically for narrow windows. Cards get an advance button (moves to next column) and right-click context menu (move to any column). Card modals are disabled - slim mode is for quick task processing.

## Board

| Shortcut | Action |
|----------|--------|
| 1-9 | Start creating a card in column N |

Columns are numbered left to right starting at 1. If a card creation form is already open in another column, it will close and any draft title you typed will be preserved - press the original number again to return to it. Boards with more than 9 columns will have the remaining columns unreachable by shortcut.

## Card Creation

When typing in the new card input:

| Shortcut | Action |
|----------|--------|
| Enter | Create card and continue adding |
| ⇧↵ | Create card and open field panel |
| ⌘↵ | Create card and open full modal |
| Escape | Cancel and close input |

The **field panel** is a compact popup that appears next to your newly created card, letting you quickly set custom fields (like type, priority, tags) without opening the full modal.

## Undo

Press **⌘Z** to undo and **⇧⌘Z** to redo. The last 20 actions are tracked.

| Action | What Undo Does |
|--------|----------------|
| Card move (drag-and-drop, advance, context menu) | Moves card back to its previous column and position |
| Card delete (inline or modal) | Restores the card with all its original data |
| Card field edit (title, description, custom fields) | Reverts changed fields to their previous values |

Performing a new action after undoing clears the redo history (standard undo/redo behavior).

Undo and redo are aware of external changes. If another process (e.g. the CLI) modifies a card that's on the stack, the operation will be skipped with a notification rather than overwriting the external change.

The undo/redo stacks are cleared when you switch boards or when the board's column/field schema changes externally.

## Card Editor

When editing a card's description:

| Shortcut | Action |
|----------|--------|
| ⌘B | Bold |
| ⌘I | Italic |
| ⌘K | Insert link |
| ⌘↵ | Save and exit edit mode |
| Escape | Save and exit edit mode |

See [Editing Cards](/docs/editing) for details on Markdown support.

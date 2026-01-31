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

## Card Creation

When typing in the new card input:

| Shortcut | Action |
|----------|--------|
| Enter | Create card and continue adding |
| ⇧↵ | Create card and open field panel |
| ⌘↵ | Create card and open full modal |
| Escape | Cancel and close input |

The **field panel** is a compact popup that appears next to your newly created card, letting you quickly set custom fields (like type, priority, tags) without opening the full modal.

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

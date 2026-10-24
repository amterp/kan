# Editing Cards

Kan uses Markdown for card descriptions. Click anywhere on a description to edit it; press **⌘↵** or click outside to stop editing.

## Blockers

Cards show **Blocked** while any blocker has not reached a column categorized as `done`. Cards in done or dropped columns do not show the marker. A dropped blocker still blocks unfinished work. Moving a blocker to done updates the cards waiting on it automatically, including when search hides the blocker.

Open a card to see **Blocked by** and **Blocks**. Click a card title in either list to open that card. Kan saves pending edits before switching; if saving fails, your edits stay open. Unavailable blockers show their stored reference. Links to another board stay blocked and show no card details here.

Post or clear a new comment before switching cards. If you are editing a comment, save or cancel that edit first. Kan keeps comment drafts open until you finish them.

Manage links from the CLI with `kan link`, `kan unlink`, or `kan add --blocked-by`. Related links do not affect the blocked marker. Link editing in the web UI is planned for a later release.

## Keyboard Shortcuts

| Shortcut | Action |
|----------|--------|
| ⌘B | Bold |
| ⌘I | Italic |
| ⌘K | Insert link |
| ⌘↵ | Exit edit mode |
| Escape | Exit edit mode |

**Tip:** For links, select the text you want to link first, then press ⌘K. The URL placeholder will be selected so you can paste or type the URL immediately.

## Supported Markdown

Kan supports [GitHub Flavored Markdown](https://github.github.com/gfm/) (GFM):

- **Bold** and *italic* text
- [Links](https://example.com) and `inline code`
- Bullet lists and numbered lists
- Task lists with checkboxes
- Tables
- Code blocks
- Blockquotes
- ~~Strikethrough~~

Line breaks work like GitHub comments: a single newline renders as a line break, and a blank line starts a new paragraph.

### Example

```markdown
## My Card

This is a **bold** statement with a [link](https://example.com).

- [ ] Todo item
- [x] Completed item

| Column A | Column B |
|----------|----------|
| Data 1   | Data 2   |
```

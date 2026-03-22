# Kan Documentation

Kan is a file-based kanban board that lives in your repository. All data is stored as plain files in `.kan/` - no database, no server dependencies, works with any VCS.

**A note on shortcuts:** Throughout these docs, ⌘ represents the super key - Cmd on Mac, Ctrl on Windows/Linux.

## Who is Kan for?

Kan works great for solo developers and small teams working on personal projects.

If you're juggling multiple side projects, you know the friction: you open a project you haven't touched in a while, and your task context is... somewhere else. Maybe a Trello board you forgot about, maybe scattered notes, maybe just your memory.

With Kan, every project has its own board that lives right in the repo. Open the project, run `kan serve`, and you're looking at exactly where you left off. No accounts, no syncing, no context switching between your code and some external service.

This works because Kan is:

- **Local and fast.** No network calls, no loading spinners. Your board is just files on disk.
- **File-based.** Plain JSON and TOML that version control tracks like any other file. Clone a repo, get its board.
- **Self-contained.** No database, no server dependencies. Just a single binary that serves a web UI.

## Quick Start

Initialize Kan in your project:

```bash
kan init
kan serve
```

That's it. Your board opens in the browser.

From there, the web UI handles everything - add cards, drag them between columns, click to edit, use ⌘K to quick-search. Most users never need to touch the CLI for anything more.

For scripting or automation (CI, AI agents, etc.), there's a full [CLI](/docs/cli) with commands to add, edit, move, and query cards programmatically.

## Topics

- [Keyboard Shortcuts](/docs/shortcuts) - Omnibar search, navigation, and editor shortcuts
- [Editing Cards](/docs/editing) - Markdown support and formatting
- [Custom Fields](/docs/custom-fields) - Define enum, tags, string, and date fields
- [Configuration](/docs/configuration) - Board structure, columns, and display options
- [Link Rules](/docs/link-rules) - Auto-link patterns like Jira tickets or GitHub issues
- [CLI Reference](/docs/cli) - Full command line tool usage

## Roadmap

Kan is usable today. Current focus areas:

- **Card relationships** - "Related to" links and "blocked by" dependencies
- **More card features** - Additional field types, richer visualization, display customization
- **Board customization** - More control over how your boards & cards look and behave
- **Quality of life** - Ongoing improvements and bug fixes

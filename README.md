# Kan

A kanban board that lives in your repository.

![web-screenshot](./assets/web-screenshot.png)

## What is Kan?

Kan is a kanban board stored as plain files in your repo.
Run `kan serve` to open the web UI, or use the CLI for quick updates.
Since it's just files, your board is version-controlled alongside your code and visible to anyone with access to the repo.

## Quick Start

```bash
kan init
kan serve
```

That's it. Your board opens in the browser.

## CLI

```bash
kan add "Fix login bug"       # Add a card
kan add "Update docs" -c done # Add to specific column
kan list                      # See all cards
kan show fix-login-bug        # View card details (by alias or ID)
kan edit fix-login-bug        # Edit a card
```

## Why Kan?

- **Your board lives where your project lives.** Clone a repo, see its board.
- **Runs locally.** No SaaS, no accounts, works offline, stays snappy.
- **Plain files, no lock-in.** Cards are JSON, config is TOML.
- **Version-controlled.** Changes tracked by Git (or any VCS) like everything else.

## Installation

```bash
go install github.com/amterp/kan/cmd/kan@latest
```

## How It Works

Kan stores data in a `.kan/` directory:

```
.kan/
  boards/
    main/
      config.toml        # Board settings, columns, labels
      cards/
        k7xQ2m.json      # One file per card
```

Cards are JSON. Board config is TOML. Version control tracks changes like any other file.

## Status

Kan is in early development. Core features work - boards, cards, columns, drag-and-drop in the web UI, CLI commands.
More coming: comments, search, and other refinements.

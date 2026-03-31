# AI Agents

Kan's file-based, CLI-first design makes it a natural fit for AI coding agents. There's no API to authenticate against, no browser to automate, no external service to configure. An agent that can run shell commands can do everything a human can - create boards, add cards, triage work, bulk-edit fields, and more.

## Why It Works

Most project management tools require browser interaction or API keys. Kan doesn't. Your board is just files on disk, and the CLI is the primary interface for programmatic access.

This means an AI agent can:

- **Set up a board from scratch** - columns, custom fields, display config, all through `kan init` and direct TOML editing
- **Create cards in bulk** - `kan add` in a loop, with custom fields, descriptions, and column placement
- **Triage and organize** - move cards between columns, update fields, add comments
- **Query the board** - `kan list`, `kan show`, and `--json` output for structured data
- **Do mass edits** - rename fields, re-categorize cards, update descriptions across dozens of cards at once

Things that would take you 20 minutes of clicking are a single prompt away.

## The Kan Skill

Kan ships with a **skill file** ([`extras/skill/SKILL.md`](https://github.com/amterp/kan/blob/main/extras/skill/SKILL.md)) that follows the [Agent Skills](https://agentskills.io) standard. Any AI agent that supports this standard can load the skill and immediately understand how to work with Kan boards.

The skill includes:

- A **board setup wizard** - an interactive flow where the agent asks about your project and suggests columns, custom fields, and display configuration tailored to your workflow
- A **complete CLI reference** - so the agent knows every command and flag available
- **Best practices** - guidance on column descriptions, field types, and board structure

Compatible agents include Claude Code, Codex, and others - check the [Agent Skills](https://agentskills.io) site for a full list. See your agent's documentation for how to install skills.

## Tips for AI-Friendly Boards

A few small things make your board much easier for agents to work with.

### Write Column Descriptions

Column descriptions aren't just for humans - agents use them to decide where cards belong. A column named "next" is ambiguous. A column with the description "Cards prioritized for the current sprint - ready to be picked up" tells the agent exactly what goes there.

You can set these in your board's `config.toml`:

```toml
[[columns]]
name = "backlog"
description = "Unprioritized work. Ideas and tasks that haven't been scheduled yet."

[[columns]]
name = "next"
description = "Prioritized and ready to pick up. Limited to ~5 cards."
```

### Use `--json` for Structured Output

When agents need to read board state, `--json` gives them structured data instead of human-formatted text:

```bash
kan list --json
kan show CARD-1 --json
```

This is more reliable for agents to parse than the default table output.

### Use `-I` for Non-Interactive Mode

The `-I` flag skips interactive prompts, which is important for agents that can't respond to terminal UI elements:

```bash
kan add -t "Fix login bug" -I
kan edit CARD-1 -s next -I
```

### Custom Fields Help Agents Categorize

Fields like `type` (bug, feature, chore) or `priority` (high, medium, low) give agents a vocabulary for organizing work. When you ask an agent to "add the bugs we discussed," it knows to set `type = bug` automatically.

See [Custom Fields](/docs/custom-fields) for the full list of field types.

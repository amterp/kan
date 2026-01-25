# Custom Fields

Kan supports custom fields on cards to track whatever metadata matters to your workflow - priority, assignee, type, labels, due dates, and more.

## Field Types

| Type | Description | Example Values |
|------|-------------|----------------|
| `enum` | Single-select from defined options | `"bug"`, `"feature"` |
| `tags` | Multi-select from defined options | `["blocked", "urgent"]` |
| `string` | Free-form text | `"John Doe"`, `"https://..."` |
| `date` | Date value | `"2024-03-15"` |

## Defining Fields

Custom fields are defined in your board's `config.toml` under the `[custom_fields]` section:

```toml
[custom_fields.type]
type = "enum"
options = [
  { value = "feature", color = "#16a34a" },
  { value = "bug", color = "#dc2626" },
  { value = "task", color = "#4b5563" },
]

[custom_fields.labels]
type = "tags"
options = [
  { value = "blocked", color = "#dc2626" },
  { value = "needs-review", color = "#f59e0b" },
]

[custom_fields.assignee]
type = "string"

[custom_fields.due_date]
type = "date"
```

### Enum and Tags

For `enum` and `tags` fields, you must define the allowed options. Each option can have a color for visual display:

```toml
[custom_fields.priority]
type = "enum"
options = [
  { value = "low", color = "#6b7280" },
  { value = "medium", color = "#f59e0b" },
  { value = "high", color = "#ef4444" },
]
```

The difference between `enum` and `tags`:
- **Enum** is single-select - a card can only have one value (e.g., a card is either a bug OR a feature)
- **Tags** is multi-select - a card can have multiple values (e.g., a card can be both "blocked" AND "urgent")

### String and Date

For `string` and `date` fields, no options are needed:

```toml
[custom_fields.assignee]
type = "string"

[custom_fields.due_date]
type = "date"
```

## Card Display

The `[card_display]` section in your board config controls how custom fields appear on cards in the board view:

```toml
[card_display]
type_indicator = "type"           # Shown as a colored badge
badges = ["labels"]               # Shown as colored chips
metadata = ["assignee"]           # Shown as small text
```

### Display Slots

| Slot | Field Types | Rendering |
|------|-------------|-----------|
| `type_indicator` | `enum` only | Colored badge (single value) |
| `badges` | `tags` only | Colored chips (multiple values) |
| `metadata` | Any | Small text in card footer |

Fields not assigned to a display slot are only visible in the card detail view.

**Example card appearance:**

```
┌────────────────────────────────────┐
│ Fix login timeout                  │  ← title
│ ┌─────────┐ ┌─────────┐ ┌────────┐ │
│ │   bug   │ │ blocked │ │ urgent │ │  ← type_indicator + badges
│ └─────────┘ └─────────┘ └────────┘ │
│ assignee: sarah              📝 💬 │  ← metadata + system indicators
└────────────────────────────────────┘
```

## Setting Fields via CLI

Use the `-f` flag on `kan add` or `kan edit` to set custom field values:

```bash
# Set a single field
kan add "Fix bug" -f type=bug

# Set multiple fields
kan add "New feature" -f type=feature -f priority=high

# Set tags (comma-separated)
kan edit abc123 -f labels=blocked,urgent

# Clear a field
kan edit abc123 -f assignee=
```

## Default Configuration

New boards are created with a `type` field and sensible defaults:

```toml
[custom_fields.type]
type = "enum"
options = [
  { value = "feature", color = "#16a34a" },
  { value = "bug", color = "#dc2626" },
  { value = "task", color = "#4b5563" },
]

[card_display]
type_indicator = "type"
```

You can modify, remove, or add fields to match your workflow.

## Reserved Names

Field names cannot start with:
- `_` (reserved for internal use)
- `kan_` (reserved for Kan)

Use the `x_` prefix if you need to escape a name that would otherwise conflict.

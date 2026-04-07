# Custom Fields

Kan supports custom fields on cards to track whatever metadata matters to your workflow - priority, assignee, type, labels, due dates, and more.

## Field Types

| Type | Description | Example Values |
|------|-------------|----------------|
| `enum` | Single-select from defined options | `"bug"`, `"feature"` |
| `enum-set` | Multi-select from defined options | `["blocked", "urgent"]` |
| `free-set` | Multi-value freeform text | `["backend", "auth"]` |
| `string` | Free-form text | `"John Doe"`, `"https://..."` |
| `date` | Date value | `"2024-03-15"` |
| `boolean` | Yes/no flag | `true`, `false` |

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
type = "enum-set"
options = [
  { value = "blocked", color = "#dc2626" },
  { value = "needs-review", color = "#f59e0b" },
]

[custom_fields.topics]
type = "free-set"

[custom_fields.assignee]
type = "string"

[custom_fields.due_date]
type = "date"
```

### Wanted Fields

Mark a field as `wanted` to get warnings when cards are missing it:

```toml
[custom_fields.type]
type = "enum"
wanted = true
options = [
  { value = "feature", color = "#16a34a" },
  { value = "bug", color = "#dc2626" },
]
```

When a card is missing a wanted field:
- CLI commands (`kan add`, `kan edit`) print warnings
- Use `--strict` flag to block operations instead of warning
- `kan doctor` reports cards missing wanted fields
- Web UI shows an asterisk on wanted field labels and a warning icon on cards

This is useful for enforcing workflow standards without making fields strictly required.

### Badge Colors

When badges or chips are shown on cards, each value gets a color:

- **Enum / enum-set options with a `color`** - the specified color is used as-is.
- **Enum / enum-set options without a `color`** - a color is automatically assigned based on the value's text.
- **Free-set values** - always auto-colored (no predefined options to attach colors to).
- **Boolean fields** - auto-colored based on the field name when shown as a badge.

Auto-assigned colors are deterministic and case-insensitive - "Bug" and "bug" will always get the same color. The same value in different field types is not guaranteed to get the same color. If you want to override an auto-assigned color for an enum or enum-set field, add an explicit `color` to the option in your board config.

### Enum and Enum-set

For `enum` and `enum-set` fields, you must define the allowed options. Each option can optionally have a color for visual display:

```toml
[custom_fields.priority]
type = "enum"
options = [
  { value = "low", color = "#6b7280" },
  { value = "medium", color = "#f59e0b" },
  { value = "high", color = "#ef4444" },
]
```

The difference between `enum` and `enum-set`:
- **Enum** is single-select - a card can only have one value (e.g., a card is either a bug OR a feature)
- **Enum-set** is multi-select - a card can have multiple values from the defined options (e.g., a card can be both "blocked" AND "urgent")

### Free-set

`free-set` fields accept any text values without predefined options - useful for ad-hoc labels, topics, or tags:

```toml
[custom_fields.topics]
type = "free-set"
```

Values are deduplicated and limited to 10 per field. In the web UI, values are added by typing and pressing Enter, and removed by clicking the X on each chip.

### String and Date

For `string` and `date` fields, no options are needed:

```toml
[custom_fields.assignee]
type = "string"

[custom_fields.due_date]
type = "date"
```

### Boolean

Boolean fields are simple yes/no flags - no options needed:

```toml
[custom_fields.high_priority]
type = "boolean"
```

In the CLI, set with `-f high_priority=true` or `-f high_priority=false`. Also accepts `yes`/`no` and `1`/`0` (case-insensitive). In the web UI, boolean fields render as toggle switches in the card detail view. When added to the `badges` display slot, a boolean field appears as a colored badge on the card when `true`, and is hidden when `false` or unset.

## Card Display

The `[card_display]` section in your board config controls how custom fields appear on cards in the board view:

```toml
[card_display]
type_indicator = "type"           # Shown as a colored badge
tint = "priority"                 # Card background color
badges = ["labels"]               # Shown as colored chips
```

### Display Slots

| Slot | Field Types | Rendering |
|------|-------------|-----------|
| `type_indicator` | `enum` only | Colored badge (single value) + left border accent |
| `tint` | `enum` only | Subtle background color wash on the entire card |
| `badges` | `enum-set`, `free-set`, `boolean` | Colored chips (set values) or field name badge (boolean, shown when `true`) |

Fields not assigned to a display slot are only visible in the card detail view.

The `tint` slot colors the entire card's background using the assigned option's `color` value. This makes specific cards visually stand out on the board. The tint only appears on cards where the field has a value set - cards without a value have no background color. Clear the field value (e.g. `kan edit <card> -f priority=`) to remove the tint. For best results, use bright, saturated colors on your enum options.

**Example card appearance:**

```
┌━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┐
┃ Fix login timeout                  ┃  ← title
┃ ┌─────────┐ ┌─────────┐ ┌────────┐ ┃
┃ │   bug   │ │ blocked │ │ urgent │ ┃  ← type_indicator + badges
┃                              📝 💬 ┃  ← system indicators
┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛
  ↑ entire card background is tinted
```

## Setting Fields via CLI

Use the `-f` flag on `kan add` or `kan edit` to set custom field values:

```bash
# Set a single field
kan add "Fix bug" -f type=bug

# Set multiple fields
kan add "New feature" -f type=feature -f priority=high

# Set multi-value fields (comma-separated)
kan edit abc123 -f labels=blocked,urgent
kan edit abc123 -f topics=backend,auth

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
